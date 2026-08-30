package async

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/azuiktech/kleisli-go/adt"
)

var (
	// ErrTaskClosed is returned when operations are performed on a completed or closed task.
	ErrTaskClosed = errors.New("task is closed or completed")
	// ErrStepCanceled is returned when the currently active step is cancelled by the caller.
	ErrStepCanceled = errors.New("task step was canceled")
	// ErrTaskTimeout is returned when an Await call times out.
	ErrTaskTimeout = errors.New("task await timed out")
)

// Promise represents the execution and suspension context inside a task.
type Promise[I any] struct {
	ctx        context.Context
	cancel     context.CancelFunc
	inputCh    chan I
	stepCtx    context.Context
	stepCancel context.CancelFunc
	stepMu     sync.Mutex
	done       atomic.Bool
	onEmitFn   func(any)
}

// Context returns the overall lifetime context of the promise/task.
func (p *Promise[I]) Context() context.Context {
	return p.ctx
}

// StepContext returns the context for the currently executing step (cancelled via CancelCurrent()).
func (p *Promise[I]) StepContext() context.Context {
	p.stepMu.Lock()
	defer p.stepMu.Unlock()
	if p.stepCtx != nil {
		return p.stepCtx
	}
	return p.ctx
}

// Receive suspends the task until the caller feeds a value via task.Send(val).
// It returns an Option[I] holding the value, or None if the step/task was cancelled or closed.
func Receive[I any](p *Promise[I]) adt.Option[I] {
	return ReceiveResult[I](p).ToOption()
}

// ReceiveResult suspends the task and returns the received value wrapped in an adt.Result[I].
func ReceiveResult[I any](p *Promise[I]) adt.Result[I] {
	p.stepMu.Lock()
	stepCtx, stepCancel := context.WithCancel(p.ctx)
	p.stepCtx = stepCtx
	p.stepCancel = stepCancel
	p.stepMu.Unlock()

	defer func() {
		p.stepMu.Lock()
		if p.stepCancel != nil {
			p.stepCancel()
			p.stepCancel = nil
			p.stepCtx = nil
		}
		p.stepMu.Unlock()
	}()

	select {
	case val, ok := <-p.inputCh:
		if !ok {
			return adt.Err[I](ErrTaskClosed)
		}
		return adt.OK(val)
	case <-stepCtx.Done():
		return adt.Err[I](ErrStepCanceled)
	case <-p.ctx.Done():
		return adt.Err[I](p.ctx.Err())
	}
}

// Emit sends an intermediate notification or query of type E to the caller without terminating the task.
func Emit[I, E any](p *Promise[I], val E) {
	if p.onEmitFn != nil {
		p.onEmitFn(val)
	}
}

// Yield atomically emits val to the caller and suspends to receive the caller's next input of type I.
func Yield[I, E any](p *Promise[I], val E) adt.Result[I] {
	Emit[I, E](p, val)
	return ReceiveResult[I](p)
}

// Task represents a running asynchronous computation/coroutine and provides
// methods to drive, observe, await, compose, and cancel it.
type Task[I, O any] struct {
	parentCtx context.Context
	promise   *Promise[I]
	resultCh  chan adt.Result[O]
	done      atomic.Bool

	finalMu  sync.Mutex
	finalVal adt.Result[O]
	hasFinal bool

	doneCallbacks []func(adt.Result[O])
	emitCallbacks []func(any)
	cbMu          sync.Mutex
}

// Launch spawns a new task with typed input I and output O.
func Launch[I, O any](ctx context.Context, fn func(p *Promise[I]) O) *Task[I, O] {
	return LaunchConfig[I, O](ctx, 64, fn)
}

