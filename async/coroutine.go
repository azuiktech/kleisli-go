package async

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/azuiktech/kleisli-go/adt"
)

var (
	// ErrTaskClosed is returned when Receive is called on a task whose input
	// channel has been closed.
	ErrTaskClosed = errors.New("task is closed or completed")
	// ErrStepCanceled is returned when the active step is cancelled via CancelCurrent.
	ErrStepCanceled = errors.New("task step was canceled")
)

// Promise is the coroutine's own execution handle — the typed suspension point
// the goroutine uses to receive inputs, emit side-channel values, and inspect
// its own lifetime context.
type Promise[I any] struct {
	ctx        context.Context
	cancel     context.CancelFunc
	inputCh    chan I
	stepMu     sync.Mutex
	stepCtx    context.Context
	stepCancel context.CancelFunc
	onEmitFn   func(any)
}

// Context returns the overall lifetime context of the task.
func (p *Promise[I]) Context() context.Context { return p.ctx }

// StepContext returns the context for the currently active step.
func (p *Promise[I]) StepContext() context.Context {
	p.stepMu.Lock()
	defer p.stepMu.Unlock()
	if p.stepCtx != nil {
		return p.stepCtx
	}
	return p.ctx
}

// withStep registers a per-step context that CancelCurrent can independently cancel.
func withStep[I any](p *Promise[I]) context.Context {
	p.stepMu.Lock()
	ctx, cancel := context.WithCancel(p.ctx)
	p.stepCtx, p.stepCancel = ctx, cancel
	p.stepMu.Unlock()
	return ctx
}

// endStep cancels and clears the current step context.
func endStep[I any](p *Promise[I]) {
	p.stepMu.Lock()
	if p.stepCancel != nil {
		p.stepCancel()
		p.stepCtx, p.stepCancel = nil, nil
	}
	p.stepMu.Unlock()
}

// Receive suspends until Send delivers a value, or the task/step is cancelled.
// Returns None on cancellation or close.
func Receive[I any](p *Promise[I]) adt.Option[I] {
	return ReceiveResult[I](p).ToOption()
}

