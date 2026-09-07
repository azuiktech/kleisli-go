package async

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/azuiktech/kleisli-go/adt"
)

func TestPackagedTask_DeferredRun_Sync(t *testing.T) {
	var executed atomic.Bool
	task := PackagedTask[int, int](Config{}, func(co *Co, in int) adt.Result[int] {
		executed.Store(true)
		return adt.OK(in * 2)
	})

	// Future should not be done prior to Run
	select {
	case <-task.Future().Context().Done():
		t.Fatal("future should not be done before Run")
	default:
	}

	if executed.Load() {
		t.Fatal("fn should not execute before Run")
	}

	// Run synchronously on calling goroutine
	res := task.Run(21)
	if res.IsErr() {
		t.Fatalf("expected OK, got: %v", res.MustErr())
	}
	if res.MustGet() != 42 {
		t.Errorf("got %d, want 42", res.MustGet())
	}
	if !executed.Load() {
		t.Fatal("expected fn to have executed")
	}

	// Subsequent Await() returns identical result
	if task.Await().MustGet() != 42 {
		t.Errorf("expected Await to return 42")
	}
}

func TestPackagedTask_DeferredRun_WorkerPool(t *testing.T) {
	type WorkItem struct {
		task  *Task[int, int]
		input int
	}

	workCh := make(chan WorkItem, 10)
	var wg sync.WaitGroup

	// Start 3 workers
	for range 3 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for item := range workCh {
				item.task.Run(item.input)
			}
		}()
	}

	var tasks []*Task[int, int]
	for i := 1; i <= 10; i++ {
		task := PackagedTask[int, int](Config{}, func(co *Co, in int) adt.Result[int] {
			time.Sleep(5 * time.Millisecond)
			return adt.OK(in * 10)
		})
		tasks = append(tasks, task)
		workCh <- WorkItem{task: task, input: i}
	}
	close(workCh)
	wg.Wait()

	for i, task := range tasks {
		expected := (i + 1) * 10
		res := task.Await()
		if res.IsErr() || res.MustGet() != expected {
			t.Errorf("task %d: got %v, want %d", i, res, expected)
		}
	}
}

func TestPackagedTask_CancelBeforeRun(t *testing.T) {
	var observedErr error
	customErr := errors.New("cancelled before start")

	task := PackagedTask[int, int](Config{}, func(co *Co, in int) adt.Result[int] {
		observedErr = co.Err()
		return adt.OK(in * 100)
	})

	if !task.Cancel(customErr) {
		t.Fatal("expected Cancel to return true")
	}

	res := task.Run(5)
	if res.IsOK() {
		t.Fatalf("expected Err, got: %v", res.MustGet())
	}
	if !errors.Is(res.MustErr(), customErr) {
		t.Errorf("got %v, want %v", res.MustErr(), customErr)
	}
	if !errors.Is(observedErr, context.Canceled) {
		t.Errorf("expected co.Err() to be context.Canceled inside fn, got: %v", observedErr)
	}
}

func TestPackagedTask_SingleSettlement(t *testing.T) {
	task := PackagedTask[int, int](Config{}, func(co *Co, in int) adt.Result[int] {
		return adt.OK(in + 1)
	})

	res1 := task.Run(10)
	res2 := task.Run(20) // second call cannot overwrite settled Promise
	res3 := task.Await()

	if res1.MustGet() != 11 || res2.MustGet() != 11 || res3.MustGet() != 11 {
		t.Errorf("all runs must return 11: r1=%v, r2=%v, r3=%v", res1, res2, res3)
	}
}

func TestCoroutine_CoEmbedsContext(t *testing.T) {
	helperNeedingContext := func(ctx context.Context) adt.Result[string] {
		deadline, ok := ctx.Deadline()
		_ = deadline
		_ = ok
		return adt.OK("context-accepted")
	}

	task := Launch[adt.Unit, string](Config{}, adt.Void, func(co *Co, _ adt.Unit) adt.Result[string] {
		// Pass co directly as context.Context
		return helperNeedingContext(co)
	})

	res := task.Await()
	if res.IsErr() || res.MustGet() != "context-accepted" {
		t.Fatalf("expected context-accepted, got: %v", res)
	}
}

