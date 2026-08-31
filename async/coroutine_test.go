package async

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/azuiktech/kleisli-go/adt"
)

func TestTask_TypedSendAndReceive(t *testing.T) {
	ctx := context.Background()

	task := Launch(ctx, func(p *Promise[int]) adt.Result[string] {
		a := p.Receive().OrElse(0)
		b := p.Receive().OrElse(0)
		return adt.OK(fmt.Sprintf("sum: %d", a+b))
	})

	time.Sleep(10 * time.Millisecond)
	if !task.Send(10) {
		t.Error("expected Send to return true")
	}
	time.Sleep(10 * time.Millisecond)
	if !task.Send(25) {
		t.Error("expected Send to return true")
	}

	res := task.Await()
	if res.IsErr() {
		t.Fatalf("unexpected error: %v", res.MustErr())
	}
	if res.MustGet() != "sum: 35" {
		t.Errorf("got %q, want %q", res.MustGet(), "sum: 35")
	}
	if task.Send(99) {
		t.Error("expected Send to closed task to return false")
	}
}

func TestTask_Yield_AtomicEmitReceive(t *testing.T) {
	ctx := context.Background()

	task := Launch(ctx, func(p *Promise[string]) adt.Result[string] {
		name := p.Yield( "what is your name?").OrElse("")
		city := p.Yield( "what is your city?").OrElse("")
		return adt.OK(fmt.Sprintf("%s from %s", name, city))
	})

	var emitted []any
	var mu sync.Mutex
	task.OnEmit(func(val any) {
		mu.Lock()
		defer mu.Unlock()
		emitted = append(emitted, val)
	})

	time.Sleep(10 * time.Millisecond)
	task.Send("Alice")
	time.Sleep(10 * time.Millisecond)
	task.Send("Bangalore")

	res := task.Await()
	if res.IsErr() || res.MustGet() != "Alice from Bangalore" {
		t.Errorf("got %v, want 'Alice from Bangalore'", res)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(emitted) != 2 || emitted[0] != "what is your name?" || emitted[1] != "what is your city?" {
		t.Errorf("emitted = %v", emitted)
	}
}

func TestTask_MonadicChaining(t *testing.T) {
	ctx := context.Background()

	base := Launch(ctx, func(p *Promise[int]) adt.Result[int] {
		x := p.Receive().OrElse(0)
		return adt.OK(x * 2)
	})

	mapped := base.Map(func(n int) string {
		return fmt.Sprintf("result: %d", n)
	})

	flatMapped := mapped.FlatMap(func(s string) *Task[int, string] {
		return Launch(ctx, func(p *Promise[int]) adt.Result[string] {
			return adt.OK(s + "!")
		})
	})

	finalTask := flatMapped.Then(func(s string) (string, error) {
		return "[" + s + "]", nil
	})

	time.Sleep(10 * time.Millisecond)
	base.Send(21)

	res := finalTask.Await()
	if res.IsErr() {
		t.Fatalf("unexpected error in chaining: %v", res.MustErr())
	}
	if res.MustGet() != "[result: 42!]" {
		t.Errorf("got %q, want '[result: 42!]'", res.MustGet())
	}
}

func TestTask_AwaitTimeout(t *testing.T) {
	ctx := context.Background()

	task := Launch(ctx, func(p *Promise[int]) adt.Result[int] {
		time.Sleep(100 * time.Millisecond)
		return adt.OK(42)
	})

	res := task.AwaitTimeout(10 * time.Millisecond)
	if res.IsOK() {
		t.Errorf("expected timeout error, got %v", res.MustGet())
	}

	// Task itself should NOT be cancelled and should eventually finish.
	finalRes := task.Await()
	if finalRes.IsErr() || finalRes.MustGet() != 42 {
		t.Errorf("expected final result 42, got %v", finalRes)
	}
}

func TestTask_All_ConcurrentExecution(t *testing.T) {
	ctx := context.Background()
	start := time.Now()

	// Each task takes 50ms. Sequential total >= 150ms; concurrent total ~50ms.
	t1 := Launch(ctx, func(p *Promise[unit]) adt.Result[int] {
		time.Sleep(50 * time.Millisecond)
		return adt.OK(10)
	})
	t2 := Launch(ctx, func(p *Promise[unit]) adt.Result[int] {
		time.Sleep(50 * time.Millisecond)
		return adt.OK(20)
	})
	t3 := Launch(ctx, func(p *Promise[unit]) adt.Result[int] {
		time.Sleep(50 * time.Millisecond)
		return adt.OK(30)
	})

	res := All(ctx, t1, t2, t3).Await()
	elapsed := time.Since(start)

	if res.IsErr() {
		t.Fatalf("All failed: %v", res.MustErr())
	}
	vals := res.MustGet()
	if len(vals) != 3 || vals[0] != 10 || vals[1] != 20 || vals[2] != 30 {
		t.Errorf("vals = %v, want [10, 20, 30]", vals)
	}
	if elapsed >= 140*time.Millisecond {
		t.Errorf("All executed sequentially! elapsed: %v", elapsed)
	}
}

func TestTask_Race_CancelsLosers(t *testing.T) {
	ctx := context.Background()

	var slowCancelled sync.WaitGroup
	slowCancelled.Add(1)

	slow := Launch(ctx, func(p *Promise[unit]) adt.Result[string] {
		select {
		case <-time.After(300 * time.Millisecond):
			return adt.OK("slow")
		case <-p.Context().Done():
			slowCancelled.Done()
			return adt.Err[string](p.Context().Err())
		}
	})

	fast := Launch(ctx, func(p *Promise[unit]) adt.Result[string] {
		time.Sleep(10 * time.Millisecond)
		return adt.OK("fast")
	})

	res := Race(ctx, slow, fast).Await()
	if res.IsErr() || res.MustGet() != "fast" {
		t.Errorf("Race got %v, want 'fast'", res)
	}

	done := make(chan struct{})
	go func() { slowCancelled.Wait(); close(done) }()

	select {
	case <-done:
	case <-time.After(100 * time.Millisecond):
		t.Error("Race did not cancel the losing task within 100ms")
	}
}

func TestTask_Any_FirstSuccessAndCancelsRest(t *testing.T) {
	ctx := context.Background()

	tFail := Launch(ctx, func(p *Promise[unit]) adt.Result[string] {
		time.Sleep(10 * time.Millisecond)
		return adt.Err[string](fmt.Errorf("task 1 failed"))
	})

	tSuccess := Launch(ctx, func(p *Promise[unit]) adt.Result[string] {
		time.Sleep(30 * time.Millisecond)
		return adt.OK("success from task 2")
	})

	var slowCancelled sync.WaitGroup
	slowCancelled.Add(1)

	tSlow := Launch(ctx, func(p *Promise[unit]) adt.Result[string] {
		select {
		case <-time.After(300 * time.Millisecond):
			return adt.OK("slow task 3")
		case <-p.Context().Done():
			slowCancelled.Done()
			return adt.Err[string](p.Context().Err())
		}
	})

	res := Any(ctx, tFail, tSuccess, tSlow).Await()
	if res.IsErr() {
		t.Fatalf("Any failed unexpectedly: %v", res.MustErr())
	}
	if res.MustGet() != "success from task 2" {
		t.Errorf("Any got %q, want 'success from task 2'", res.MustGet())
	}

	done := make(chan struct{})
	go func() { slowCancelled.Wait(); close(done) }()

	select {
	case <-done:
	case <-time.After(100 * time.Millisecond):
		t.Error("Any did not cancel the remaining task within 100ms")
	}
}

func TestTask_Any_AllFailed(t *testing.T) {
	ctx := context.Background()

	t1 := Launch(ctx, func(p *Promise[unit]) adt.Result[string] {
		return adt.Err[string](fmt.Errorf("err 1"))
	})
	t2 := Launch(ctx, func(p *Promise[unit]) adt.Result[string] {
		return adt.Err[string](fmt.Errorf("err 2"))
	})

	res := Any(ctx, t1, t2).Await()
	if res.IsOK() {
		t.Fatalf("expected Any to fail when all tasks fail, got %v", res.MustGet())
	}
	errMsg := res.MustErr().Error()
	if !strings.Contains(errMsg, "err 1") || !strings.Contains(errMsg, "err 2") {
		t.Errorf("expected combined error, got %q", errMsg)
	}
}

func TestTask_OnEmitAs_TypedFiltering(t *testing.T) {
	ctx := context.Background()

	type CustomEvent struct{ ID int }

	task := Launch(ctx, func(p *Promise[unit]) adt.Result[string] {
		p.Emit( "string message")
		p.Emit( CustomEvent{ID: 42})
		p.Emit( 12345)
		return adt.OK("done")
	})

	var customEvents []CustomEvent
	var mu sync.Mutex
	OnEmitAs(task, func(evt CustomEvent) {
		mu.Lock()
		defer mu.Unlock()
		customEvents = append(customEvents, evt)
	})

	if res := task.Await(); res.IsErr() {
		t.Fatalf("task failed: %v", res.MustErr())
	}

	mu.Lock()
	defer mu.Unlock()
	if len(customEvents) != 1 || customEvents[0].ID != 42 {
		t.Errorf("customEvents = %v, want [{ID: 42}]", customEvents)
	}
}

func TestTask_OnEmit_LateRegistrationReplay(t *testing.T) {
	ctx := context.Background()

	task := Launch(ctx, func(p *Promise[unit]) adt.Result[string] {
		p.Emit( "event 1")
		p.Emit( "event 2")
		return adt.OK("completed")
	})

	if res := task.Await(); res.IsErr() {
		t.Fatalf("task failed: %v", res.MustErr())
	}

	var replayed []string
	var mu sync.Mutex
	task.OnEmit(func(val any) {
		mu.Lock()
		defer mu.Unlock()
		if s, ok := val.(string); ok {
			replayed = append(replayed, s)
		}
	})

	mu.Lock()
	defer mu.Unlock()
	if len(replayed) != 2 || replayed[0] != "event 1" || replayed[1] != "event 2" {
		t.Errorf("replayed = %v, want ['event 1', 'event 2']", replayed)
	}
}

func TestTask_StepCancel(t *testing.T) {
	ctx := context.Background()

	task := Launch(ctx, func(p *Promise[string]) adt.Result[string] {
		res1 := p.ReceiveResult()
		if res1.IsErr() {
			res2 := p.Receive().OrElse("default")
			return adt.OK("recovered: " + res2)
		}
		return adt.OK("normal: " + res1.MustGet())
	})

	time.Sleep(10 * time.Millisecond)
	task.CancelCurrent()
	time.Sleep(10 * time.Millisecond)
	task.Send("valid payload")

	if res := task.Await(); res.MustGet() != "recovered: valid payload" {
		t.Errorf("res = %q, want 'recovered: valid payload'", res.MustGet())
	}
}

func TestTask_CancelAll(t *testing.T) {
	ctx := context.Background()

	task := Launch(ctx, func(p *Promise[int]) adt.Result[int] {
		_ = p.Receive()
		return adt.OK(100)
	})

	time.Sleep(10 * time.Millisecond)
	task.Cancel()

	res := task.Await()
	if res.IsOK() {
		t.Fatalf("expected cancellation error, got %v", res.MustGet())
	}
	if !task.IsDone() {
		t.Error("task should be marked done after cancel")
	}
}

func TestTask_AwaitCancellationRaceFix(t *testing.T) {
	// Verify that completing right before ctx cancel does not lose the result.
	for range 50 {
		ctx, cancel := context.WithCancel(context.Background())
		task := Launch(ctx, func(p *Promise[unit]) adt.Result[int] {
			return adt.OK(42)
		})
		time.Sleep(1 * time.Millisecond)
		cancel()

		res := task.Await()
		if res.IsOK() && res.MustGet() != 42 {
			t.Errorf("unexpected value %d", res.MustGet())
		}
	}
}

type unit struct{}
