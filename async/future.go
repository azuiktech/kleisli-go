package async

import (
	"context"
	"errors"
	"sync"

	"github.com/azuiktech/kleisli-go/adt"
)

// Receiver represents any concurrency primitive that yields a *Future[T].
// Implemented by *Future[T], *Promise[T], and *Task[I, O].
type Receiver[T any] interface {
	Future() *Future[T]
}

// Future is the read-only consumer handle to get the final Result[T].
type Future[T any] struct {
	ctx    context.Context
	cancel context.CancelCauseFunc
	once   sync.Once
	result adt.Result[T]
}

// Await blocks until the future completes or is cancelled, returning its Result.
func (f *Future[T]) Await() adt.Result[T] {
	<-f.ctx.Done()
	f.once.Do(func() {
		cause := adt.Opt(context.Cause(f.ctx)).OrElse(f.ctx.Err())
		f.result = adt.Err[T](cause)
	})
	return f.result
}

// AwaitCtx blocks until the future completes or ctx expires.
// Does not cancel the future if ctx expires first.
func (f *Future[T]) AwaitCtx(ctx context.Context) adt.Result[T] {
	select {
	case <-f.ctx.Done():
		return f.Await()
	case <-ctx.Done():
		return adt.Err[T](context.Cause(ctx))
	}
}

// Future returns itself so that *Future[T] satisfies Receiver[T].
func (f *Future[T]) Future() *Future[T] {
	return f
}

// Context returns the context associated with this future.
func (f *Future[T]) Context() context.Context {
	return f.ctx
}

// Cancel cancels the future with the given cause.
// If cause is nil, context.Canceled is used via adt.Opt.
// Returns true if this call cancelled the future, or false if already settled.
func (f *Future[T]) Cancel(cause error) bool {
	err := adt.Opt(cause).OrElse(context.Canceled)
	return f.settle(adt.Err[T](err), err)
}

func (f *Future[T]) settle(res adt.Result[T], cause error) bool {
	settled := false
	f.once.Do(func() {
		f.result = res
		f.cancel(cause)
		settled = true
	})
	return settled
}

// Promise is the single-assignment producer handle to set the Future's Result.
type Promise[T any] struct {
	future *Future[T]
}

// NewPromise creates a paired Promise and Future bound to ctx.
func NewPromise[T any](ctx context.Context) (*Promise[T], *Future[T]) {
	c := adt.Opt(ctx).OrElse(context.Background())
	futCtx, cancel := context.WithCancelCause(c)
	fut := &Future[T]{
		ctx:    futCtx,
		cancel: cancel,
	}
	prom := &Promise[T]{
		future: fut,
	}
	return prom, fut
}

// Resolve sets the future's result to val (OK) and settles the future.
// Returns true if this call settled the future, or false if already settled.
func (p *Promise[T]) Resolve(val T) bool {
	return p.future.settle(adt.OK(val), nil)
}

// Reject sets the future's result to err (Err) and settles the future.
// Returns true if this call settled the future, or false if already settled.
func (p *Promise[T]) Reject(err error) bool {
	e := adt.Opt(err).OrElse(errors.New("promise rejected with nil error"))
	return p.future.settle(adt.Err[T](e), e)
}

// Cancel cancels setting the value with the given cause.
// If cause is nil, context.Canceled is used via adt.Opt.
// Returns true if this call cancelled the future, or false if already settled.
func (p *Promise[T]) Cancel(cause error) bool {
	err := adt.Opt(cause).OrElse(context.Canceled)
	return p.future.settle(adt.Err[T](err), err)
}

// Future returns the read-only Future handle.
func (p *Promise[T]) Future() *Future[T] {
	return p.future
}

// Context returns the context associated with the underlying future.
func (p *Promise[T]) Context() context.Context {
	return p.future.ctx
}

var (
	// ErrEmptySources is returned when a combinator requiring at least one source receives none.
	ErrEmptySources = errors.New("async: empty sources")

	// ErrAllFailed is returned when all sources in AnyOf fail.
	ErrAllFailed = errors.New("async: all sources failed")
)

