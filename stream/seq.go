package stream

import (
	"cmp"
	"iter"
	"slices"

	"github.com/azuiktech/kleisli-go/adt"
)

// Seq wraps an iter.Seq[T] — Stream's lazy, pull-based counterpart, safe
// over arbitrary sources including unbounded ones. Every method here
// works incrementally: none needs to see the whole sequence to make
// progress or produce correct output. Operations that do (sorting,
// reversing, grouping) exist only on Stream, reachable via Collect —
// sound there specifically because a slice is always finite and already
// in memory; unsound here, since a Seq might not be.
//
// Filter/Map/TakeWhile/... below and Stream's own versions of the same
// names share one implementation (the unexported *Seq engine functions
// in this file) — a Stream just runs it via slices.Values then collects
// immediately, so the two can never silently disagree about what an
// operation means.
type Seq[T any] struct {
	seq iter.Seq[T]
}

// FromSeq wraps an iter.Seq[T] — the standard library's own iterator
// protocol — as a Seq.
func FromSeq[T any](seq iter.Seq[T]) Seq[T] { return Seq[T]{seq: seq} }

// SeqOfOption lifts an Option into a single-element or empty Seq: Some(v)
// yields v once, None yields nothing.
func SeqOfOption[T any](o adt.Option[T]) Seq[T] {
	return FromSeq(slices.Values(o.ToSlice()))
}

// SeqOfMap wraps a map's entries as a Seq of Pair — the lazy counterpart
// of stream.OfMap. Map iteration order is randomized by Go, so callers
// needing a deterministic order should collect and sort.
func SeqOfMap[K comparable, V any](m map[K]V) Seq[Pair[K, V]] {
	return FromSeq(func(yield func(Pair[K, V]) bool) {
		for k, v := range m {
			if !yield(Pair[K, V]{First: k, Second: v}) {
				return
			}
		}
	})
}

// filterSeq is Filter's shared engine.
func filterSeq[T any](seq iter.Seq[T], fn func(T) bool) iter.Seq[T] {
	return func(yield func(T) bool) {
		for v := range seq {
			if fn(v) && !yield(v) {
				return
			}
		}
	}
}

// Filter yields only elements for which fn returns true.
func (s Seq[T]) Filter(fn func(T) bool) Seq[T] {
	return Seq[T]{seq: filterSeq(s.seq, fn)}
}

// mapSeq is Map's shared engine.
func mapSeq[T, U any](seq iter.Seq[T], fn func(T) U) iter.Seq[U] {
	return func(yield func(U) bool) {
		for v := range seq {
			if !yield(fn(v)) {
				return
			}
		}
	}
}

// Map transforms each element into type U.
func (s Seq[T]) Map[U any](fn func(T) U) Seq[U] {
	return Seq[U]{seq: mapSeq(s.seq, fn)}
}

// flatMapSeq is FlatMap's shared engine.
func flatMapSeq[T, U any](seq iter.Seq[T], fn func(T) []U) iter.Seq[U] {
	return func(yield func(U) bool) {
		for v := range seq {
			for _, u := range fn(v) {
				if !yield(u) {
					return
				}
			}
		}
	}
}

// FlatMap maps each element to a slice of U and flattens the results.
func (s Seq[T]) FlatMap[U any](fn func(T) []U) Seq[U] {
	return Seq[U]{seq: flatMapSeq(s.seq, fn)}
}

// takeSeq is Take's shared engine — a pull-based counter, since an
// arbitrary Seq has no random access to reslice the way a Stream does.
func takeSeq[T any](seq iter.Seq[T], n int) iter.Seq[T] {
	return func(yield func(T) bool) {
		if n <= 0 {
			return
		}
		count := 0
		for v := range seq {
			if !yield(v) {
				return
			}
			count++
			if count >= n {
				return
			}
		}
	}
}

// Take yields at most n leading elements, then stops pulling from the
// source entirely.
func (s Seq[T]) Take(n int) Seq[T] {
	return Seq[T]{seq: takeSeq(s.seq, n)}
}

// skipSeq is Skip's shared engine.
func skipSeq[T any](seq iter.Seq[T], n int) iter.Seq[T] {
	return func(yield func(T) bool) {
		count := 0
		for v := range seq {
			if count < n {
				count++
				continue
			}
			if !yield(v) {
				return
			}
		}
	}
}

// Skip discards the first n elements, yielding everything after.
func (s Seq[T]) Skip(n int) Seq[T] {
	return Seq[T]{seq: skipSeq(s.seq, n)}
}

// Each calls fn on every element as it's produced.
func (s Seq[T]) Each(fn func(T)) Seq[T] {
	for v := range s.seq {
		fn(v)
	}
	return s
}

// Any reports whether fn returns true for at least one element —
// short-circuits, never pulling past the first match.
func (s Seq[T]) Any(fn func(T) bool) bool {
	for v := range s.seq {
		if fn(v) {
			return true
		}
	}
	return false
}

// All reports whether fn returns true for every element — short-circuits
// on the first failure.
func (s Seq[T]) All(fn func(T) bool) bool {
	for v := range s.seq {
		if !fn(v) {
			return false
		}
	}
	return true
}

// First returns Some(first element satisfying fn), or None —
// short-circuits, never pulling past the match.
func (s Seq[T]) First(fn func(T) bool) adt.Option[T] {
	for v := range s.seq {
		if fn(v) {
			return adt.Some(v)
		}
	}
	return adt.None[T]()
}

// Reduce folds every element into an accumulator of type U.
func (s Seq[T]) Reduce[U any](initial U, fn func(U, T) U) U {
	acc := initial
	for v := range s.seq {
		acc = fn(acc, v)
	}
	return acc
}

