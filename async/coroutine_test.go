package async

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/azuiktech/kleisli-go/adt"
)

func TestCoroutine_BasicReturn(t *testing.T) {
	task := Launch[adt.Unit, string](Config{}, func(co *Co[adt.Unit, string]) adt.Result[string] {
		return adt.OK("hello world")
	})

	res := task.Await()
	if res.IsErr() {
		t.Fatalf("expected OK, got: %v", res.MustErr())
	}
	if res.MustGet() != "hello world" {
		t.Errorf("got %q, want %q", res.MustGet(), "hello world")
	}
}

func TestCoroutine_PanicRecovery(t *testing.T) {
	for range 100 {
		task := Launch[adt.Unit, string](Config{}, func(co *Co[adt.Unit, string]) adt.Result[string] {
			panic("boom")
		})

		res := task.Await()
		if res.IsOK() {
			t.Fatalf("expected Err on panic, got OK: %v", res.MustGet())
		}
		if !strings.Contains(res.MustErr().Error(), "boom") {
			t.Errorf("expected error containing 'boom', got: %v", res.MustErr())
		}
	}
}

// Case B: Pure fire-and-forget push. Callee emits without waiting.
func TestCoroutine_CaseB_FireAndForgetEmit(t *testing.T) {
	var observed []int
	var mu sync.Mutex

	cfg := Config{
		OnEmit: func(val any) {
			mu.Lock()
			defer mu.Unlock()
			if i, ok := val.(int); ok {
				observed = append(observed, i)
			}
		},
	}

	task := Launch[adt.Unit, string](cfg, func(co *Co[adt.Unit, string]) adt.Result[string] {
		co.Emit(1)
		co.Emit(2)
		co.Emit(3)
		return adt.OK("emitted")
	})

	res := task.Await()
	if res.IsErr() || res.MustGet() != "emitted" {
		t.Fatalf("unexpected task result: %v", res)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(observed) != 3 || observed[0] != 1 || observed[1] != 2 || observed[2] != 3 {
		t.Errorf("got emissions %v, want [1, 2, 3]", observed)
	}
}

func TestCoroutine_EarlyEmission_NeverLost(t *testing.T) {
	received := make(chan int, 3)

	cfg := Config{
		OnEmit: func(val any) {
			received <- val.(int)
		},
	}

	task := Launch[adt.Unit, string](cfg, func(co *Co[adt.Unit, string]) adt.Result[string] {
		co.Emit(100)
		co.Emit(200)
		co.Emit(300)
		return adt.OK("done")
	})

	res := task.Await()
	if res.IsErr() {
		t.Fatalf("unexpected err: %v", res.MustErr())
	}

	close(received)
	var vals []int
	for v := range received {
		vals = append(vals, v)
	}
	if len(vals) != 3 || vals[0] != 100 || vals[1] != 200 || vals[2] != 300 {
		t.Errorf("got %v, want [100, 200, 300]", vals)
	}
}

// Case A: Acknowledged emission / handshake via Call[adt.Unit].
func TestCoroutine_CaseA_AcknowledgedEmission(t *testing.T) {
	var order []string
	var mu sync.Mutex

	cfg := Config{
		OnCall: func(req any, p *Promise[any]) {
			mu.Lock()
			order = append(order, fmt.Sprintf("caller:processing-%v", req))
			mu.Unlock()
			time.Sleep(10 * time.Millisecond)
			p.Resolve(adt.Unit{})
		},
	}

	task := Launch[adt.Unit, string](cfg, func(co *Co[adt.Unit, string]) adt.Result[string] {
		mu.Lock()
		order = append(order, "callee:before-emit")
		mu.Unlock()

		// Acknowledged emission: waits for caller to complete processing
		co.Call[adt.Unit]("critical-event").Await()

		mu.Lock()
		order = append(order, "callee:after-ack")
		mu.Unlock()

		return adt.OK("done")
	})

	res := task.Await()
	if res.IsErr() {
		t.Fatalf("expected OK, got: %v", res.MustErr())
	}

	mu.Lock()
	defer mu.Unlock()
	expected := []string{"callee:before-emit", "caller:processing-critical-event", "callee:after-ack"}
	if len(order) != len(expected) {
		t.Fatalf("got order %v, want %v", order, expected)
	}
	for i := range expected {
		if order[i] != expected[i] {
			t.Errorf("order[%d] = %q, want %q", i, order[i], expected[i])
		}
	}
}

// Case C & D: Internal async tasks (coupled and decoupled).
func TestCoroutine_CaseCD_AsyncInternal(t *testing.T) {
	task := Launch[adt.Unit, int](Config{}, func(co *Co[adt.Unit, int]) adt.Result[int] {
		// Case C: Coupled Call + Await
		val1 := co.Async(func(ctx context.Context) adt.Result[int] {
			return adt.OK(10)
		}).Await().MustGet()

		// Case D: Decoupled Call ... Await
		tok1 := co.Async(func(ctx context.Context) adt.Result[int] {
			time.Sleep(10 * time.Millisecond)
			return adt.OK(20)
		})
		tok2 := co.Async(func(ctx context.Context) adt.Result[int] {
			time.Sleep(10 * time.Millisecond)
			return adt.OK(30)
		})

		val2 := tok1.Await().MustGet()
		val3 := tok2.Await().MustGet()

		return adt.OK(val1 + val2 + val3)
	})

	res := task.Await()
	if res.IsErr() {
		t.Fatalf("expected OK, got: %v", res.MustErr())
	}
	if res.MustGet() != 60 {
		t.Errorf("got %d, want 60", res.MustGet())
	}
}

// Case E & F: External bidirectional requests (coupled and decoupled).
func TestCoroutine_CaseEF_CallExternal(t *testing.T) {
	type WeatherReq struct{ City string }
	type WeatherReport struct{ Temp int }

	cfg := Config{
		OnCall: func(req any, p *Promise[any]) {
			if r, ok := req.(WeatherReq); ok {
				switch r.City {
				case "NYC":
					p.Resolve(WeatherReport{Temp: 72})
				case "LON":
					p.Resolve(WeatherReport{Temp: 55})
				}
				return
			}
			p.Reject(errors.New("unknown request"))
		},
	}

	task := Launch[adt.Unit, string](cfg, func(co *Co[adt.Unit, string]) adt.Result[string] {
		// Case E: Coupled Call + Await
		r1 := co.Call[WeatherReport](WeatherReq{City: "NYC"}).Await().MustGet()

		// Case F: Decoupled Call ... Await
		tok := co.Call[WeatherReport](WeatherReq{City: "LON"})
		r2 := tok.Await().MustGet()

		return adt.OK(fmt.Sprintf("NYC: %d, LON: %d", r1.Temp, r2.Temp))
	})

	res := task.Await()
	if res.IsErr() {
		t.Fatalf("expected OK, got: %v", res.MustErr())
	}
	if res.MustGet() != "NYC: 72, LON: 55" {
		t.Errorf("got %q, want %q", res.MustGet(), "NYC: 72, LON: 55")
	}
}

func TestCoroutine_Call_TypeMismatch(t *testing.T) {
	cfg := Config{
		OnCall: func(req any, p *Promise[any]) {
			// Mistakenly resolve with string instead of int
			p.Resolve("not-an-int")
		},
	}

	task := Launch[adt.Unit, string](cfg, func(co *Co[adt.Unit, string]) adt.Result[string] {
		res := co.Call[int]("give-me-int").Await()
		if res.IsOK() {
			return adt.OK("unexpected-ok")
		}
		return adt.OK("type-mismatch-caught: " + res.MustErr().Error())
	})

	res := task.Await()
	if res.IsErr() {
		t.Fatalf("expected OK, got: %v", res.MustErr())
	}
	if !strings.Contains(res.MustGet(), "type mismatch") {
		t.Errorf("expected type mismatch error, got: %s", res.MustGet())
	}
}

func TestCoroutine_Call_NoHandler(t *testing.T) {
	task := Launch[adt.Unit, string](Config{}, func(co *Co[adt.Unit, string]) adt.Result[string] {
		res := co.Call[int]("req").Await()
		if res.IsErr() {
			return adt.OK("handled-no-listener")
		}
		return adt.Err[string](errors.New("should have failed"))
	})

	// No OnCall registered
	res := task.Await()
	if res.IsErr() || res.MustGet() != "handled-no-listener" {
		t.Errorf("expected handled-no-listener, got: %v", res)
	}
}

func TestCoroutine_Async_ChildContextCancelledOnExit(t *testing.T) {
	childCancelled := make(chan struct{})

	task := Launch[adt.Unit, string](Config{}, func(co *Co[adt.Unit, string]) adt.Result[string] {
		// Spawn child task that sleeps until cancelled
		co.Async(func(ctx context.Context) adt.Result[adt.Unit] {
			<-ctx.Done()
			close(childCancelled)
			return adt.OK(adt.Unit{})
		})
		// Callee exits immediately without awaiting the child task
		return adt.OK("parent-exited")
	})

	res := task.Await()
	if res.IsErr() || res.MustGet() != "parent-exited" {
		t.Fatalf("unexpected task result: %v", res)
	}

	select {
	case <-childCancelled:
		// Succeeded: child was cancelled when parent finished
	case <-time.After(500 * time.Millisecond):
		t.Fatal("child task was not cancelled when coroutine exited")
	}
}

func TestCoroutine_TaskCancel(t *testing.T) {
	coroutineExited := make(chan struct{})
	customErr := errors.New("aborted by caller")

	task := Launch[adt.Unit, string](Config{}, func(co *Co[adt.Unit, string]) adt.Result[string] {
		<-co.Context().Done()
		close(coroutineExited)
		return adt.Err[string](context.Cause(co.Context()))
	})

	if !task.Cancel(customErr) {
		t.Errorf("expected Cancel to return true")
	}

	select {
	case <-coroutineExited:
		// Succeeded: coroutine context was cancelled and goroutine exited promptly
	case <-time.After(500 * time.Millisecond):
		t.Fatal("coroutine goroutine leaked: co.Context() was not cancelled by task.Cancel()")
	}

	res := task.Await()
	if res.IsOK() {
		t.Fatalf("expected Err, got OK: %v", res.MustGet())
	}
	if !errors.Is(res.MustErr(), customErr) {
		t.Errorf("got %v, want %v", res.MustErr(), customErr)
	}
}

func TestCoroutine_TaskCancel_ConcurrentAwaitAndCancel(t *testing.T) {
	const iters = 100
	for range iters {
		task := Launch[adt.Unit, string](Config{}, func(co *Co[adt.Unit, string]) adt.Result[string] {
			<-co.Context().Done()
			return adt.Err[string](co.Context().Err())
		})

		var wg sync.WaitGroup
		wg.Add(2)
		var cancelResult bool

		go func() {
			defer wg.Done()
			task.Await()
		}()

		go func() {
			defer wg.Done()
			cancelResult = task.Cancel(errors.New("cancel race"))
		}()

		wg.Wait()
		if !cancelResult {
			t.Fatalf("task.Cancel returned false during concurrent Await")
		}
	}
}
