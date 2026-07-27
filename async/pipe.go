// Package async provides Pipe[T], a channel-backed pipeline type for
// CSP-style concurrent composition — stream's counterpart for work that
// benefits from goroutines: worker pools, rate limiting, fan-out/fan-in,
// batching.
//
// Every stage is an explicitly named call — nothing here infers an
// execution strategy from context, the way Java's Stream.parallel() does.
// A Pipe, once started, must be fully drained (Collect, Each, Reduce, or
// Await) — one abandoned mid-pipeline leaks its producer goroutine.
//
//	total := async.From(urls).
//	    Parallel(8, fetchAndParse).
//	    Reduce(0, sum)
package async

import (
	"context"
	"sync"

	"golang.org/x/time/rate"
)

// Pipe wraps a channel for CSP-style pipeline composition — the
// channel-backed counterpart to stream.Stream's slice-backed one.
type Pipe[T any] struct {
	ch <-chan T
}

// From lifts an already-computed slice into a Pipe: one goroutine sends
// each item in order.
func From[T any](items []T) Pipe[T] {
	ch := make(chan T)
	go func() {
		defer close(ch)
		for _, item := range items {
			ch <- item
		}
	}()
	return Pipe[T]{ch: ch}
}

// Go runs fn on its own goroutine — the CSP "future" primitive: kick off
// one background computation, keep composing. Pair with Await.
func Go[T any](fn func() T) Pipe[T] {
	ch := make(chan T, 1)
	go func() {
		defer close(ch)
		ch <- fn()
	}()
	return Pipe[T]{ch: ch}
}

// Map applies fn to each item as it flows through — one goroutine, same
// order as the source, no pooling. This is what makes a transform
// "async" without making it "parallel": production and consumption of
// this stage can overlap with its neighbors, but fn itself never runs
// concurrently with itself.
func (p Pipe[T]) Map[U any](fn func(T) U) Pipe[U] {
	out := make(chan U)
	go func() {
		defer close(out)
		for item := range p.ch {
			out <- fn(item)
		}
	}()
	return Pipe[U]{ch: out}
}

// Parallel spawns n worker goroutines pulling from p and applying fn
// concurrently — bounded fan-out. Output arrival order is completion
// order, not input order; Enumerate before, Ordered after, if input
// order must survive.
func (p Pipe[T]) Parallel[U any](n int, fn func(T) U) Pipe[U] {
	out := make(chan U)
	var wg sync.WaitGroup
	wg.Add(n)
	for range n {
		go func() {
			defer wg.Done()
			for item := range p.ch {
				out <- fn(item)
			}
		}()
	}
	go func() {
		wg.Wait()
		close(out)
	}()
	return Pipe[U]{ch: out}
}

// Buffer inserts a channel of capacity n between this stage and the
// next: up to n items can queue between a producer and a slower consumer
// without either blocking on the other.
func (p Pipe[T]) Buffer(n int) Pipe[T] {
	out := make(chan T, n)
	go func() {
		defer close(out)
		for item := range p.ch {
			out <- item
		}
	}()
	return Pipe[T]{ch: out}
}

// Fork duplicates p into n independent Pipes, each receiving every item
// p produces — broadcast, not load-balancing (Parallel is for that).
// Every branch must be read at roughly the same pace: a slow branch's
// channel blocks the shared pump, which blocks every sibling — each
// item is sent to branch 0, then branch 1, and so on, in order.
func (p Pipe[T]) Fork(n int) []Pipe[T] {
	outs := make([]chan T, n)
	pipes := make([]Pipe[T], n)
	for i := range n {
		outs[i] = make(chan T)
		pipes[i] = Pipe[T]{ch: outs[i]}
	}
	go func() {
		defer func() {
			for _, out := range outs {
				close(out)
			}
		}()
		for item := range p.ch {
			for _, out := range outs {
				out <- item
			}
		}
	}()
	return pipes
}

// Merge fans multiple Pipes of the same type into one, interleaved in
// arrival order. A free function, not a method — it combines N
// independent Pipes, the same reasoning stream.Zip already uses for
// combining two Streams rather than being a method on one of them.
func Merge[T any](pipes ...Pipe[T]) Pipe[T] {
	out := make(chan T)
	var wg sync.WaitGroup
	wg.Add(len(pipes))
	for _, p := range pipes {
		go func() {
			defer wg.Done()
			for item := range p.ch {
				out <- item
			}
		}()
	}
	go func() {
		wg.Wait()
		close(out)
	}()
	return Pipe[T]{ch: out}
}

