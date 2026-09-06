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

// Op represents a strongly-typed operation mapping a request of type Req to a response of type Resp.
type Op[Req, Resp any] struct {
	name string
}

// Name returns the operation's identifier.
func (o Op[Req, Resp]) Name() string { return o.name }

// DefineOp creates a strongly-typed operation mapping request Req to response Resp.
func DefineOp[Req, Resp any](name string) Op[Req, Resp] {
	return Op[Req, Resp]{name: adt.Opt(name).Filter(func(s string) bool { return s != "" }).OrElseGet(func() string {
		return fmt.Sprintf("%T->%T", *new(Req), *new(Resp))
	})}
}

// CallInvocation packages an operation with its request payload for dispatch.
type CallInvocation struct {
	OpName string
	Req    any
}

func (c CallInvocation) String() string {
	return fmt.Sprintf("%s(%v)", c.OpName, c.Req)
}

// Config configures the context and listeners for a launched coroutine.
type Config struct {
	Context context.Context
	OnEmit  func(val any)
	OnCall  func(req any, p *Promise[any])
}

// RegisterHandler registers a strongly-typed handler on Config for a specific Op[Req, Resp].
func RegisterHandler[Req, Resp any](cfg *Config, op Op[Req, Resp], fn func(req Req) adt.Result[Resp]) {
	prev := cfg.OnCall
	cfg.OnCall = func(raw any, p *Promise[any]) {
		if inv, ok := raw.(CallInvocation); ok && inv.OpName == op.name {
			if typedReq, ok := inv.Req.(Req); ok {
				fn(typedReq).Fold(
					func(resp Resp) adt.Unit {
						p.Resolve(resp)
						return adt.Void
					},
					func(err error) adt.Unit {
						p.Reject(err)
						return adt.Void
					},
				)
				return
			}
		}
		if prev != nil {
			prev(raw, p)
		}
	}
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

// Call sends a bidirectional request for operation op, returning a Future.
// Both the request type Req and response type Resp are strictly bound to op.
func (c *Co) Call[Req, Resp any](op Op[Req, Resp], req Req) *Future[Resp] {
	promResp, futResp := NewPromise[Resp](c)
	promAny, futAny := NewPromise[any](c)
	go func() {
		defer settlePanic(promResp)
		c.onCallFn(CallInvocation{OpName: op.Name(), Req: req}, promAny)
		futAny.Await().Fold(
			func(val any) adt.Unit {
				v, ok := val.(Resp)
				adt.FromOk(v, ok).Fold(
					promResp.Resolve,
					func() bool {
						var zero Resp
						return promResp.Reject(fmt.Errorf("call response type mismatch: expected %T, got %T", zero, val))
					},
				)
				return adt.Void
			},
			func(err error) adt.Unit {
				promResp.Reject(err)
				return adt.Void
			},
		)
	}()
	return futResp
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
