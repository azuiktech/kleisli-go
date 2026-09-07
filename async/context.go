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
	d.maxSteps = n
}

// Call executes an asymmetric operation or replays it from journal history.
func (d *DurableContext) Call(op string, s any) *Future[any] {
	prom, fut := NewPromise[any](d.Context)
	step := d.stepIndex
	d.stepIndex++

	// 1. Replay from journal if already completed in a prior turn
	if val, ok := d.journal.Get(step); ok {
		prom.Resolve(val)
		return fut
	}

	// 2. Suspend if this turn reached the step limit
	if d.maxSteps >= 0 && step >= d.maxSteps {
		prom.Reject(ErrSuspended)
		return fut
	}

	// 3. First execution of this step
	inv := CallInvocation{OpName: op, S: s}
	go func() {
		defer settlePanic(prom)
		promExec, futExec := NewPromise[any](d.Context)
		if !dispatchCall(d.handlers, inv, promExec) {
			prom.Reject(fmt.Errorf("no handler registered for operation: %s", op))
			return
		}
		res := futExec.Await()
		if res.IsErr() {
			prom.Reject(res.MustErr())
			return
		}
		val := res.MustGet()
		d.journal.Append(val)
		prom.Resolve(val)
	}()
	return fut
}

// Async executes a symmetric peer function or replays it from journal history.
func (d *DurableContext) Async(fn func(ctx context.Context) adt.Result[any]) *Future[any] {
	prom, fut := NewPromise[any](d.Context)
	step := d.stepIndex
	d.stepIndex++

	// 1. Replay from journal if already completed in a prior turn
	if val, ok := d.journal.Get(step); ok {
		prom.Resolve(val)
		return fut
	}

	// 2. Suspend if this turn reached the step limit
	if d.maxSteps >= 0 && step >= d.maxSteps {
		prom.Reject(ErrSuspended)
		return fut
	}

	// 3. First execution of this step
	go func() {
		defer settlePanic(prom)
		res := fn(fut.Context())
		if res.IsErr() {
			prom.Reject(res.MustErr())
			return
		}
		val := res.MustGet()
		d.journal.Append(val)
		prom.Resolve(val)
	}()
	return fut
}
