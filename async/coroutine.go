package async

import (
	"context"
	"errors"
	"fmt"

	"github.com/azuiktech/kleisli-go/adt"
)

var (
	noopEmit = func(any) {}
	noopCall = func(_ any, p *Promise[any]) {
		p.Reject(errors.New("no OnCall handler registered"))
	}
)

// Config configures the context and listeners for a launched coroutine.
type Config struct {
	Context context.Context
	OnEmit  func(val any)
	OnCall  func(req any, p *Promise[any])
}

// Co is the execution and communication scope passed to the coroutine function.
type Co[I, O any] struct {
	ctx      context.Context
	cancel   context.CancelCauseFunc
	onEmitFn func(any)
	onCallFn func(any, *Promise[any])
}

// Context returns the execution context of the coroutine.
func (c *Co[I, O]) Context() context.Context { return c.ctx }

// Emit sends a fire-and-forget notification directly to the caller's OnEmit listener.
func (c *Co[I, O]) Emit(val any) { c.onEmitFn(val) }

// Async spawns an asynchronous computation as a child of this coroutine, returning a Future.
func (c *Co[I, O]) Async[R any](fn func(ctx context.Context) adt.Result[R]) *Future[R] {
	prom, fut := NewPromise[R](c.ctx)
	go func() {
		defer settlePanic(prom)
		fn(fut.Context()).Fold(prom.Resolve, prom.Reject)
	}()
	return fut
}

// Call sends a bidirectional request to the caller's OnCall listener, returning a Future.
func (c *Co[I, O]) Call[R any](req any) *Future[R] {
	promR, futR := NewPromise[R](c.ctx)
	promAny, futAny := NewPromise[any](c.ctx)
	c.onCallFn(req, promAny)
	go func() {
		futAny.Await().Fold(
			func(val any) adt.Unit {
				v, ok := val.(R)
				adt.FromOk(v, ok).Fold(
					promR.Resolve,
					func() bool {
						var zero R
						return promR.Reject(fmt.Errorf("call response type mismatch: expected %T, got %T", zero, val))
					},
				)
				return adt.Void
			},
			func(err error) adt.Unit {
				promR.Reject(err)
				return adt.Void
			},
		)
	}()
	return futR
}

// Task is the caller-side driver for a launched coroutine.
type Task[I, O any] struct {
	fut *Future[O]
	co  *Co[I, O]
}

// Launch spawns a coroutine configured with the given Config and function.
func Launch[I, O any](cfg Config, fn func(*Co[I, O]) adt.Result[O]) *Task[I, O] {
	baseCtx := adt.Opt(cfg.Context).OrElse(context.Background())
	coCtx, coCancel := context.WithCancelCause(baseCtx)
	prom, fut := NewPromise[O](coCtx)

	co := &Co[I, O]{
		ctx:      coCtx,
		cancel:   coCancel,
		onEmitFn: adt.Opt(cfg.OnEmit).OrElse(noopEmit),
		onCallFn: adt.Opt(cfg.OnCall).OrElse(noopCall),
	}

	task := &Task[I, O]{
		fut: fut,
		co:  co,
	}

	go func() {
		defer coCancel(context.Canceled)
		defer settlePanic(prom)
		fn(co).Fold(prom.Resolve, prom.Reject)
	}()

	return task
}

// Future returns the read-only Future handle.
func (t *Task[I, O]) Future() *Future[O] { return t.fut }

// Await blocks until the coroutine completes and returns its final Result.
func (t *Task[I, O]) Await() adt.Result[O] { return t.fut.Await() }

// AwaitCtx blocks until the coroutine completes or ctx expires.
func (t *Task[I, O]) AwaitCtx(ctx context.Context) adt.Result[O] { return t.fut.AwaitCtx(ctx) }

// Cancel cancels the coroutine with the given cause.
// Returns true if this call cancelled the coroutine, or false if already settled.
func (t *Task[I, O]) Cancel(cause error) bool {
	err := adt.Opt(cause).OrElse(context.Canceled)
	cancelled := t.fut.Cancel(err)
	adt.FromOk(t.co.cancel, cancelled).Tap(func(cancel context.CancelCauseFunc) {
		cancel(err)
	})
	return cancelled
}

// Context returns the context of the coroutine.
func (t *Task[I, O]) Context() context.Context { return t.co.ctx }

func settlePanic[T any](p *Promise[T]) {
	adt.Opt(recover()).Tap(func(r any) {
		p.Reject(panicToErr(r))
	})
}

func panicToErr(r any) error {
	err, ok := r.(error)
	return adt.FromOk(err, ok).OrElseGet(func() error {
		return fmt.Errorf("coroutine panic: %v", r)
	})
}