// TaskGroup coordinates concurrent execution of tasks with cancellation context.
type TaskGroup struct {
	wg     sync.WaitGroup
	ctx    context.Context
	cancel context.CancelCauseFunc
}

// NewTaskGroup creates a new TaskGroup with a background context.
func NewTaskGroup() *TaskGroup {
	return TaskGroupWithContext(context.Background())
}

// TaskGroupWithContext creates a new TaskGroup derived from ctx.
func TaskGroupWithContext(ctx context.Context) *TaskGroup {
	c := adt.Opt(ctx).OrElse(context.Background())
	gctx, cancel := context.WithCancelCause(c)
	return &TaskGroup{ctx: gctx, cancel: cancel}
}

// Context returns the group's cancellation context.
func (tg *TaskGroup) Context() context.Context { return tg.ctx }

// Run executes all sources under tg. If policy returns true, the group is marked done
// and all remaining pending sources are cancelled immediately.
func (tg *TaskGroup) Run[T any](sources []Receiver[T], policy func(idx int, res adt.Result[T]) bool) {
	for i, s := range sources {
		tg.wg.Go(func() {
			res := s.Future().AwaitCtx(tg.ctx)
			if policy(i, res) {
				cause := res.Fold(
					func(T) error { return context.Canceled },
					func(e error) error { return e },
				)
				tg.cancel(cause)
				cancelAll(sources, cause)
			}
		})
	}
	tg.wg.Wait()
}

// AllOf returns a Future that resolves when all sources resolve successfully,
// or rejects immediately with the error of the first source that fails.
// When one source fails, all other pending sources are cancelled.
func AllOf[T any](ctx context.Context, sources ...Receiver[T]) *Future[[]T] {
	if len(sources) == 0 {
		return resolvedFuture(ctx, []T{})
	}
	tg := TaskGroupWithContext(ctx)
	prom, fut := NewPromise[[]T](tg.Context())
	watchCancel(fut, func(c error) { cancelAll(sources, c) })

	out := make([]T, len(sources))
	var once sync.Once
	go func() {
		tg.Run(sources, func(i int, res adt.Result[T]) bool {
			return res.Fold(
				func(v T) bool { out[i] = v; return false },
				func(err error) bool {
					once.Do(func() { prom.Reject(err) })
					return true
				},
			)
		})
		once.Do(func() { prom.Resolve(out) })
	}()
	return fut
}

// AllSettled returns a Future that resolves when all sources have settled
// (either resolved or rejected). The returned slice preserves input order
// and contains the Result[T] of each source.
func AllSettled[T any](ctx context.Context, sources ...Receiver[T]) *Future[[]adt.Result[T]] {
	if len(sources) == 0 {
		return resolvedFuture(ctx, []adt.Result[T]{})
	}
	tg := TaskGroupWithContext(ctx)
	prom, fut := NewPromise[[]adt.Result[T]](tg.Context())
	watchCancel(fut, func(c error) { cancelAll(sources, c) })

	out := make([]adt.Result[T], len(sources))
	go func() {
		tg.Run(sources, func(i int, res adt.Result[T]) bool {
			out[i] = res
			return false
		})
		prom.Resolve(out)
	}()
	return fut
}

// Race returns a Future that settles with the result (OK or Err) of the first
// source that completes. All other pending sources are cancelled.
// If sources is empty, the returned Future is rejected with ErrEmptySources.
func Race[T any](ctx context.Context, sources ...Receiver[T]) *Future[T] {
	if len(sources) == 0 {
		return rejectedFuture[T](ctx, ErrEmptySources)
	}
	tg := TaskGroupWithContext(ctx)
	prom, fut := NewPromise[T](tg.Context())
	watchCancel(fut, func(c error) { cancelAll(sources, c) })

	var once sync.Once
	go func() {
		tg.Run(sources, func(_ int, res adt.Result[T]) bool {
			once.Do(func() { res.Fold(prom.Resolve, prom.Reject) })
			return true
		})
	}()
	return fut
}