// RateLimit paces items through no faster than lim allows — accepts
// x/time/rate's own Limiter directly rather than inventing a parallel
// rate-config shape; construct with rate.NewLimiter. ctx cancelling
// stops the pipe early, mid-item — the one place a Pipe stage needs a
// ctx at all, since it's the one place a blocking wait is unavoidable.
func (p Pipe[T]) RateLimit(ctx context.Context, lim *rate.Limiter) Pipe[T] {
	out := make(chan T)
	go func() {
		defer close(out)
		for item := range p.ch {
			if err := lim.Wait(ctx); err != nil {
				return
			}
			out <- item
		}
	}()
	return Pipe[T]{ch: out}
}

// Window batches items into chunks of n, emitting each full chunk as one
// []T item as soon as it's full — the async analogue of
// stream.WindowFixed, but emitting incrementally instead of only once
// the whole source is exhausted. The trailing chunk may be shorter than
// n. A free function: Pipe[T] -> Pipe[[]T] hits the same "instantiation
// cycle" compiler limit stream.Enumerate's own doc already flags for a
// method of this shape.
func Window[T any](p Pipe[T], n int) Pipe[[]T] {
	out := make(chan []T)
	go func() {
		defer close(out)
		batch := make([]T, 0, n)
		for item := range p.ch {
			batch = append(batch, item)
			if len(batch) == n {
				out <- batch
				batch = make([]T, 0, n)
			}
		}
		if len(batch) > 0 {
			out <- batch
		}
	}()
	return Pipe[[]T]{ch: out}
}

// Indexed pairs an item with its position in whatever Pipe produced it.
type Indexed[T any] struct {
	Index int
	Value T
}

// Enumerate tags each item with its position as it's produced. Call this
// right after From/Go if a later stage (typically Parallel) will
// scramble arrival order and Ordered must restore it afterward — nothing
// tracks position unless you ask for it here. A free function for the
// same instantiation-cycle reason as Window.
func Enumerate[T any](p Pipe[T]) Pipe[Indexed[T]] {
	out := make(chan Indexed[T])
	go func() {
		defer close(out)
		i := 0
		for item := range p.ch {
			out <- Indexed[T]{Index: i, Value: item}
			i++
		}
	}()
	return Pipe[Indexed[T]]{ch: out}
}

// Ordered buffers Indexed items until they can be emitted strictly in
// ascending Index order — Enumerate's pair, undoing whatever reordering
// a concurrent stage in between introduced. One badly-delayed item
// stalls everything queued behind it — the standard cost of restoring
// order after concurrent work, not a bug.
func Ordered[T any](p Pipe[Indexed[T]]) Pipe[T] {
	out := make(chan T)
	go func() {
		defer close(out)
		pending := map[int]T{}
		next := 0
		for item := range p.ch {
			pending[item.Index] = item.Value
			for {
				v, ok := pending[next]
				if !ok {
					break
				}
				out <- v
				delete(pending, next)
				next++
			}
		}
	}()
	return Pipe[T]{ch: out}
}

// Collect drains p fully and returns every item in arrival order.
func (p Pipe[T]) Collect() []T {
	var out []T
	for item := range p.ch {
		out = append(out, item)
	}
	return out
}

// Reduce folds every item into an accumulator of type U, draining p
// fully — the same shape as stream.Stream's own Reduce.
func (p Pipe[T]) Reduce[U any](initial U, fn func(U, T) U) U {
	acc := initial
	for item := range p.ch {
		acc = fn(acc, item)
	}
	return acc
}

// Each drains p, calling fn on every item as it arrives.
func (p Pipe[T]) Each(fn func(T)) {
	for item := range p.ch {
		fn(item)
	}
}

// Await blocks for the one item a Go-built Pipe produces — the natural
// terminal for Go specifically. Returns the zero value of T if p is
// already drained/empty, matching a plain channel receive's own
// behavior; it does not panic.
func (p Pipe[T]) Await() T {
	return <-p.ch
}