// ReceiveResult is Receive returning a Result: Err on cancel/close, OK on delivery.
func ReceiveResult[I any](p *Promise[I]) adt.Result[I] {
	stepCtx := withStep(p)
	defer endStep(p)
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

// Emit sends a side-channel value to OnEmit listeners without suspending.
func Emit[I, E any](p *Promise[I], val E) {
	if p.onEmitFn != nil {
		p.onEmitFn(val)
	}
}

// Yield atomically emits val and suspends to receive the next input.
func Yield[I, E any](p *Promise[I], val E) adt.Result[I] {
	Emit[I, E](p, val)
	return ReceiveResult[I](p)
}

// Task drives a typed coroutine: I is the input type, O is the output type.
type Task[I, O any] struct {
	parentCtx context.Context
	promise   *Promise[I]
	doneCh    chan struct{} // closed once on completion; safe to read result after
	once      sync.Once
	result    adt.Result[O]

	cbMu          sync.Mutex
	done          bool
	doneCallbacks []func(adt.Result[O])
	emitCallbacks []func(any)
	emittedValues []any
}

// Launch spawns a coroutine with typed input I and output O.
// The body returns Result[O] directly — Err propagates without panic.
func Launch[I, O any](ctx context.Context, fn func(*Promise[I]) adt.Result[O]) *Task[I, O] {
	return LaunchConfig[I, O](ctx, 64, fn)
}

// LaunchConfig is Launch with an explicit input-buffer capacity (0 = strict rendezvous).
func LaunchConfig[I, O any](ctx context.Context, bufferCap int, fn func(*Promise[I]) adt.Result[O]) *Task[I, O] {
	promCtx, cancel := context.WithCancel(ctx)
	promise := &Promise[I]{
		ctx:     promCtx,
		cancel:  cancel,
		inputCh: make(chan I, bufferCap),
	}
	t := &Task[I, O]{
		parentCtx: ctx,
		promise:   promise,
		doneCh:    make(chan struct{}),
	}
	promise.onEmitFn = t.dispatchEmit
	go func() {
		defer func() {
			if r := recover(); r != nil {
				t.complete(adt.Err[O](panicToErr(r)))
			}
			promise.cancel()
		}()
		t.complete(fn(promise))
	}()
	return t
}

func panicToErr(r any) error {
	if err, ok := r.(error); ok {
		return err
	}
	return fmt.Errorf("task panic: %v", r)
}

func (t *Task[I, O]) complete(res adt.Result[O]) {
	t.once.Do(func() {
		t.cbMu.Lock()
		t.result = res
		t.done = true
		cbs := append([]func(adt.Result[O]){}, t.doneCallbacks...)
		t.cbMu.Unlock()
		close(t.doneCh)
		for _, cb := range cbs {
			cb(res)
		}
	})
}

func (t *Task[I, O]) dispatchEmit(val any) {
	t.cbMu.Lock()
	t.emittedValues = append(t.emittedValues, val)
	cbs := append([]func(any){}, t.emitCallbacks...)
	t.cbMu.Unlock()
	for _, cb := range cbs {
		cb(val)
	}
}

// Send delivers val to the coroutine.
// Returns false if the task is already done or cancelled.
func (t *Task[I, O]) Send(val I) bool {
	select {
	case t.promise.inputCh <- val:
		return true
	case <-t.promise.ctx.Done():
		return false
	case <-t.doneCh:
		return false
	}
}

// Await blocks until the task completes and returns its Result.
func (t *Task[I, O]) Await() adt.Result[O] {
	<-t.doneCh
	return t.result
}

// AwaitCtx blocks until the task completes or ctx expires.
// Does not cancel the background task on timeout.
func (t *Task[I, O]) AwaitCtx(ctx context.Context) adt.Result[O] {
	select {
	case <-t.doneCh:
		return t.result
	case <-ctx.Done():
		return adt.Err[O](ctx.Err())
	}
}

// AwaitTimeout blocks at most d for the task to complete.
func (t *Task[I, O]) AwaitTimeout(d time.Duration) adt.Result[O] {
	ctx, cancel := context.WithTimeout(context.Background(), d)
	defer cancel()
	return t.AwaitCtx(ctx)
}

// Map transforms a successful output into P, propagating errors.
func (t *Task[I, O]) Map[P any](fn func(O) P) *Task[I, P] {
	return Launch[I, P](t.parentCtx, func(_ *Promise[I]) adt.Result[P] {
		return t.Await().Map(fn)
	})
}

// FlatMap chains another Task onto a successful output.
func (t *Task[I, O]) FlatMap[P any](fn func(O) *Task[I, P]) *Task[I, P] {
	return Launch[I, P](t.parentCtx, func(_ *Promise[I]) adt.Result[P] {
		return t.Await().FlatMap(func(o O) adt.Result[P] {
			return fn(o).Await()
		})
	})
}

// Then chains a fallible Go-idiomatic function onto a successful output.
func (t *Task[I, O]) Then[P any](fn func(O) (P, error)) *Task[I, P] {
	return Launch[I, P](t.parentCtx, func(_ *Promise[I]) adt.Result[P] {
		return t.Await().Then(fn)
	})
}

// OnDone registers fn, calling it immediately if the task is already complete.
func (t *Task[I, O]) OnDone(fn func(adt.Result[O])) *Task[I, O] {
	t.cbMu.Lock()
	if t.done {
		res := t.result
		t.cbMu.Unlock()
		fn(res)
		return t
	}
	t.doneCallbacks = append(t.doneCallbacks, fn)
	t.cbMu.Unlock()
	return t
}

// OnReturn is OnDone filtered to successful completions.
func (t *Task[I, O]) OnReturn(fn func(O)) *Task[I, O] {
	return t.OnDone(func(res adt.Result[O]) {
		if res.IsOK() {
			fn(res.MustGet())
		}
	})
}

// OnErr is OnDone filtered to error and cancellation completions.
func (t *Task[I, O]) OnErr(fn func(error)) *Task[I, O] {
	return t.OnDone(func(res adt.Result[O]) {
		if res.IsErr() {
			fn(res.MustErr())
		}
	})
}

// OnEmit registers fn for intermediate emissions, replaying any past values.
func (t *Task[I, O]) OnEmit(fn func(any)) *Task[I, O] {
	t.cbMu.Lock()
	past := append([]any{}, t.emittedValues...)
	t.emitCallbacks = append(t.emitCallbacks, fn)
	t.cbMu.Unlock()
	for _, val := range past {
		fn(val)
	}
	return t
}

// OnEmitAs registers a typed OnEmit handler — fn is only called for emissions of type E.
func OnEmitAs[I, O, E any](t *Task[I, O], fn func(E)) *Task[I, O] {
	return t.OnEmit(func(val any) {
		if typed, ok := val.(E); ok {
			fn(typed)
		}
	})
}

// CancelCurrent cancels the coroutine's currently executing step.
func (t *Task[I, O]) CancelCurrent() {
	t.promise.stepMu.Lock()
	defer t.promise.stepMu.Unlock()
	if t.promise.stepCancel != nil {
		t.promise.stepCancel()
	}
}

// Cancel terminates the task. If already complete, this is a no-op.
func (t *Task[I, O]) Cancel() {
	t.complete(adt.Err[O](context.Canceled))
	t.promise.cancel()
}

// IsDone reports whether the task has completed (success, error, or cancellation).
func (t *Task[I, O]) IsDone() bool {
	t.cbMu.Lock()
	defer t.cbMu.Unlock()
	return t.done
}

// cancelAll cancels every task in the slice.
func cancelAll[I, O any](tasks []*Task[I, O]) {
	for _, t := range tasks {
		t.Cancel()
	}
}

// All runs tasks concurrently, collecting results in index order.
// Fails fast on the first error, cancelling remaining tasks.
func All[I, O any](ctx context.Context, tasks ...*Task[I, O]) *Task[I, []O] {
	return Launch[I, []O](ctx, func(p *Promise[I]) adt.Result[[]O] {
		if len(tasks) == 0 {
			return adt.OK([]O{})
		}
		type indexed struct {
			i   int
			res adt.Result[O]
		}
		ch := make(chan indexed, len(tasks))
		for i, t := range tasks {
			go func(idx int, tsk *Task[I, O]) {
				ch <- indexed{idx, tsk.AwaitCtx(p.Context())}
			}(i, t)
		}
		results := make([]O, len(tasks))
		for range len(tasks) {
			select {
			case ir := <-ch:
				if ir.res.IsErr() {
					cancelAll(tasks)
					return adt.Err[[]O](ir.res.MustErr())
				}
				results[ir.i] = ir.res.MustGet()
			case <-p.Context().Done():
				cancelAll(tasks)
				return adt.Err[[]O](p.Context().Err())
			}
		}
		return adt.OK(results)
	})
}

// Race returns the result of the first task to complete (success or error),
// cancelling all remaining tasks.
func Race[I, O any](ctx context.Context, tasks ...*Task[I, O]) *Task[I, O] {
	return Launch[I, O](ctx, func(p *Promise[I]) adt.Result[O] {
		if len(tasks) == 0 {
			return adt.Err[O](errors.New("race: no tasks provided"))
		}
		ch := make(chan adt.Result[O], len(tasks))
		for _, t := range tasks {
			go func(tsk *Task[I, O]) {
				ch <- tsk.AwaitCtx(p.Context())
			}(t)
		}
		select {
		case res := <-ch:
			cancelAll(tasks)
			return res
		case <-p.Context().Done():
			cancelAll(tasks)
			return adt.Err[O](p.Context().Err())
		}
	})
}

// Any returns the first task to succeed; if all tasks fail, combines all errors.
func Any[I, O any](ctx context.Context, tasks ...*Task[I, O]) *Task[I, O] {
	return Launch[I, O](ctx, func(p *Promise[I]) adt.Result[O] {
		if len(tasks) == 0 {
			return adt.Err[O](errors.New("any: no tasks provided"))
		}
		ch := make(chan adt.Result[O], len(tasks))
		for _, t := range tasks {
			go func(tsk *Task[I, O]) {
				ch <- tsk.AwaitCtx(p.Context())
			}(t)
		}
		var errs []error
		for range len(tasks) {
			select {
			case res := <-ch:
				if res.IsOK() {
					cancelAll(tasks)
					return res
				}
				errs = append(errs, res.MustErr())
			case <-p.Context().Done():
				cancelAll(tasks)
				return adt.Err[O](p.Context().Err())
			}
		}
		return adt.Err[O](errors.Join(errs...))
	})
}