// Collect drains s into a slice — the one operation that forces full
// materialization, same status as any other terminal.
func (s Seq[T]) Collect() []T { return slices.Collect(s.seq) }

// ToStream materializes the Seq into an eager Stream. This forces full
// evaluation — choose Stream over Seq when downstream operations need
// the whole collection at once (sorting, reversing, grouping).
func (s Seq[T]) ToStream() Stream[T] { return Of(s.Collect()) }

// gatherSeq is Gather's shared engine — the same Gatherer value Stream
// uses, driven by a pull instead of a slice loop, emitting as it goes
// rather than building one output slice.
func gatherSeq[T, A, U any](seq iter.Seq[T], g Gatherer[T, A, U]) iter.Seq[U] {
	return func(yield func(U) bool) {
		state := g.Init()
		stopped := false
		emit := func(u U) {
			if stopped {
				return
			}
			if !yield(u) {
				stopped = true
			}
		}
		for v := range seq {
			if stopped {
				return
			}
			next, cont := g.Integrate(state, v, emit)
			state = next
			if !cont {
				break
			}
		}
		if !stopped && g.Finish != nil {
			g.Finish(state, emit)
		}
	}
}

// Gather runs g over s.
func (s Seq[T]) Gather[A, U any](g Gatherer[T, A, U]) Seq[U] {
	return Seq[U]{seq: gatherSeq(s.seq, g)}
}

// MapMulti: fn gets each element plus an emit callback it can call zero,
// one, or many times — see Stream.MapMulti.
func (s Seq[T]) MapMulti[U any](fn func(item T, emit func(U))) Seq[U] {
	return s.Gather(statelessGatherer(fn))
}

// FilterMap keeps and transforms only the elements for which fn's second
// return is true.
func (s Seq[T]) FilterMap[U any](fn func(T) (U, bool)) Seq[U] {
	return s.MapMulti(func(item T, emit func(U)) {
		if u, ok := fn(item); ok {
			emit(u)
		}
	})
}

// TakeWhile yields elements up to but not including the first one for
// which fn is false, then stops pulling from the source entirely.
func (s Seq[T]) TakeWhile(fn func(T) bool) Seq[T] {
	return s.Gather(takeWhileGatherer(fn))
}

// DropWhile skips elements while fn is true, then yields everything from
// the first failure onward.
func (s Seq[T]) DropWhile(fn func(T) bool) Seq[T] {
	return s.Gather(dropWhileGatherer(fn))
}

// DistinctBy keeps only the first element for each key fn extracts.
func (s Seq[T]) DistinctBy[K comparable](fn func(T) K) Seq[T] {
	return s.Gather(distinctByGatherer[T, K](fn))
}

// DistinctSeq keeps only the first occurrence of each element, by direct
// equality — Seq's own Distinct; a method can't narrow Seq[T]'s T to
// comparable, only a function generic over its own T can (same reasoning
// as Stream's own Distinct).
func DistinctSeq[T comparable](s Seq[T]) Seq[T] {
	return s.DistinctBy(func(t T) T { return t })
}

// EnumerateSeq pairs each element with its zero-based index — Seq's own
// Enumerate. A free function, not a method, for the same
// instantiation-cycle reason Stream's Enumerate already is.
func EnumerateSeq[T any](s Seq[T]) Seq[Indexed[T]] {
	return s.Gather(enumerateGatherer[T]())
}

// FlattenSeq collapses a Seq[[]T] into a Seq[T], yielding each inner slice's
// elements in order. Named to match DistinctSeq / EnumerateSeq rather than
// shadowing stream.Flatten.
func FlattenSeq[T any](s Seq[[]T]) Seq[T] {
	return Seq[T]{seq: flatMapSeq(s.seq, func(sl []T) []T { return sl })}
}

// MinBy returns the element with the smallest key fn extracts, draining the
// Seq fully. Returns None if the Seq is empty. When multiple elements share
// the minimum key the first one wins.
func (s Seq[T]) MinBy[K cmp.Ordered](fn func(T) K) adt.Option[T] {
	var best T
	var bestKey K
	found := false
	for v := range s.seq {
		k := fn(v)
		if !found || k < bestKey {
			best, bestKey, found = v, k, true
		}
	}
	if !found {
		return adt.None[T]()
	}
	return adt.Some(best)
}

// MaxBy returns the element with the largest key fn extracts, draining the
// Seq fully. Returns None if the Seq is empty. When multiple elements share
// the maximum key the first one wins.
func (s Seq[T]) MaxBy[K cmp.Ordered](fn func(T) K) adt.Option[T] {
	var best T
	var bestKey K
	found := false
	for v := range s.seq {
		k := fn(v)
		if !found || k > bestKey {
			best, bestKey, found = v, k, true
		}
	}
	if !found {
		return adt.None[T]()
	}
	return adt.Some(best)
}

// ZipSeq pairs elements from two Seqs positionally, stopping at the
// shorter one, via iter.Pull to consume both in lockstep.
func ZipSeq[A, B any](sa Seq[A], sb Seq[B]) Seq[Pair[A, B]] {
	return Seq[Pair[A, B]]{seq: func(yield func(Pair[A, B]) bool) {
		nextA, stopA := iter.Pull(sa.seq)
		defer stopA()
		nextB, stopB := iter.Pull(sb.seq)
		defer stopB()
		for {
			a, okA := nextA()
			b, okB := nextB()
			if !okA || !okB {
				return
			}
			if !yield(Pair[A, B]{First: a, Second: b}) {
				return
			}
		}
	}}
}
