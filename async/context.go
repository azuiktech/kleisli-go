package async

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/azuiktech/kleisli-go/adt"
)

// Context defines the execution, cancellation, and effect engine for both
// asymmetric (Call) and symmetric (Async) coroutines.
type Context interface {
	context.Context
	Call(op string, s any) *Future[any]
	Async(fn func(ctx context.Context) adt.Result[any]) *Future[any]
}

// HandlerContext is implemented by Context implementations that support registering call handlers.
type HandlerContext interface {
	SetHandlers(handlers []CallHandler)
}

// GoroutineContext implements Context using in-memory goroutines and local handlers.
type GoroutineContext struct {
	context.Context
	handlers []CallHandler
}

// NewGoroutineContext creates an in-memory coroutine Context.
func NewGoroutineContext(ctx context.Context) *GoroutineContext {
	baseCtx := adt.Opt(ctx).OrElse(context.Background())
	return &GoroutineContext{Context: baseCtx}
}

// SetHandlers appends call handlers to the GoroutineContext.
func (g *GoroutineContext) SetHandlers(handlers []CallHandler) {
	g.handlers = append(g.handlers, handlers...)
}


func (g *GoroutineContext) Call(op string, s any) *Future[any] {
	prom, fut := NewPromise[any](g.Context)
	go func() {
		defer settlePanic(prom)
		inv := CallInvocation{OpName: op, S: s}
		if !dispatchCall(g.handlers, inv, prom) {
			prom.Reject(fmt.Errorf("no handler registered for operation: %s", op))
		}
	}()
	return fut
}

func (g *GoroutineContext) Async(fn func(ctx context.Context) adt.Result[any]) *Future[any] {
	prom, fut := NewPromise[any](g.Context)
	go func() {
		defer settlePanic(prom)
		fn(fut.Context()).Fold(prom.Resolve, prom.Reject)
	}()
	return fut
}

// ErrSuspended indicates that a coroutine has suspended execution waiting for external input or next turn.
var ErrSuspended = errors.New("coroutine suspended")

// Journal represents an in-memory or persisted step journal for durable execution.
type Journal struct {
	mu      sync.Mutex
	history []any
}

// NewJournal creates an empty Journal.
func NewJournal() *Journal {
	return &Journal{}
}

// Append records a completed step value to the journal.
func (j *Journal) Append(val any) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.history = append(j.history, val)
}

// Get returns the recorded value at step idx, if present.
func (j *Journal) Get(idx int) (any, bool) {
	j.mu.Lock()
	defer j.mu.Unlock()
	if idx < len(j.history) {
		return j.history[idx], true
	}
	return nil, false
}

// Len returns the number of recorded step entries in the journal.
func (j *Journal) Len() int {
	j.mu.Lock()
	defer j.mu.Unlock()
	return len(j.history)
}

// DurableContext implements Context with deterministic step replay across workflow turns.
type DurableContext struct {
	context.Context
	journal   *Journal
	mu        sync.Mutex
	handlers  []CallHandler
	stepIndex int
	maxSteps  int // maximum steps to execute before suspending (-1 = run until completion)
}

// NewDurableContext constructs a DurableContext backed by a Journal.
func NewDurableContext(ctx context.Context, journal *Journal) *DurableContext {
	baseCtx := adt.Opt(ctx).OrElse(context.Background())
	return &DurableContext{
		Context:  baseCtx,
		journal:  journal,
		maxSteps: -1,
	}
}

// SetMaxSteps limits the number of newly executed steps on this turn before suspending with ErrSuspended.
func (d *DurableContext) SetMaxSteps(n int) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.maxSteps = n
}

// SetHandlers appends call handlers to the DurableContext.
func (d *DurableContext) SetHandlers(handlers []CallHandler) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.handlers = append(d.handlers, handlers...)
}

func (d *DurableContext) allocStep() (any, bool, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	step := d.stepIndex
	d.stepIndex++
	val, replayed := d.journal.Get(step)
	suspended := !replayed && d.maxSteps >= 0 && step >= d.maxSteps
	return val, replayed, suspended
}

// Call executes an asymmetric operation or replays it from journal history.
func (d *DurableContext) Call(op string, s any) *Future[any] {
	prom, fut := NewPromise[any](d.Context)
	val, replayed, suspended := d.allocStep()

	if replayed {
		prom.Resolve(val)
		return fut
	}
	if suspended {
		prom.Reject(ErrSuspended)
		return fut
	}

	d.mu.Lock()
	handlers := append([]CallHandler(nil), d.handlers...)
	d.mu.Unlock()

	inv := CallInvocation{OpName: op, S: s}
	go func() {
		defer settlePanic(prom)
		promExec, futExec := NewPromise[any](d.Context)
		if !dispatchCall(handlers, inv, promExec) {
			prom.Reject(fmt.Errorf("no handler registered for operation: %s", op))
			return
		}
		res := futExec.Await()
		if res.IsErr() {
			prom.Reject(res.MustErr())
			return
		}
		resVal := res.MustGet()
		d.journal.Append(resVal)
		prom.Resolve(resVal)
	}()
	return fut
}

// Async executes a symmetric peer function or replays it from journal history.
func (d *DurableContext) Async(fn func(ctx context.Context) adt.Result[any]) *Future[any] {
	prom, fut := NewPromise[any](d.Context)
	val, replayed, suspended := d.allocStep()

	if replayed {
		prom.Resolve(val)
		return fut
	}
	if suspended {
		prom.Reject(ErrSuspended)
		return fut
	}

	go func() {
		defer settlePanic(prom)
		res := fn(fut.Context())
		if res.IsErr() {
			prom.Reject(res.MustErr())
			return
		}
		resVal := res.MustGet()
		d.journal.Append(resVal)
		prom.Resolve(resVal)
	}()
	return fut
}
