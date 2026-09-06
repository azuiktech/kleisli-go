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
// It embeds context.Context directly so it satisfies the context.Context interface.
type Co struct {
	context.Context
	onEmitFn func(any)
	onCallFn func(any, *Promise[any])
}

// Emit sends a fire-and-forget notification directly to the caller's OnEmit listener.
func (c *Co) Emit(val any) { c.onEmitFn(val) }

// Async spawns an asynchronous computation as a child of this coroutine, returning a Future.
func (c *Co) Async[R any](fn func(ctx context.Context) adt.Result[R]) *Future[R] {
	prom, fut := NewPromise[R](c)
	go func() {
		defer settlePanic(prom)
		fn(fut.Context()).Fold(prom.Resolve, prom.Reject)
	}()
	return fut
}

// Call sends a bidirectional request to the caller's OnCall listener, returning a Future.
func (c *Co) Call[R any](req any) *Future[R] {
	promR, futR := NewPromise[R](c)
	promAny, futAny := NewPromise[any](c)
	go func() {
		defer settlePanic(promR)
		c.onCallFn(req, promAny)
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

// Task represents a packaged task (akin to std::packaged_task<O(I)>)
// that pairs a callable function with a Future handle and deferred execution.
type Task[I, O any] struct {
	fut *Future[O]
	run func(I) adt.Result[O]
}

// PackagedTask packages the coroutine function and configuration without executing it.
func PackagedTask[I, O any](cfg Config, fn func(*Co, I) adt.Result[O]) *Task[I, O] {
	prom, fut := NewPromise[O](cfg.Context)
	co := &Co{
		Context:  fut.Context(),
		onEmitFn: adt.Opt(cfg.OnEmit).OrElse(noopEmit),
		onCallFn: adt.Opt(cfg.OnCall).OrElse(noopCall),
	}
	return &Task[I, O]{
		fut: fut,
		run: func(in I) adt.Result[O] {
			defer settlePanic(prom)
			fn(co, in).Fold(prom.Resolve, prom.Reject)
			return fut.Await()
		},
	}
}

// Launch packages and immediately executes the task on a new goroutine.
func Launch[I, O any](cfg Config, in I, fn func(*Co, I) adt.Result[O]) *Task[I, O] {
	task := PackagedTask[I, O](cfg, fn)
	go task.Run(in)
	return task
}

// Run executes the packaged task with the given input.
// It can be invoked synchronously on the current goroutine, or submitted to a worker pool / goroutine.
func (t *Task[I, O]) Run(in I) adt.Result[O] { return t.run(in) }

// Future returns the read-only Future handle.
func (t *Task[I, O]) Future() *Future[O] { return t.fut }

// Await blocks until the coroutine completes and returns its final Result.
func (t *Task[I, O]) Await() adt.Result[O] { return t.fut.Await() }

// AwaitCtx blocks until the coroutine completes or ctx expires.
func (t *Task[I, O]) AwaitCtx(ctx context.Context) adt.Result[O] { return t.fut.AwaitCtx(ctx) }

// Cancel cancels the coroutine with the given cause.
// Returns true if this call cancelled the coroutine, or false if already settled.
func (t *Task[I, O]) Cancel(cause error) bool { return t.fut.Cancel(cause) }

// Context returns the context of the coroutine.
func (t *Task[I, O]) Context() context.Context { return t.fut.Context() }

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
