package async

import (
	"context"
	"errors"
	"sync"

	"github.com/azuiktech/kleisli-go/adt"
)

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
