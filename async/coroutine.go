package async

import (
	"context"
	"fmt"

	"github.com/azuiktech/kleisli-go/adt"
)

var (
	noopEmit = func(any) {}
)

// Op represents a coroutine suspension effect: suspends with S, resumes with R.
type Op[S, R any] struct {
	name string
}

// Name returns the operation's identifier.
func (o Op[S, R]) Name() string { return o.name }

// DefineOp creates a strongly-typed operation mapping suspend type S to resume type R.
func DefineOp[S, R any](name string) Op[S, R] {
	return Op[S, R]{name: adt.Opt(name).Filter(func(s string) bool { return s != "" }).OrElseGet(func() string {
		return fmt.Sprintf("%T->%T", *new(S), *new(R))
	})}
}

// CallInvocation packages an operation with its suspension payload S for dispatch.
type CallInvocation struct {
	OpName string
	S      any
}

func (c CallInvocation) String() string {
	return fmt.Sprintf("%s(%v)", c.OpName, c.S)
}

// CallHandler binds an Op[S, R] to its strongly-typed resumption handler.
type CallHandler interface {
	OpName() string
	Handle(inv CallInvocation, p *Promise[any]) bool
}

type typedCallHandler[S, R any] struct {
	op Op[S, R]
	fn func(S) adt.Result[R]
}

func (h typedCallHandler[S, R]) OpName() string { return h.op.Name() }

func (h typedCallHandler[S, R]) Handle(inv CallInvocation, p *Promise[any]) bool {
	if inv.OpName != h.op.Name() {
		return false
	}
	typedS, ok := inv.S.(S)
	if !ok {
		var zero S
		p.Reject(fmt.Errorf("request type mismatch for operation %s: expected %T, got %T", h.op.Name(), zero, inv.S))
		return true
	}
	h.fn(typedS).Fold(
		func(r R) adt.Unit {
			p.Resolve(r)
			return adt.Void
		},
		func(err error) adt.Unit {
			p.Reject(err)
			return adt.Void
		},
	)
	return true
}

// Handle creates a strongly-typed CallHandler for operation op.
func (o Op[S, R]) Handle(fn func(s S) adt.Result[R]) CallHandler {
	return typedCallHandler[S, R]{op: o, fn: fn}
}

// Config configures the context and listeners for a launched coroutine.
type Config struct {
	Context Context       // Execution engine. If nil or *GoroutineContext, uses in-memory GoroutineContext
	OnEmit  func(val any) // Universal callback handler for emissions
	OnCall  []CallHandler // Universal operation handlers
}

// Co is the execution and communication scope passed to the coroutine function.
// It embeds context.Context directly so it satisfies the context.Context interface,
// with cancellation scoped to the active task.
type Co struct {
	context.Context
	engine   Context
	onEmitFn func(any)
}

// Emit sends a fire-and-forget notification directly to the caller's OnEmit listener.
func (c *Co) Emit(val any) { c.onEmitFn(val) }

// Async spawns an asynchronous computation as a child of this coroutine, returning a Future.
func (c *Co) Async[R any](fn func(ctx context.Context) adt.Result[R]) *Future[R] {
	promR, futR := NewPromise[R](c)
	futAny := c.engine.Async(func(ctx context.Context) adt.Result[any] {
		return fn(ctx).Map(func(r R) any { return any(r) })
	})
	go func() {
		defer settlePanic(promR)
		futAny.Await().Fold(
			func(val any) adt.Unit {
				v, ok := val.(R)
				adt.FromOk(v, ok).Fold(
					promR.Resolve,
					func() bool {
						var zero R
						return promR.Reject(fmt.Errorf("async result type mismatch: expected %T, got %T", zero, val))
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

// Call suspends the coroutine with payload s for operation op, returning a Future that resolves upon resumption with R.
func (c *Co) Call[S, R any](op Op[S, R], s S) *Future[R] {
	promR, futR := NewPromise[R](c)
	futAny := c.engine.Call(op.Name(), s)
	go func() {
		defer settlePanic(promR)
		futAny.Await().Fold(
			func(val any) adt.Unit {
				v, ok := val.(R)
				adt.FromOk(v, ok).Fold(
					promR.Resolve,
					func() bool {
						var zero R
						return promR.Reject(fmt.Errorf("call response type mismatch for op %s: expected %T, got %T", op.Name(), zero, val))
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

func dispatchCall(handlers []CallHandler, inv CallInvocation, prom *Promise[any]) bool {
	for _, h := range handlers {
		if h.Handle(inv, prom) {
			return true
		}
	}
	return false
}

// Task represents a packaged task (akin to std::packaged_task<O(I)>)
// that pairs a callable function with a Future handle and deferred execution.
type Task[I, O any] struct {
	fut *Future[O]
	run func(I) adt.Result[O]
}

// PackagedTask packages the coroutine function and configuration without executing it.
func PackagedTask[I, O any](cfg Config, fn func(*Co, I) adt.Result[O]) *Task[I, O] {
	baseCtx := adt.Opt[context.Context](cfg.Context).OrElse(context.Background())
	prom, fut := NewPromise[O](baseCtx)

	coCtx := adt.Opt(cfg.Context).OrElseGet(func() Context {
		return NewGoroutineContext(fut.Context())
	})
	if hc, ok := coCtx.(HandlerContext); ok && len(cfg.OnCall) > 0 {
		hc.SetHandlers(cfg.OnCall)
	}


	co := &Co{
		Context:  fut.Context(),
		engine:   coCtx,
		onEmitFn: adt.Opt(cfg.OnEmit).OrElse(noopEmit),
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
