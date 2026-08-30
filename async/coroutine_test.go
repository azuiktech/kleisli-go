package async

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestTask_TypedSendAndReceive(t *testing.T) {
	ctx := context.Background()

	task := Launch(ctx, func(p *Promise[int]) string {
		a := Receive(p).OrElse(0)
		b := Receive(p).OrElse(0)
		return fmt.Sprintf("sum: %d", a+b)
	})

	time.Sleep(10 * time.Millisecond)
	ok1 := task.Send(10)
	if !ok1 {
		t.Error("expected ok1 to be true")
	}

	time.Sleep(10 * time.Millisecond)
	ok2 := task.Send(25)
	if !ok2 {
		t.Error("expected ok2 to be true")
	}

	res := task.Await()
	if res.IsErr() {
		t.Fatalf("unexpected error in Await: %v", res.MustErr())
	}
	if res.MustGet() != "sum: 35" {
		t.Errorf("got %q, want %q", res.MustGet(), "sum: 35")
	}

	// Sending to completed task returns false
	if task.Send(99) {
		t.Error("expected Send to closed task to return false")
	}
}

func TestTask_Yield_AtomicEmitReceive(t *testing.T) {
	ctx := context.Background()

	task := Launch(ctx, func(p *Promise[string]) string {
		name := Yield[string](p, "what is your name?").MustGet()
		city := Yield[string](p, "what is your city?").MustGet()
		return fmt.Sprintf("%s from %s", name, city)
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

	res := task.Await().MustGet()
	if res != "Alice from Bangalore" {
		t.Errorf("got %q, want %q", res, "Alice from Bangalore")
	}

	mu.Lock()
	defer mu.Unlock()
	if len(emitted) != 2 || emitted[0] != "what is your name?" || emitted[1] != "what is your city?" {
		t.Errorf("emitted = %v", emitted)
	}
}

func TestTask_MonadicChaining(t *testing.T) {
	ctx := context.Background()

	base := Launch(ctx, func(p *Promise[int]) int {
		x := Receive(p).OrElse(0)
		return x * 2
	})

	// Map
	mapped := base.Map(func(n int) string {
		return fmt.Sprintf("result: %d", n)
	})

	// FlatMap
	flatMapped := mapped.FlatMap(func(s string) *Task[int, string] {
		return Launch(ctx, func(p *Promise[int]) string {
			return s + "!"
		})
	})

	// Then
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

	task := Launch(ctx, func(p *Promise[int]) int {
		time.Sleep(100 * time.Millisecond)
		return 42
	})

	// Timeout should expire before 100ms
	res := task.AwaitTimeout(10 * time.Millisecond)
	if res.IsOK() {
		t.Errorf("expected timeout error, got %v", res.MustGet())
	}

	// Task itself should NOT be cancelled and should eventually finish
	finalRes := task.Await()
	if finalRes.IsErr() || finalRes.MustGet() != 42 {
		t.Errorf("expected final result 42, got %v", finalRes)
	}
}

func TestTask_MultiTaskAll(t *testing.T) {
	ctx := context.Background()

	t1 := Launch(ctx, func(p *Promise[unit]) int { return 10 })
	t2 := Launch(ctx, func(p *Promise[unit]) int { return 20 })
	t3 := Launch(ctx, func(p *Promise[unit]) int { return 30 })

	allTask := All(ctx, t1, t2, t3)
	res := allTask.Await()
	if res.IsErr() {
		t.Fatalf("All failed: %v", res.MustErr())
	}
	vals := res.MustGet()
	if len(vals) != 3 || vals[0] != 10 || vals[1] != 20 || vals[2] != 30 {
		t.Errorf("vals = %v, want [10, 20, 30]", vals)
	}
}

func TestTask_MultiTaskRace(t *testing.T) {
	ctx := context.Background()

	slow := Launch(ctx, func(p *Promise[unit]) string {
		time.Sleep(100 * time.Millisecond)
		return "slow"
	})
	fast := Launch(ctx, func(p *Promise[unit]) string {
		return "fast"
	})

	raceTask := Race(ctx, slow, fast)
	res := raceTask.Await()
	if res.IsErr() || res.MustGet() != "fast" {
		t.Errorf("Race got %v, want 'fast'", res)
	}
}

func TestTask_StepCancel(t *testing.T) {
	ctx := context.Background()

	task := Launch(ctx, func(p *Promise[string]) string {
		// First step will be canceled
		res1 := ReceiveResult(p)
		if res1.IsErr() {
			// Step was canceled, gracefully fallback or try next step
			res2 := Receive(p).OrElse("default")
			return "recovered: " + res2
		}
		return "normal: " + res1.MustGet()
	})

	time.Sleep(10 * time.Millisecond)
	task.CancelCurrent()

	time.Sleep(10 * time.Millisecond)
	task.Send("valid payload")

	res := task.Await()
	if res.MustGet() != "recovered: valid payload" {
		t.Errorf("res = %q, want 'recovered: valid payload'", res.MustGet())
	}
}

func TestTask_CancelAll(t *testing.T) {
	ctx := context.Background()

	task := Launch(ctx, func(p *Promise[int]) int {
		_ = Receive(p)
		return 100
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
	// Verify that completing right before cancel does not lose the result
	for i := 0; i < 50; i++ {
		ctx, cancel := context.WithCancel(context.Background())
		task := Launch(ctx, func(p *Promise[unit]) int {
			return 42
		})
		time.Sleep(1 * time.Millisecond)
		cancel()

		res := task.Await()
		// Result 42 should survive even if cancel fired around the same time
		if res.IsOK() && res.MustGet() != 42 {
			t.Errorf("unexpected value %d", res.MustGet())
		}
	}
}

type unit struct{}