// LaunchConfig spawns a new task with a specified input buffer capacity (0 for strict rendezvous).
func LaunchConfig[I, O any](ctx context.Context, bufferCap int, fn func(p *Promise[I]) O) *Task[I, O] {
	promiseCtx, cancel := context.WithCancel(ctx)
	promise := &Promise[I]{
		ctx:     promiseCtx,
		cancel:  cancel,
		inputCh: make(chan I, bufferCap),
	}

	t := &Task[I, O]{
		parentCtx: ctx,
		promise:   promise,
		resultCh:  make(chan adt.Result[O], 1),
	}
	promise.onEmitFn = t.dispatchEmit

	go func() {
		defer func() {
			if r := recover(); r != nil {
				t.complete(adt.Err[O](fmt.Errorf("task panic: %v", r)))
			}
			promise.done.Store(true)
			promise.cancel()
		}()

		result := fn(promise)
		t.complete(adt.OK(result))
	}()

	return t
}

func (t *Task[I, O]) complete(res adt.Result[O]) {
	t.finalMu.Lock()
	if t.hasFinal {
		t.finalMu.Unlock()
		return
	}
	t.finalVal = res
	t.hasFinal = true
	t.done.Store(true)
	t.finalMu.Unlock()

	select {
	case t.resultCh <- res:
	default:
	}

	t.cbMu.Lock()
	cbs := make([]func(adt.Result[O]), len(t.doneCallbacks))
	copy(cbs, t.doneCallbacks)
	t.cbMu.Unlock()

	for _, cb := range cbs {
		cb(res)
	}
}

func (t *Task[I, O]) dispatchEmit(val any) {
	t.cbMu.Lock()
	cbs := make([]func(any), len(t.emitCallbacks))
	copy(cbs, t.emitCallbacks)
	t.cbMu.Unlock()

	for _, cb := range cbs {
		cb(val)
	}
}

// Send feeds input into the task. Returns true if delivered into the task,
// or false if the task is closed, completed, or cancelled.
func (t *Task[I, O]) Send(val I) bool {
	if t.done.Load() || t.promise.ctx.Err() != nil {
		return false
	}
	select {
	case t.promise.inputCh <- val:
		return true
	case <-t.promise.ctx.Done():
		return false
	}
}

// Await blocks until the task returns its final output or encounters an error.
// Guaranteed race-free against early cancellation.
func (t *Task[I, O]) Await() adt.Result[O] {
	t.finalMu.Lock()
	if t.hasFinal {
		res := t.finalVal
		t.finalMu.Unlock()
		return res
	}
	t.finalMu.Unlock()

	// Drain resultCh first if available to avoid cancel race
	select {
	case res := <-t.resultCh:
		return res
	default:
	}

	select {
	case res := <-t.resultCh:
		return res
	case <-t.promise.ctx.Done():
		// Check once more in case completion occurred right before cancellation
		select {
		case res := <-t.resultCh:
			return res
		default:
			t.finalMu.Lock()
			if t.hasFinal {
				res := t.finalVal
				t.finalMu.Unlock()
				return res
			}
			t.finalMu.Unlock()
			return adt.Err[O](t.promise.ctx.Err())
		}
	}
}

// AwaitCtx blocks until the task finishes or the provided caller context expires.
// Does NOT cancel the background task itself on timeout.
func (t *Task[I, O]) AwaitCtx(ctx context.Context) adt.Result[O] {
	t.finalMu.Lock()
	if t.hasFinal {
		res := t.finalVal
		t.finalMu.Unlock()
		return res
	}
	t.finalMu.Unlock()

	select {
	case res := <-t.resultCh:
		return res
	case <-ctx.Done():
		return adt.Err[O](ctx.Err())
	case <-t.promise.ctx.Done():
		return t.Await()
	}
}

// AwaitTimeout blocks up to duration d.
func (t *Task[I, O]) AwaitTimeout(d time.Duration) adt.Result[O] {
	ctx, cancel := context.WithTimeout(context.Background(), d)
	defer cancel()
	return t.AwaitCtx(ctx)
}

// Map transforms the output of the task upon successful completion.
func (t *Task[I, O]) Map[P any](fn func(O) P) *Task[I, P] {
	return Launch[I, P](t.parentCtx, func(p *Promise[I]) P {
		res := t.Await()
		if res.IsErr() {
			panic(res.MustErr())
		}
		return fn(res.MustGet())
	})
}