// AnyOf returns a Future that resolves with the value of the first source that
// succeeds. If a source fails, other sources continue executing.
// The returned Future is rejected with ErrAllFailed only if all sources fail.
// If sources is empty, the returned Future is rejected with ErrEmptySources.
func AnyOf[T any](ctx context.Context, sources ...Receiver[T]) *Future[T] {
	if len(sources) == 0 {
		return rejectedFuture[T](ctx, ErrEmptySources)
	}
	tg := TaskGroupWithContext(ctx)
	prom, fut := NewPromise[T](tg.Context())
	watchCancel(fut, func(c error) { cancelAll(sources, c) })

	var once sync.Once
	errs := make([]error, len(sources)+1)
	errs[0] = ErrAllFailed
	go func() {
		tg.Run(sources, func(i int, res adt.Result[T]) bool {
			return res.Fold(
				func(v T) bool {
					once.Do(func() { prom.Resolve(v) })
					return true
				},
				func(e error) bool {
					errs[i+1] = e
					return false
				},
			)
		})
		once.Do(func() { prom.Reject(errors.Join(errs...)) })
	}()
	return fut
}

// Zip2 combines two heterogeneous sources into a Future of adt.Pair.
// If either source fails, the combined future fails fast and cancels the other.
func Zip2[A, B any](ctx context.Context, sa Receiver[A], sb Receiver[B]) *Future[adt.Pair[A, B]] {
	tg := TaskGroupWithContext(ctx)
	prom, fut := NewPromise[adt.Pair[A, B]](tg.Context())
	watchCancel(fut, func(c error) { sa.Future().Cancel(c); sb.Future().Cancel(c) })

	var a A
	var b B
	var once sync.Once
	fail := func(err error) {
		once.Do(func() { prom.Reject(err) })
		sa.Future().Cancel(err)
		sb.Future().Cancel(err)
	}
	zipBranch(tg, sa, func(v A) { a = v }, fail)
	zipBranch(tg, sb, func(v B) { b = v }, fail)

	go func() {
		tg.wg.Wait()
		once.Do(func() { prom.Resolve(adt.PairOf(a, b)) })
	}()
	return fut
}

// Zip3 combines three heterogeneous sources into a Future of adt.Triple.
// If any source fails, the combined future fails fast and cancels the others.
func Zip3[A, B, C any](ctx context.Context, sa Receiver[A], sb Receiver[B], sc Receiver[C]) *Future[adt.Triple[A, B, C]] {
	tg := TaskGroupWithContext(ctx)
	prom, fut := NewPromise[adt.Triple[A, B, C]](tg.Context())
	watchCancel(fut, func(c error) { sa.Future().Cancel(c); sb.Future().Cancel(c); sc.Future().Cancel(c) })

	var a A
	var b B
	var c C
	var once sync.Once
	fail := func(err error) {
		once.Do(func() { prom.Reject(err) })
		sa.Future().Cancel(err)
		sb.Future().Cancel(err)
		sc.Future().Cancel(err)
	}
	zipBranch(tg, sa, func(v A) { a = v }, fail)
	zipBranch(tg, sb, func(v B) { b = v }, fail)
	zipBranch(tg, sc, func(v C) { c = v }, fail)

	go func() {
		tg.wg.Wait()
		once.Do(func() { prom.Resolve(adt.TripleOf(a, b, c)) })
	}()
	return fut
}

func zipBranch[T any](tg *TaskGroup, s Receiver[T], onVal func(T), onErr func(error)) {
	tg.wg.Go(func() {
		s.Future().AwaitCtx(tg.ctx).
			Tap(onVal).
			TapErr(func(err error) {
				onErr(err)
				tg.cancel(err)
			})
	})
}

func watchCancel[T any](fut *Future[T], onCancel func(error)) {
	go func() {
		<-fut.Context().Done()
		onCancel(context.Cause(fut.Context()))
	}()
}

func resolvedFuture[T any](ctx context.Context, val T) *Future[T] {
	prom, fut := NewPromise[T](ctx)
	prom.Resolve(val)
	return fut
}

func rejectedFuture[T any](ctx context.Context, err error) *Future[T] {
	prom, fut := NewPromise[T](ctx)
	prom.Reject(err)
	return fut
}

func cancelAll[T any](sources []Receiver[T], cause error) {
	for _, s := range sources {
		s.Future().Cancel(cause)
	}
}