func TestCoroutine_BasicReturn(t *testing.T) {
	task := Launch[adt.Unit, string](Config{}, adt.Void, func(co *Co, _ adt.Unit) adt.Result[string] {
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

func TestCoroutine_InputPassing(t *testing.T) {
	task := Launch[string, string](Config{}, "Gopher", func(co *Co, in string) adt.Result[string] {
		return adt.OK("Hello, " + in)
	})

	res := task.Await()
	if res.IsErr() || res.MustGet() != "Hello, Gopher" {
		t.Errorf("got %v, want Hello, Gopher", res)
	}
}

func TestCoroutine_PanicRecovery(t *testing.T) {
	for range 100 {
		task := Launch[adt.Unit, string](Config{}, adt.Void, func(co *Co, _ adt.Unit) adt.Result[string] {
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

	task := Launch[adt.Unit, string](cfg, adt.Void, func(co *Co, _ adt.Unit) adt.Result[string] {
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

	task := Launch[adt.Unit, string](cfg, adt.Void, func(co *Co, _ adt.Unit) adt.Result[string] {
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
	eventOp := DefineOp[string, adt.Unit]("critical-event")
	var mu sync.Mutex

	cfg := Config{
		OnCall: []CallHandler{
			eventOp.Handle(func(req string) adt.Result[adt.Unit] {
				mu.Lock()
				order = append(order, fmt.Sprintf("caller:processing-%s", req))
				mu.Unlock()
				time.Sleep(10 * time.Millisecond)
				return adt.OK(adt.Void)
			}),
		},
	}

	task := Launch[adt.Unit, string](cfg, adt.Void, func(co *Co, _ adt.Unit) adt.Result[string] {
		mu.Lock()
		order = append(order, "callee:before-emit")
		mu.Unlock()

		// Acknowledged emission: waits for caller to complete processing
		co.Call(eventOp, "critical-event").Await()

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
	task := Launch[adt.Unit, int](Config{}, adt.Void, func(co *Co, _ adt.Unit) adt.Result[int] {
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
	type WeatherReport struct{ Temp int }
	type WeatherReq struct{ City string }

	weatherOp := DefineOp[WeatherReq, WeatherReport]("weather")

	cfg := Config{
		OnCall: []CallHandler{
			weatherOp.Handle(func(r WeatherReq) adt.Result[WeatherReport] {
				switch r.City {
				case "NYC":
					return adt.OK(WeatherReport{Temp: 72})
				case "LON":
					return adt.OK(WeatherReport{Temp: 55})
				default:
					return adt.Err[WeatherReport](errors.New("unknown request"))
				}
			}),
		},
	}

	task := Launch[adt.Unit, string](cfg, adt.Void, func(co *Co, _ adt.Unit) adt.Result[string] {
		// Case E: Coupled Call + Await (type inferred!)
		r1 := co.Call(weatherOp, WeatherReq{City: "NYC"}).Await().MustGet()

		// Case F: Decoupled Call ... Await (type inferred!)
		tok := co.Call(weatherOp, WeatherReq{City: "LON"})
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

func TestCoroutine_Call_DecoupledNonBlocking(t *testing.T) {
	req1Started := make(chan struct{})
	allowReq1ToFinish := make(chan struct{})
	lineBReached := make(chan struct{})

	strOp := DefineOp[string, string]("str_req")

	cfg := Config{
		OnCall: []CallHandler{
			strOp.Handle(func(req string) adt.Result[string] {
				switch req {
				case "req1":
					close(req1Started)
					<-allowReq1ToFinish
					return adt.OK("resp1")
				case "req2":
					return adt.OK("resp2")
				default:
					return adt.Err[string](errors.New("unknown"))
				}
			}),
		},
	}

	task := Launch[adt.Unit, string](cfg, adt.Void, func(co *Co, _ adt.Unit) adt.Result[string] {
		// Line A: Call req1 which blocks in handler
		tok1 := co.Call(strOp, "req1")

		// Ensure handler for req1 has started and is currently blocked
		<-req1Started

		// Line B: Must be reached immediately without waiting for req1 to finish!
		tok2 := co.Call(strOp, "req2")
		close(lineBReached)

		// Unblock req1 now
		close(allowReq1ToFinish)

		r1 := tok1.Await().MustGet()
		r2 := tok2.Await().MustGet()

		return adt.OK(r1 + "+" + r2)
	})

	select {
	case <-lineBReached:
		// Succeeded: Line B was reached while req1 was still blocked in OnCall!
	case <-time.After(1 * time.Second):
		t.Fatal("co.Call blocked callee: Line B was not reached while req1 was pending")
	}

	res := task.Await()
	if res.IsErr() || res.MustGet() != "resp1+resp2" {
		t.Fatalf("expected resp1+resp2, got: %v", res)
	}
}

type mismatchHandler struct{}

func (m mismatchHandler) OpName() string { return "give_int" }
func (m mismatchHandler) Handle(inv CallInvocation, p *Promise[any]) bool {
	if inv.OpName == "give_int" {
		p.Resolve("not-an-int")
		return true
	}
	return false
}

func TestCoroutine_Call_TypeMismatch(t *testing.T) {
	intOp := DefineOp[string, int]("give_int")
	cfg := Config{
		OnCall: []CallHandler{mismatchHandler{}},
	}

	task := Launch[adt.Unit, string](cfg, adt.Void, func(co *Co, _ adt.Unit) adt.Result[string] {
		res := co.Call(intOp, "give-me-int").Await()
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
	intOp := DefineOp[string, int]("req_op")
	task := Launch[adt.Unit, string](Config{}, adt.Void, func(co *Co, _ adt.Unit) adt.Result[string] {
		res := co.Call(intOp, "req").Await()
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

	task := Launch[adt.Unit, string](Config{}, adt.Void, func(co *Co, _ adt.Unit) adt.Result[string] {
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

	task := Launch[adt.Unit, string](Config{}, adt.Void, func(co *Co, _ adt.Unit) adt.Result[string] {
		<-co.Done()
		close(coroutineExited)
		return adt.Err[string](context.Cause(co))
	})

	if !task.Cancel(customErr) {
		t.Errorf("expected Cancel to return true")
	}

	select {
	case <-coroutineExited:
		// Succeeded: coroutine context was cancelled and goroutine exited promptly
	case <-time.After(500 * time.Millisecond):
		t.Fatal("coroutine goroutine leaked: co was not cancelled by task.Cancel()")
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
		task := Launch[adt.Unit, string](Config{}, adt.Void, func(co *Co, _ adt.Unit) adt.Result[string] {
			<-co.Done()
			return adt.Err[string](co.Err())
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

func TestCoroutine_Handle_MultipleTypedHandlers(t *testing.T) {
	opA := DefineOp[int, int]("double")
	opB := DefineOp[string, string]("upper")

	cfg := Config{
		OnCall: []CallHandler{
			opA.Handle(func(val int) adt.Result[int] {
				return adt.OK(val * 2)
			}),
			opB.Handle(func(text string) adt.Result[string] {
				return adt.OK(strings.ToUpper(text))
			}),
		},
	}

	task := Launch[adt.Unit, string](cfg, adt.Void, func(co *Co, _ adt.Unit) adt.Result[string] {
		futA := co.Call(opA, 21)
		futB := co.Call(opB, "hello")

		ansA := futA.Await().MustGet()
		ansB := futB.Await().MustGet()

		return adt.OK(fmt.Sprintf("%d:%s", ansA, ansB))
	})

	res := task.Await()
	if res.IsErr() || res.MustGet() != "42:HELLO" {
		t.Fatalf("expected 42:HELLO, got %v", res)
	}
}

func TestCoroutine_AlternatingAsyncCall_Goroutine(t *testing.T) {
	op1 := DefineOp[string, string]("op1")
	op2 := DefineOp[int, int]("op2")

	var (
		mu         sync.Mutex
		trace      []string
		async1Runs atomic.Int32
		call1Runs  atomic.Int32
		async2Runs atomic.Int32
		call2Runs  atomic.Int32
	)

	logTrace := func(s string) {
		mu.Lock()
		defer mu.Unlock()
		trace = append(trace, s)
	}

	cfg := Config{
		OnCall: []CallHandler{
			op1.Handle(func(s string) adt.Result[string] {
				call1Runs.Add(1)
				logTrace("call1_exec:" + s)
				return adt.OK("res_" + s)
			}),
			op2.Handle(func(n int) adt.Result[int] {
				call2Runs.Add(1)
				logTrace(fmt.Sprintf("call2_exec:%d", n))
				return adt.OK(n * 2)
			}),
		},
	}

	workflow := func(co *Co, in string) adt.Result[string] {
		// 1. fInitial
		logTrace("fInitial:" + in)

		// 2. Alternate: Async, Call, Async, Call
		a1 := co.Async(func(ctx context.Context) adt.Result[string] {
			async1Runs.Add(1)
			logTrace("async1_exec")
			return adt.OK("async1_done")
		})

		c1 := co.Call(op1, "c1_arg")

		a2 := co.Async(func(ctx context.Context) adt.Result[string] {
			async2Runs.Add(1)
			logTrace("async2_exec")
			return adt.OK("async2_done")
		})

		c2 := co.Call(op2, 21)

		// 3. Await all 4
		rA1 := a1.Await().MustGet()
		rC1 := c1.Await().MustGet()
		rA2 := a2.Await().MustGet()
		rC2 := c2.Await().MustGet()

		// 4. fFinal
		logTrace("fFinal")

		return adt.OK(fmt.Sprintf("%s|%s|%s|%d", rA1, rC1, rA2, rC2))
	}

	task := Launch[string, string](cfg, "start", workflow)
	res := task.Await()

	if res.IsErr() {
		t.Fatalf("unexpected error: %v", res.MustErr())
	}
	expectedOutput := "async1_done|res_c1_arg|async2_done|42"
	if res.MustGet() != expectedOutput {
		t.Fatalf("expected output %q, got %q", expectedOutput, res.MustGet())
	}

	if async1Runs.Load() != 1 || call1Runs.Load() != 1 || async2Runs.Load() != 1 || call2Runs.Load() != 1 {
		t.Fatalf("expected exactly 1 execution each, got a1=%d, c1=%d, a2=%d, c2=%d",
			async1Runs.Load(), call1Runs.Load(), async2Runs.Load(), call2Runs.Load())
	}

	mu.Lock()
	defer mu.Unlock()
	if len(trace) < 2 || trace[0] != "fInitial:start" || trace[len(trace)-1] != "fFinal" {
		t.Fatalf("trace order violation: %v", trace)
	}
}

func TestCoroutine_AlternatingAsyncCall_Durable(t *testing.T) {
	op1 := DefineOp[string, string]("op1")
	op2 := DefineOp[int, int]("op2")

	var (
		mu          sync.Mutex
		trace       []string
		async1Runs  atomic.Int32
		call1Runs   atomic.Int32
		async2Runs  atomic.Int32
		call2Runs   atomic.Int32
		initialRuns atomic.Int32
		finalRuns   atomic.Int32
	)

	logTrace := func(s string) {
		mu.Lock()
		defer mu.Unlock()
		trace = append(trace, s)
	}

	handlers := []CallHandler{
		op1.Handle(func(s string) adt.Result[string] {
			call1Runs.Add(1)
			logTrace("call1_exec:" + s)
			return adt.OK("res_" + s)
		}),
		op2.Handle(func(n int) adt.Result[int] {
			call2Runs.Add(1)
			logTrace(fmt.Sprintf("call2_exec:%d", n))
			return adt.OK(n * 2)
		}),
	}

	workflow := func(co *Co, in string) adt.Result[string] {
		// 1. fInitial
		initialRuns.Add(1)
		logTrace("fInitial:" + in)

		// 2. Alternate: Async, Call, Async, Call
		a1 := co.Async(func(ctx context.Context) adt.Result[string] {
			async1Runs.Add(1)
			logTrace("async1_exec")
			return adt.OK("async1_done")
		})

		c1 := co.Call(op1, "c1_arg")

		a2 := co.Async(func(ctx context.Context) adt.Result[string] {
			async2Runs.Add(1)
			logTrace("async2_exec")
			return adt.OK("async2_done")
		})

		c2 := co.Call(op2, 21)

		// 3. Await all 4, cleanly propagating any suspension error
		resA1 := a1.Await()
		if resA1.IsErr() {
			return adt.Err[string](resA1.MustErr())
		}
		rA1 := resA1.MustGet()

		resC1 := c1.Await()
		if resC1.IsErr() {
			return adt.Err[string](resC1.MustErr())
		}
		rC1 := resC1.MustGet()

		resA2 := a2.Await()
		if resA2.IsErr() {
			return adt.Err[string](resA2.MustErr())
		}
		rA2 := resA2.MustGet()

		resC2 := c2.Await()
		if resC2.IsErr() {
			return adt.Err[string](resC2.MustErr())
		}
		rC2 := resC2.MustGet()

		// 4. fFinal
		finalRuns.Add(1)
		logTrace("fFinal")

		return adt.OK(fmt.Sprintf("%s|%s|%s|%d", rA1, rC1, rA2, rC2))
	}

	journal := NewJournal()

	// Simulate restarts before each of the points (turns 0, 1, 2, 3) and final run (turn 4)
	for turn := 0; turn <= 4; turn++ {
		durableCtx := NewDurableContext(context.Background(), journal)
		durableCtx.SetMaxSteps(turn) // On turn i, allows execution up to i steps before suspending

		cfg := Config{
			Context: durableCtx,
			OnCall:  handlers,
		}

		task := Launch[string, string](cfg, "start", workflow)
		res := task.Await()

		if turn < 4 {
			// Expected to suspend with ErrSuspended
			if !res.IsErr() || !errors.Is(res.MustErr(), ErrSuspended) {
				t.Fatalf("turn %d: expected ErrSuspended, got %v", turn, res)
			}
			if journal.Len() != turn {
				t.Fatalf("turn %d: expected journal len %d, got %d", turn, turn, journal.Len())
			}
		} else {
			// Final turn: must succeed and produce the complete output
			if res.IsErr() {
				t.Fatalf("turn 4: unexpected error: %v", res.MustErr())
			}
			expectedOutput := "async1_done|res_c1_arg|async2_done|42"
			if res.MustGet() != expectedOutput {
				t.Fatalf("turn 4: expected output %q, got %q", expectedOutput, res.MustGet())
			}
		}
	}

	// VERIFY EXECUTION COUNTS:
	// Even though 5 turns occurred, each async / call MUST execute EXACTLY ONCE!
	if async1Runs.Load() != 1 {
		t.Fatalf("expected async1 to execute exactly once, got %d", async1Runs.Load())
	}
	if call1Runs.Load() != 1 {
		t.Fatalf("expected call1 to execute exactly once, got %d", call1Runs.Load())
	}
	if async2Runs.Load() != 1 {
		t.Fatalf("expected async2 to execute exactly once, got %d", async2Runs.Load())
	}
	if call2Runs.Load() != 1 {
		t.Fatalf("expected call2 to execute exactly once, got %d", call2Runs.Load())
	}

	// fInitial ran on every restart (5 times)
	if initialRuns.Load() != 5 {
		t.Fatalf("expected fInitial to run 5 times (on every restart), got %d", initialRuns.Load())
	}

	// fFinal ran ONLY on the final successful completion (1 time)
	if finalRuns.Load() != 1 {
		t.Fatalf("expected fFinal to run exactly 1 time, got %d", finalRuns.Load())
	}
}

func TestTask_Cancel_PropagatesTo_Co_GoroutineContext(t *testing.T) {
	started := make(chan struct{})
	unblocked := make(chan struct{})

	task := Launch[adt.Unit, string](Config{}, adt.Void, func(co *Co, _ adt.Unit) adt.Result[string] {
		close(started)
		select {
		case <-co.Done():
			close(unblocked)
			return adt.Err[string](co.Err())
		case <-time.After(2 * time.Second):
			return adt.Err[string](errors.New("timed out waiting for co.Done()"))
		}
	})

	<-started
	cancelErr := errors.New("aborted by caller")
	if !task.Cancel(cancelErr) {
		t.Fatal("expected task.Cancel to return true")
	}

	select {
	case <-unblocked:
	case <-time.After(1 * time.Second):
		t.Fatal("co.Done() was not unblocked after task.Cancel")
	}

	res := task.Await()
	if !res.IsErr() {
		t.Fatal("expected task result to be error")
	}
	if !errors.Is(res.MustErr(), cancelErr) {
		t.Fatalf("expected error %v, got %v", cancelErr, res.MustErr())
	}
}

func TestTask_Cancel_PropagatesTo_Co_DurableContext(t *testing.T) {
	started := make(chan struct{})
	unblocked := make(chan struct{})

	journal := NewJournal()
	durableCtx := NewDurableContext(context.Background(), journal)

	task := Launch[adt.Unit, string](Config{Context: durableCtx}, adt.Void, func(co *Co, _ adt.Unit) adt.Result[string] {
		close(started)
		select {
		case <-co.Done():
			close(unblocked)
			return adt.Err[string](co.Err())
		case <-time.After(2 * time.Second):
			return adt.Err[string](errors.New("timed out waiting for co.Done()"))
		}
	})

	<-started
	cancelErr := errors.New("aborted durable workflow")
	if !task.Cancel(cancelErr) {
		t.Fatal("expected task.Cancel to return true")
	}

	select {
	case <-unblocked:
	case <-time.After(1 * time.Second):
		t.Fatal("co.Done() was not unblocked after task.Cancel with DurableContext")
	}

	res := task.Await()
	if !res.IsErr() {
		t.Fatal("expected task result to be error")
	}
	if !errors.Is(res.MustErr(), cancelErr) {
		t.Fatalf("expected error %v, got %v", cancelErr, res.MustErr())
	}
}

type customContextWithHandlers struct {
	context.Context
	handlers []CallHandler
}

func (c *customContextWithHandlers) SetHandlers(handlers []CallHandler) {
	c.handlers = append(c.handlers, handlers...)
}

func (c *customContextWithHandlers) Call(op string, s any) *Future[any] {
	prom, fut := NewPromise[any](c.Context)
	go func() {
		defer settlePanic(prom)
		inv := CallInvocation{OpName: op, S: s}
		if !dispatchCall(c.handlers, inv, prom) {
			prom.Reject(fmt.Errorf("no handler registered for operation: %s", op))
		}
	}()
	return fut
}

func (c *customContextWithHandlers) Async(fn func(ctx context.Context) adt.Result[any]) *Future[any] {
	prom, fut := NewPromise[any](c.Context)
	go func() {
		defer settlePanic(prom)
		fn(fut.Context()).Fold(prom.Resolve, prom.Reject)
	}()
	return fut
}

func TestHandlerContext_CustomContextReceivesHandlers(t *testing.T) {
	myOp := DefineOp[string, int]("my_op")
	customCtx := &customContextWithHandlers{Context: context.Background()}

	cfg := Config{
		Context: customCtx,
		OnCall: []CallHandler{
			myOp.Handle(func(s string) adt.Result[int] {
				return adt.OK(len(s))
			}),
		},
	}

	task := Launch[adt.Unit, int](cfg, adt.Void, func(co *Co, _ adt.Unit) adt.Result[int] {
		return co.Call(myOp, "hello").Await()
	})

	res := task.Await()
	if res.IsErr() {
		t.Fatalf("unexpected error: %v", res.MustErr())
	}
	if res.MustGet() != 5 {
		t.Fatalf("expected 5, got %d", res.MustGet())
	}
}

func TestHandlerContext_PreloadedAndConfigHandlers(t *testing.T) {
	op1 := DefineOp[int, int]("op1")
	op2 := DefineOp[int, int]("op2")

	gc := NewGoroutineContext(context.Background())
	gc.SetHandlers([]CallHandler{
		op1.Handle(func(x int) adt.Result[int] { return adt.OK(x * 2) }),
	})

	cfg := Config{
		Context: gc,
		OnCall: []CallHandler{
			op2.Handle(func(x int) adt.Result[int] { return adt.OK(x + 10) }),
		},
	}

	task := Launch[adt.Unit, int](cfg, adt.Void, func(co *Co, _ adt.Unit) adt.Result[int] {
		r1 := co.Call(op1, 5).Await().MustGet()
		r2 := co.Call(op2, 5).Await().MustGet()
		return adt.OK(r1 + r2)
	})

	res := task.Await()
	if res.IsErr() {
		t.Fatalf("unexpected error: %v", res.MustErr())
	}
	if res.MustGet() != 25 { // 10 + 15
		t.Fatalf("expected 25, got %d", res.MustGet())
	}
}

func TestOp_DefineOp_EmptyName(t *testing.T) {
	op := DefineOp[int, string]("")
	expected := "int->string"
	if op.Name() != expected {
		t.Fatalf("expected op name %q, got %q", expected, op.Name())
	}
}

func TestOp_DefineOp_NonEmpty(t *testing.T) {
	op := DefineOp[int, string]("custom_name")
	expected := "custom_name"
	if op.Name() != expected {
		t.Fatalf("expected op name %q, got %q", expected, op.Name())
	}
}

func TestCallHandler_RequestTypeMismatch(t *testing.T) {
	op := DefineOp[int, int]("math_op")
	handler := op.Handle(func(x int) adt.Result[int] {
		return adt.OK(x * 2)
	})

	prom, fut := NewPromise[any](context.Background())
	inv := CallInvocation{OpName: "math_op", S: "not_an_int"}

	handled := handler.Handle(inv, prom)
	if !handled {
		t.Fatal("expected handler.Handle to return true for matching op name")
	}

	res := fut.Await()
	if !res.IsErr() {
		t.Fatal("expected future to reject on type mismatch")
	}
	expectedSub := "request type mismatch for operation math_op: expected int, got string"
	if res.MustErr().Error() != expectedSub {
		t.Fatalf("expected error %q, got %q", expectedSub, res.MustErr().Error())
	}
}

func TestGoroutineContext_NoHandlerRegistered(t *testing.T) {
	task := Launch[adt.Unit, string](Config{}, adt.Void, func(co *Co, _ adt.Unit) adt.Result[string] {
		unregisteredOp := DefineOp[string, string]("unregistered")
		return co.Call(unregisteredOp, "test").Await()
	})

	res := task.Await()
	if !res.IsErr() {
		t.Fatal("expected error when no handler is registered")
	}
	expectedSub := "no handler registered for operation: unregistered"
	if res.MustErr().Error() != expectedSub {
		t.Fatalf("expected error %q, got %q", expectedSub, res.MustErr().Error())
	}
}

func TestGoroutineContext_MultipleOps(t *testing.T) {
	opA := DefineOp[int, int]("opA")
	opB := DefineOp[string, string]("opB")

	cfg := Config{
		OnCall: []CallHandler{
			opA.Handle(func(x int) adt.Result[int] { return adt.OK(x * 10) }),
			opB.Handle(func(s string) adt.Result[string] { return adt.OK("hello " + s) }),
		},
	}

	task := Launch[adt.Unit, string](cfg, adt.Void, func(co *Co, _ adt.Unit) adt.Result[string] {
		rA := co.Call(opA, 3).Await().MustGet()
		rB := co.Call(opB, "world").Await().MustGet()
		return adt.OK(fmt.Sprintf("%d:%s", rA, rB))
	})

	res := task.Await()
	if res.IsErr() {
		t.Fatalf("unexpected error: %v", res.MustErr())
	}
	if res.MustGet() != "30:hello world" {
		t.Fatalf("expected '30:hello world', got %q", res.MustGet())
	}
}

type syncContext struct {
	context.Context
}

func (s *syncContext) Call(op string, val any) *Future[any] {
	prom, fut := NewPromise[any](s.Context)
	prom.Resolve(val)
	return fut
}

func (s *syncContext) Async(fn func(ctx context.Context) adt.Result[any]) *Future[any] {
	prom, fut := NewPromise[any](s.Context)
	fn(s.Context).Fold(prom.Resolve, prom.Reject)
	return fut
}

func TestCoroutine_SynchronousFuture_SettlesSynchronously(t *testing.T) {
	op := DefineOp[int, int]("sync_op")
	ctx := &syncContext{Context: context.Background()}

	task := Launch[adt.Unit, int](Config{Context: ctx}, adt.Void, func(co *Co, _ adt.Unit) adt.Result[int] {
		futCall := co.Call(op, 42)
		select {
		case <-futCall.Context().Done():
		default:
			t.Error("expected futCall to be settled synchronously")
		}

		futAsync := co.Async(func(ctx context.Context) adt.Result[int] {
			return adt.OK(58)
		})
		select {
		case <-futAsync.Context().Done():
		default:
			t.Error("expected futAsync to be settled synchronously")
		}

		return adt.OK(futCall.Await().MustGet() + futAsync.Await().MustGet())
	})

	res := task.Await()
	if res.IsErr() {
		t.Fatalf("unexpected error: %v", res.MustErr())
	}
	if res.MustGet() != 100 {
		t.Fatalf("expected 100, got %d", res.MustGet())
	}
}

func TestPackagedTask_Run_CalledTwice_RunsOnce(t *testing.T) {
	var runs atomic.Int32

	task := PackagedTask[int, int](Config{}, func(co *Co, in int) adt.Result[int] {
		runs.Add(1)
		time.Sleep(10 * time.Millisecond)
		return adt.OK(in * 2)
	})

	var wg sync.WaitGroup
	results := make([]adt.Result[int], 10)
	for i := 0; i < 10; i++ {
		idx := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			results[idx] = task.Run(21)
		}()
	}
	wg.Wait()

	if runs.Load() != 1 {
		t.Fatalf("expected fn to execute exactly once, executed %d times", runs.Load())
	}
	for i, res := range results {
		if res.IsErr() {
			t.Fatalf("result %d returned error: %v", i, res.MustErr())
		}
		if res.MustGet() != 42 {
			t.Fatalf("result %d got %d, want 42", i, res.MustGet())
		}
	}
}

func TestLaunch_EquivalentToPackagedTaskPlusRun(t *testing.T) {
	fn := func(co *Co, in int) adt.Result[int] {
		return adt.OK(in + 100)
	}

	task1 := Launch[int, int](Config{}, 50, fn)
	task2 := PackagedTask[int, int](Config{}, fn)
	go task2.Run(50)

	r1 := task1.Await()
	r2 := task2.Await()

	if r1.IsErr() || r2.IsErr() {
		t.Fatalf("unexpected error: r1=%v, r2=%v", r1, r2)
	}
	if r1.MustGet() != r2.MustGet() || r1.MustGet() != 150 {
		t.Fatalf("expected 150, got r1=%d, r2=%d", r1.MustGet(), r2.MustGet())
	}
}

func TestTask_Future_IsSameAsFutAwait(t *testing.T) {
	task := Launch[adt.Unit, string](Config{}, adt.Void, func(co *Co, _ adt.Unit) adt.Result[string] {
		return adt.OK("task_future_ok")
	})

	resTask := task.Await()
	resFut := task.Future().Await()

	if resTask.IsErr() || resFut.IsErr() {
		t.Fatalf("unexpected error: resTask=%v, resFut=%v", resTask, resFut)
	}
	if resTask.MustGet() != resFut.MustGet() {
		t.Fatalf("expected equality, got %q and %q", resTask.MustGet(), resFut.MustGet())
	}
}

func TestTask_AwaitCtx_TimeoutDoesNotCancelFuture(t *testing.T) {
	task := Launch[adt.Unit, string](Config{}, adt.Void, func(co *Co, _ adt.Unit) adt.Result[string] {
		time.Sleep(50 * time.Millisecond)
		return adt.OK("eventual_success")
	})

	ctxTimeout, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
	defer cancel()

	timeoutRes := task.AwaitCtx(ctxTimeout)
	if !timeoutRes.IsErr() {
		t.Fatal("expected timeoutRes to be Err")
	}

	res := task.Await()
	if res.IsErr() {
		t.Fatalf("expected final task to succeed, got: %v", res.MustErr())
	}
	if res.MustGet() != "eventual_success" {
		t.Fatalf("expected eventual_success, got %q", res.MustGet())
	}
}

func TestCoroutine_Async_FnPanics(t *testing.T) {
	task := Launch[adt.Unit, string](Config{}, adt.Void, func(co *Co, _ adt.Unit) adt.Result[string] {
		fut := co.Async(func(ctx context.Context) adt.Result[string] {
			panic("something went horribly wrong")
		})
		return fut.Await()
	})

	res := task.Await()
	if !res.IsErr() {
		t.Fatal("expected error when async fn panics")
	}
	expectedMsg := "coroutine panic: something went horribly wrong"
	if res.MustErr().Error() != expectedMsg {
		t.Fatalf("expected error message %q, got %q", expectedMsg, res.MustErr().Error())
	}
}

func TestCoroutine_Call_HandlerFnPanics(t *testing.T) {
	panicOp := DefineOp[string, string]("panic_op")
	customErr := errors.New("custom panic error")

	cfg := Config{
		OnCall: []CallHandler{
			panicOp.Handle(func(s string) adt.Result[string] {
				panic(customErr)
			}),
		},
	}

	task := Launch[adt.Unit, string](cfg, adt.Void, func(co *Co, _ adt.Unit) adt.Result[string] {
		return co.Call(panicOp, "test").Await()
	})

	res := task.Await()
	if !res.IsErr() {
		t.Fatal("expected error when handler fn panics")
	}
	if !errors.Is(res.MustErr(), customErr) {
		t.Fatalf("expected custom error %v, got %v", customErr, res.MustErr())
	}
}

func TestCoroutine_Call_HandlerPanicsNonError(t *testing.T) {
	panicOp := DefineOp[string, string]("panic_str_op")

	cfg := Config{
		OnCall: []CallHandler{
			panicOp.Handle(func(s string) adt.Result[string] {
				panic("non-error panic payload")
			}),
		},
	}

	task := Launch[adt.Unit, string](cfg, adt.Void, func(co *Co, _ adt.Unit) adt.Result[string] {
		return co.Call(panicOp, "test").Await()
	})

	res := task.Await()
	if !res.IsErr() {
		t.Fatal("expected error when handler panics with string")
	}
	expectedMsg := "coroutine panic: non-error panic payload"
	if res.MustErr().Error() != expectedMsg {
		t.Fatalf("expected %q, got %q", expectedMsg, res.MustErr().Error())
	}
}

func TestCoroutine_Call_TwoSlowHandlers_RunInParallel(t *testing.T) {
	op1 := DefineOp[int, int]("op1")
	op2 := DefineOp[int, int]("op2")

	cfg := Config{
		OnCall: []CallHandler{
			op1.Handle(func(x int) adt.Result[int] {
				time.Sleep(50 * time.Millisecond)
				return adt.OK(x + 1)
			}),
			op2.Handle(func(x int) adt.Result[int] {
				time.Sleep(50 * time.Millisecond)
				return adt.OK(x + 2)
			}),
		},
	}

	start := time.Now()
	task := Launch[adt.Unit, int](cfg, adt.Void, func(co *Co, _ adt.Unit) adt.Result[int] {
		fut1 := co.Call(op1, 10)
		fut2 := co.Call(op2, 20)

		r1 := fut1.Await().MustGet()
		r2 := fut2.Await().MustGet()
		return adt.OK(r1 + r2)
	})

	res := task.Await()
	elapsed := time.Since(start)

	if res.IsErr() {
		t.Fatalf("unexpected error: %v", res.MustErr())
	}
	if res.MustGet() != 33 {
		t.Fatalf("expected 33, got %d", res.MustGet())
	}
	if elapsed >= 95*time.Millisecond {
		t.Fatalf("expected parallel execution under 95ms, took %v", elapsed)
	}
}

func TestCoroutine_Call_CancelledMidHandler(t *testing.T) {
	slowOp := DefineOp[int, int]("slow_op")
	handlerStarted := make(chan struct{})
	handlerRelease := make(chan struct{})

	cfg := Config{
		OnCall: []CallHandler{
			slowOp.Handle(func(x int) adt.Result[int] {
				close(handlerStarted)
				<-handlerRelease
				return adt.OK(x * 2)
			}),
		},
	}

	task := Launch[adt.Unit, int](cfg, adt.Void, func(co *Co, _ adt.Unit) adt.Result[int] {
		fut := co.Call(slowOp, 10)
		return fut.Await()
	})

	<-handlerStarted
	cancelErr := errors.New("aborted while handler running")
	task.Cancel(cancelErr)
	close(handlerRelease)

	res := task.Await()
	if !res.IsErr() {
		t.Fatal("expected task to return error on cancellation")
	}
	if !errors.Is(res.MustErr(), cancelErr) {
		t.Fatalf("expected cancel error %v, got %v", cancelErr, res.MustErr())
	}
}

func TestCoroutine_Async_AlreadyCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	cfg := Config{
		Context: NewGoroutineContext(ctx),
	}

	task := Launch[adt.Unit, string](cfg, adt.Void, func(co *Co, _ adt.Unit) adt.Result[string] {
		if co.Err() != nil {
			return adt.Err[string](co.Err())
		}
		return co.Async(func(childCtx context.Context) adt.Result[string] {
			return adt.OK("should not run")
		}).Await()
	})

	res := task.Await()
	if !res.IsErr() {
		t.Fatal("expected task to fail with cancelled context error")
	}
	if !errors.Is(res.MustErr(), context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", res.MustErr())
	}
}

func TestCoroutine_Emit_InvokesOnEmit(t *testing.T) {
	var emitted any
	cfg := Config{
		OnEmit: func(v any) {
			emitted = v
		},
	}

	task := Launch[adt.Unit, string](cfg, adt.Void, func(co *Co, _ adt.Unit) adt.Result[string] {
		co.Emit("progress_50")
		return adt.OK("done")
	})

	res := task.Await()
	if res.IsErr() {
		t.Fatalf("unexpected error: %v", res.MustErr())
	}
	if emitted != "progress_50" {
		t.Fatalf("expected emitted 'progress_50', got %v", emitted)
	}
}

func TestCoroutine_Emit_NilOnEmit_NoPanic(t *testing.T) {
	cfg := Config{
		OnEmit: nil,
	}

	task := Launch[adt.Unit, string](cfg, adt.Void, func(co *Co, _ adt.Unit) adt.Result[string] {
		co.Emit("ignored")
		return adt.OK("ok")
	})

	res := task.Await()
	if res.IsErr() {
		t.Fatalf("unexpected error: %v", res.MustErr())
	}
	if res.MustGet() != "ok" {
		t.Fatalf("expected 'ok', got %q", res.MustGet())
	}
}

func TestCoroutine_Emit_MultipleValues_InOrder(t *testing.T) {
	var mu sync.Mutex
	var values []int

	cfg := Config{
		OnEmit: func(v any) {
			mu.Lock()
			defer mu.Unlock()
			values = append(values, v.(int))
		},
	}

	task := Launch[adt.Unit, string](cfg, adt.Void, func(co *Co, _ adt.Unit) adt.Result[string] {
		co.Emit(1)
		co.Emit(2)
		co.Emit(3)
		return adt.OK("done")
	})

	res := task.Await()
	if res.IsErr() {
		t.Fatalf("unexpected error: %v", res.MustErr())
	}

	mu.Lock()
	defer mu.Unlock()
	if len(values) != 3 || values[0] != 1 || values[1] != 2 || values[2] != 3 {
		t.Fatalf("expected [1, 2, 3], got %v", values)
	}
}

func TestDurableContext_BasicCallReplay(t *testing.T) {
	journal := NewJournal()
	op := DefineOp[int, int]("double")
	var handlerCalls atomic.Int32

	cfg1 := Config{
		Context: NewDurableContext(context.Background(), journal),
		OnCall: []CallHandler{
			op.Handle(func(x int) adt.Result[int] {
				handlerCalls.Add(1)
				return adt.OK(x * 2)
			}),
		},
	}
	task1 := Launch[int, int](cfg1, 21, func(co *Co, in int) adt.Result[int] {
		return co.Call(op, in).Await()
	})
	res1 := task1.Await()
	if res1.IsErr() || res1.MustGet() != 42 {
		t.Fatalf("run 1 failed: %v", res1)
	}
	if handlerCalls.Load() != 1 {
		t.Fatalf("expected 1 call, got %d", handlerCalls.Load())
	}

	// Turn 2: Replay with same journal, SetMaxSteps(0)
	dCtx2 := NewDurableContext(context.Background(), journal)
	dCtx2.SetMaxSteps(0)
	cfg2 := Config{
		Context: dCtx2,
		OnCall: []CallHandler{
			op.Handle(func(x int) adt.Result[int] {
				handlerCalls.Add(1)
				return adt.OK(x * 2)
			}),
		},
	}
	task2 := Launch[int, int](cfg2, 21, func(co *Co, in int) adt.Result[int] {
		return co.Call(op, in).Await()
	})
	res2 := task2.Await()
	if res2.IsErr() || res2.MustGet() != 42 {
		t.Fatalf("run 2 replay failed: %v", res2)
	}
	if handlerCalls.Load() != 1 {
		t.Fatalf("expected handler NOT to be called on replay, calls=%d", handlerCalls.Load())
	}
}

func TestDurableContext_BasicAsyncReplay(t *testing.T) {
	journal := NewJournal()
	var asyncCalls atomic.Int32

	cfg1 := Config{
		Context: NewDurableContext(context.Background(), journal),
	}
	task1 := Launch[adt.Unit, string](cfg1, adt.Void, func(co *Co, _ adt.Unit) adt.Result[string] {
		return co.Async(func(ctx context.Context) adt.Result[string] {
			asyncCalls.Add(1)
			return adt.OK("async_result")
		}).Await()
	})
	res1 := task1.Await()
	if res1.IsErr() || res1.MustGet() != "async_result" {
		t.Fatalf("run 1 failed: %v", res1)
	}
	if asyncCalls.Load() != 1 {
		t.Fatalf("expected 1 call, got %d", asyncCalls.Load())
	}

	// Turn 2: Replay with same journal, SetMaxSteps(0)
	dCtx2 := NewDurableContext(context.Background(), journal)
	dCtx2.SetMaxSteps(0)
	cfg2 := Config{Context: dCtx2}
	task2 := Launch[adt.Unit, string](cfg2, adt.Void, func(co *Co, _ adt.Unit) adt.Result[string] {
		return co.Async(func(ctx context.Context) adt.Result[string] {
			asyncCalls.Add(1)
			return adt.OK("async_result")
		}).Await()
	})
	res2 := task2.Await()
	if res2.IsErr() || res2.MustGet() != "async_result" {
		t.Fatalf("run 2 replay failed: %v", res2)
	}
	if asyncCalls.Load() != 1 {
		t.Fatalf("expected async NOT to be called on replay, calls=%d", asyncCalls.Load())
	}
}

func TestDurableContext_HandlerError_NotJournaled(t *testing.T) {
	journal := NewJournal()
	errOp := DefineOp[int, int]("err_op")

	cfg := Config{
		Context: NewDurableContext(context.Background(), journal),
		OnCall: []CallHandler{
			errOp.Handle(func(x int) adt.Result[int] {
				return adt.Err[int](errors.New("handler failed"))
			}),
		},
	}
	task := Launch[adt.Unit, int](cfg, adt.Void, func(co *Co, _ adt.Unit) adt.Result[int] {
		return co.Call(errOp, 10).Await()
	})
	res := task.Await()
	if !res.IsErr() {
		t.Fatal("expected error")
	}
	if journal.Len() != 0 {
		t.Fatalf("expected journal to remain empty, got %d", journal.Len())
	}
}

func TestDurableContext_HandlerPanic_NotJournaled(t *testing.T) {
	journal := NewJournal()
	panicOp := DefineOp[int, int]("panic_op")

	cfg := Config{
		Context: NewDurableContext(context.Background(), journal),
		OnCall: []CallHandler{
			panicOp.Handle(func(x int) adt.Result[int] {
				panic("boom")
			}),
		},
	}
	task := Launch[adt.Unit, int](cfg, adt.Void, func(co *Co, _ adt.Unit) adt.Result[int] {
		return co.Call(panicOp, 10).Await()
	})
	res := task.Await()
	if !res.IsErr() {
		t.Fatal("expected error")
	}
	if journal.Len() != 0 {
		t.Fatalf("expected journal to remain empty, got %d", journal.Len())
	}
}

func TestDurableContext_MaxSteps_ZeroSuspendsOnFirstCall(t *testing.T) {
	journal := NewJournal()
	dCtx := NewDurableContext(context.Background(), journal)
	dCtx.SetMaxSteps(0)
	op := DefineOp[int, int]("op")

	cfg := Config{
		Context: dCtx,
		OnCall: []CallHandler{
			op.Handle(func(x int) adt.Result[int] { return adt.OK(x) }),
		},
	}
	task := Launch[adt.Unit, int](cfg, adt.Void, func(co *Co, _ adt.Unit) adt.Result[int] {
		return co.Call(op, 5).Await()
	})
	res := task.Await()
	if !res.IsErr() || !errors.Is(res.MustErr(), ErrSuspended) {
		t.Fatalf("expected ErrSuspended, got %v", res)
	}
	if journal.Len() != 0 {
		t.Fatalf("expected journal len 0, got %d", journal.Len())
	}
}

func TestDurableContext_MaxSteps_OneAllowsFirstOnly(t *testing.T) {
	journal := NewJournal()
	dCtx := NewDurableContext(context.Background(), journal)
	dCtx.SetMaxSteps(1)
	op1 := DefineOp[int, int]("op1")
	op2 := DefineOp[int, int]("op2")

	cfg := Config{
		Context: dCtx,
		OnCall: []CallHandler{
			op1.Handle(func(x int) adt.Result[int] { return adt.OK(x + 1) }),
			op2.Handle(func(x int) adt.Result[int] { return adt.OK(x + 2) }),
		},
	}
	task := Launch[adt.Unit, int](cfg, adt.Void, func(co *Co, _ adt.Unit) adt.Result[int] {
		r1 := co.Call(op1, 10).Await()
		if r1.IsErr() {
			return r1
		}
		return co.Call(op2, 20).Await()
	})
	res := task.Await()
	if !res.IsErr() || !errors.Is(res.MustErr(), ErrSuspended) {
		t.Fatalf("expected ErrSuspended, got %v", res)
	}
	if journal.Len() != 1 {
		t.Fatalf("expected journal len 1, got %d", journal.Len())
	}
}

func TestDurableContext_Restart_HandlerCalledExactlyOnce(t *testing.T) {
	journal := NewJournal()
	op1 := DefineOp[int, int]("step1")
	op2 := DefineOp[int, int]("step2")

	var h1Calls, h2Calls atomic.Int32

	handlers := []CallHandler{
		op1.Handle(func(x int) adt.Result[int] {
			h1Calls.Add(1)
			return adt.OK(x * 2)
		}),
		op2.Handle(func(x int) adt.Result[int] {
			h2Calls.Add(1)
			return adt.OK(x * 3)
		}),
	}

	workflow := func(co *Co, in int) adt.Result[int] {
		r1 := co.Call(op1, in).Await()
		if r1.IsErr() {
			return r1
		}
		r2 := co.Call(op2, r1.MustGet()).Await()
		if r2.IsErr() {
			return r2
		}
		return r2
	}

	// Turn 0: maxSteps = 0 -> suspends before step1
	dCtx0 := NewDurableContext(context.Background(), journal)
	dCtx0.SetMaxSteps(0)
	t0 := Launch[int, int](Config{Context: dCtx0, OnCall: handlers}, 5, workflow)
	if !errors.Is(t0.Await().MustErr(), ErrSuspended) {
		t.Fatal("turn 0 expected ErrSuspended")
	}

	// Turn 1: maxSteps = 1 -> runs step1, suspends before step2
	dCtx1 := NewDurableContext(context.Background(), journal)
	dCtx1.SetMaxSteps(1)
	t1 := Launch[int, int](Config{Context: dCtx1, OnCall: handlers}, 5, workflow)
	if !errors.Is(t1.Await().MustErr(), ErrSuspended) {
		t.Fatal("turn 1 expected ErrSuspended")
	}

	// Turn 2: maxSteps = -1 -> replays step1 from journal, runs step2 to finish
	dCtx2 := NewDurableContext(context.Background(), journal)
	t2 := Launch[int, int](Config{Context: dCtx2, OnCall: handlers}, 5, workflow)
	res := t2.Await()
	if res.IsErr() || res.MustGet() != 30 {
		t.Fatalf("turn 2 failed: %v", res)
	}

	if h1Calls.Load() != 1 {
		t.Fatalf("expected h1 called exactly once, got %d", h1Calls.Load())
	}
	if h2Calls.Load() != 1 {
		t.Fatalf("expected h2 called exactly once, got %d", h2Calls.Load())
	}
}

func TestDurableContext_MixedAsyncAndCall_ReplayOrder(t *testing.T) {
	journal := NewJournal()
	op := DefineOp[string, string]("echo_op")
	handlers := []CallHandler{
		op.Handle(func(s string) adt.Result[string] {
			return adt.OK("called:" + s)
		}),
	}

	workflow := func(co *Co, _ adt.Unit) adt.Result[string] {
		// Step 0: Async
		a1 := co.Async(func(ctx context.Context) adt.Result[string] {
			return adt.OK("async_1")
		}).Await()
		if a1.IsErr() {
			return a1
		}

		// Step 1: Call
		c1 := co.Call(op, a1.MustGet()).Await()
		if c1.IsErr() {
			return c1
		}

		// Step 2: Async
		a2 := co.Async(func(ctx context.Context) adt.Result[string] {
			return adt.OK("async_2")
		}).Await()
		if a2.IsErr() {
			return a2
		}

		return adt.OK(fmt.Sprintf("%s|%s|%s", a1.MustGet(), c1.MustGet(), a2.MustGet()))
	}

	// Run step 0 only
	d0 := NewDurableContext(context.Background(), journal)
	d0.SetMaxSteps(1)
	Launch[adt.Unit, string](Config{Context: d0, OnCall: handlers}, adt.Void, workflow).Await()
	if journal.Len() != 1 {
		t.Fatalf("expected 1 entry, got %d", journal.Len())
	}

	// Run step 1
	d1 := NewDurableContext(context.Background(), journal)
	d1.SetMaxSteps(2)
	Launch[adt.Unit, string](Config{Context: d1, OnCall: handlers}, adt.Void, workflow).Await()
	if journal.Len() != 2 {
		t.Fatalf("expected 2 entries, got %d", journal.Len())
	}

	// Run all to completion
	d2 := NewDurableContext(context.Background(), journal)
	res := Launch[adt.Unit, string](Config{Context: d2, OnCall: handlers}, adt.Void, workflow).Await()
	if res.IsErr() {
		t.Fatalf("unexpected error: %v", res.MustErr())
	}
	expected := "async_1|called:async_1|async_2"
	if res.MustGet() != expected {
		t.Fatalf("expected %q, got %q", expected, res.MustGet())
	}
}







