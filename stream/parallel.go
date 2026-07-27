// parallel.go bridges Stream[T] to async.Pipe[T] — the one thing this
// package deliberately doesn't do itself: running an operation across
// goroutines. Region is the general escape hatch, taking a closure that
// can call anything async.Pipe offers (Parallel, Buffer, RateLimit,
// Fork, Merge, Window, chained however the region needs). Parallel is
// sugar over Region for the single most common case: run one operation
// with bounded concurrency, nothing more.
package stream

import "github.com/azuiktech/kleisli-go/async"

// ToPipe hands s's items to async.Pipe — the explicit boundary between
// stream's synchronous world and async's CSP one. Everything async
// offers is reachable from here; stream knows nothing about how any of
// it works.
func (s Stream[T]) ToPipe() async.Pipe[T] { return async.From(s.items) }

// FromPipe collects an async.Pipe back into a Stream — ToPipe's return
// half.
func FromPipe[T any](p async.Pipe[T]) Stream[T] { return Of(p.Collect()) }

// Region is ToPipe + FromPipe as one call, so a fluent chain doesn't
// have to break into separate statements to cross the boundary and
// back. fn receives s's own Pipe and returns whatever async.Pipe[U] the
// region should produce — any combination of async's own operations,
// not just Parallel.
func (s Stream[T]) Region[U any](fn func(async.Pipe[T]) async.Pipe[U]) Stream[U] {
	return FromPipe(fn(s.ToPipe()))
}

// Parallel runs fn across n worker goroutines — sugar over Region for
// the single most common case. Output order is not preserved (matches
// async.Pipe.Parallel's own semantics); use Region directly with
// Enumerate/Ordered if input order must survive, or if the region needs
// anything Parallel alone doesn't cover (rate limiting, buffering,
// fan-out).
func (s Stream[T]) Parallel[U any](n int, fn func(T) U) Stream[U] {
	return s.Region(func(p async.Pipe[T]) async.Pipe[U] {
		return p.Parallel(n, fn)
	})
}