// FlatMap chains another task-returning computation onto this task.
func (t *Task[I, O]) FlatMap[P any](fn func(O) *Task[I, P]) *Task[I, P] {
	return Launch[I, P](t.parentCtx, func(p *Promise[I]) P {
		res := t.Await()
		if res.IsErr() {
			panic(res.MustErr())
		}
		nextTask := fn(res.MustGet())
		nextRes := nextTask.Await()
		if nextRes.IsErr() {
			panic(nextRes.MustErr())
		}
		return nextRes.MustGet()
	})
}

// Then chains a fallible function onto this task.
func (t *Task[I, O]) Then[P any](fn func(O) (P, error)) *Task[I, P] {
	return Launch[I, P](t.parentCtx, func(p *Promise[I]) P {
		res := t.Await()
		if res.IsErr() {
			panic(res.MustErr())
		}
		val, err := fn(res.MustGet())
		if err != nil {
			panic(err)
		}
		return val
	})
}

// OnDone registers an asynchronous callback triggered when the task finishes (success or error).
func (t *Task[I, O]) OnDone(fn func(res adt.Result[O])) *Task[I, O] {
	t.finalMu.Lock()
	if t.hasFinal {
		res := t.finalVal
		t.finalMu.Unlock()
		fn(res)
		return t
	}
	t.finalMu.Unlock()

	t.cbMu.Lock()
	t.doneCallbacks = append(t.doneCallbacks, fn)
	t.cbMu.Unlock()
	return t
}

// OnReturn registers an asynchronous callback triggered only on successful return.
func (t *Task[I, O]) OnReturn(fn func(val O)) *Task[I, O] {
	return t.OnDone(func(res adt.Result[O]) {
		if res.IsOK() {
			fn(res.MustGet())
		}
	})
}

// OnErr registers an asynchronous callback triggered only on error or cancellation.
func (t *Task[I, O]) OnErr(fn func(err error)) *Task[I, O] {
	return t.OnDone(func(res adt.Result[O]) {
		if res.IsErr() {
			fn(res.MustErr())
		}
	})
}

// OnEmit registers an asynchronous callback triggered whenever Emit/Yield is called inside the task.
func (t *Task[I, O]) OnEmit(fn func(val any)) *Task[I, O] {
	t.cbMu.Lock()
	t.emitCallbacks = append(t.emitCallbacks, fn)
	t.cbMu.Unlock()
	return t
}

// CancelCurrent cancels only the currently executing/suspended step via StepContext().
func (t *Task[I, O]) CancelCurrent() {
	t.promise.stepMu.Lock()
	defer t.promise.stepMu.Unlock()
	if t.promise.stepCancel != nil {
		t.promise.stepCancel()
	}
}

// Cancel terminates the entire task via Context().
func (t *Task[I, O]) Cancel() {
	t.promise.done.Store(true)
	t.promise.cancel()
}

// IsDone reports whether the task has completed or been cancelled.
func (t *Task[I, O]) IsDone() bool {
	return t.done.Load() || t.promise.ctx.Err() != nil
}

// All runs multiple tasks to completion, returning a task that collects all outputs.
// Fail-fast on the first encountered error.
func All[I, O any](ctx context.Context, tasks ...*Task[I, O]) *Task[I, []O] {
	return Launch[I, []O](ctx, func(p *Promise[I]) []O {
		results := make([]O, len(tasks))
		for i, t := range tasks {
			res := t.Await()
			if res.IsErr() {
				panic(res.MustErr())
			}
			results[i] = res.MustGet()
		}
		return results
	})
}

// Race returns a task that completes with the result of the first task to finish.
func Race[I, O any](ctx context.Context, tasks ...*Task[I, O]) *Task[I, O] {
	return Launch[I, O](ctx, func(p *Promise[I]) O {
		doneCh := make(chan adt.Result[O], len(tasks))
		for _, t := range tasks {
			go func(tsk *Task[I, O]) {
				doneCh <- tsk.Await()
			}(t)
		}
		res := <-doneCh
		if res.IsErr() {
			panic(res.MustErr())
		}
		return res.MustGet()
	})
}
