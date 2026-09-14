package async

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/azuiktech/kleisli-go/adt"
)

func TestPromise_Resolve(t *testing.T) {
	prom, fut := NewPromise[int](context.Background())

	go func() {
		time.Sleep(10 * time.Millisecond)
		if !prom.Resolve(42) {
			t.Errorf("expected Resolve to return true")
		}
	}()

	res := fut.Await()
	if res.IsErr() {
		t.Fatalf("expected OK, got error: %v", res.MustErr())
	}
	if res.MustGet() != 42 {
		t.Errorf("got %d, want 42", res.MustGet())
	}
}

func TestPromise_Reject(t *testing.T) {
	prom, fut := NewPromise[string](context.Background())
	expectedErr := errors.New("something went wrong")

	go func() {
		time.Sleep(10 * time.Millisecond)
		if !prom.Reject(expectedErr) {
			t.Errorf("expected Reject to return true")
		}
	}()

	res := fut.Await()
	if res.IsOK() {
		t.Fatalf("expected Err, got OK: %v", res.MustGet())
	}
	if !errors.Is(res.MustErr(), expectedErr) {
		t.Errorf("got %v, want %v", res.MustErr(), expectedErr)
	}
}

func TestPromise_Cancel(t *testing.T) {
	prom, fut := NewPromise[int](context.Background())
	cause := errors.New("cancelled by producer")

	if !prom.Cancel(cause) {
		t.Errorf("expected Cancel to return true")
	}

	res := fut.Await()
	if res.IsOK() {
		t.Fatalf("expected Err, got %v", res.MustGet())
	}
	if !errors.Is(res.MustErr(), cause) {
		t.Errorf("got %v, want %v", res.MustErr(), cause)
	}
}

func TestPromise_Cancel_NilDefaultsToContextCanceled(t *testing.T) {
	prom, fut := NewPromise[int](context.Background())

	if !prom.Cancel(nil) {
		t.Errorf("expected Cancel to return true")
	}

	res := fut.Await()
	if res.IsOK() {
		t.Fatalf("expected Err, got %v", res.MustGet())
	}
	if !errors.Is(res.MustErr(), context.Canceled) {
		t.Errorf("got %v, want context.Canceled", res.MustErr())
	}
}

func TestPromise_SingleAssignment(t *testing.T) {
	prom, fut := NewPromise[int](context.Background())

	if !prom.Resolve(100) {
		t.Errorf("first Resolve should return true")
	}
	if prom.Resolve(200) {
		t.Errorf("second Resolve should return false")
	}
	if prom.Reject(errors.New("err")) {
		t.Errorf("subsequent Reject should return false")
	}
	if prom.Cancel(errors.New("cancel")) {
		t.Errorf("subsequent Cancel should return false")
	}

	res := fut.Await()
	if res.IsErr() || res.MustGet() != 100 {
		t.Errorf("expected 100, got %v", res)
	}
}

func TestFuture_Cancel(t *testing.T) {
	prom, fut := NewPromise[int](context.Background())
	cause := errors.New("cancelled by consumer")

	if !fut.Cancel(cause) {
		t.Errorf("expected Cancel to return true")
	}
	if fut.Cancel(cause) {
		t.Errorf("subsequent Cancel should return false")
	}

	if prom.Resolve(999) {
		t.Errorf("Resolve after consumer cancel should return false")
	}

	res := fut.Await()
	if res.IsOK() {
		t.Fatalf("expected Err, got %v", res.MustGet())
	}
	if !errors.Is(res.MustErr(), cause) {
		t.Errorf("got %v, want %v", res.MustErr(), cause)
	}
}

func TestFuture_Cancel_NilDefaultsToContextCanceled(t *testing.T) {
	prom, fut := NewPromise[int](context.Background())

	if !fut.Cancel(nil) {
		t.Errorf("expected Cancel to return true")
	}

	if prom.Resolve(999) {
		t.Errorf("Resolve after consumer cancel should return false")
	}

	res := fut.Await()
	if res.IsOK() {
		t.Fatalf("expected Err, got %v", res.MustGet())
	}
	if !errors.Is(res.MustErr(), context.Canceled) {
		t.Errorf("got %v, want context.Canceled", res.MustErr())
	}
}

func TestFuture_ParentContextCancel(t *testing.T) {
	parentCtx, parentCancel := context.WithCancel(context.Background())
	prom, fut := NewPromise[int](parentCtx)

	parentCancel()

	res := fut.Await()
	if res.IsOK() {
		t.Fatalf("expected Err, got %v", res.MustGet())
	}
	if !errors.Is(res.MustErr(), context.Canceled) {
		t.Errorf("got %v, want context.Canceled", res.MustErr())
	}

	if prom.Resolve(42) {
		t.Errorf("Resolve after parent cancel should return false")
	}
}

func TestFuture_AwaitCtxTimeout(t *testing.T) {
	prom, fut := NewPromise[string](context.Background())

	timeoutCtx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	res := fut.AwaitCtx(timeoutCtx)
	if res.IsOK() {
		t.Fatalf("expected timeout Err, got %v", res.MustGet())
	}
	if !errors.Is(res.MustErr(), context.DeadlineExceeded) {
		t.Errorf("got %v, want context.DeadlineExceeded", res.MustErr())
	}

	if !prom.Resolve("eventual-success") {
		t.Errorf("future should still be resolvable after AwaitCtx timeout")
	}

	finalRes := fut.Await()
	if finalRes.IsErr() || finalRes.MustGet() != "eventual-success" {
		t.Errorf("expected eventual-success, got %v", finalRes)
	}
}

func TestFuture_ConcurrentResolveAndCancel(t *testing.T) {
	const iters = 100
	for range iters {
		prom, fut := NewPromise[int](context.Background())
		var wg sync.WaitGroup
		wg.Add(2)

		go func() {
			defer wg.Done()
			prom.Resolve(42)
		}()

		go func() {
			defer wg.Done()
			fut.Cancel(errors.New("cancel race"))
		}()

		wg.Wait()
		res := fut.Await()
		if res.IsOK() {
			if res.MustGet() != 42 {
				t.Errorf("unexpected value: %d", res.MustGet())
			}
		} else {
			if res.MustErr() == nil {
				t.Errorf("expected non-nil error")
			}
		}
	}
}

func TestAllOf_Empty(t *testing.T) {
	fut := AllOf[int]()
	res := fut.Await()
	if res.IsErr() {
		t.Fatalf("expected OK, got error: %v", res.MustErr())
	}
	if len(res.MustGet()) != 0 {
		t.Fatalf("expected empty slice, got: %v", res.MustGet())
	}
}

func TestAllOf_MixedPrimitivesSuccess(t *testing.T) {
	p1, fut1 := NewPromise[int](context.Background())
	p2, _ := NewPromise[int](context.Background())
	task := Launch(Config{}, 30, func(_ *Co, in int) adt.Result[int] {
		time.Sleep(10 * time.Millisecond)
		return adt.OK(in)
	})

	go func() {
		time.Sleep(15 * time.Millisecond)
		p1.Resolve(10)
	}()
	go func() {
		time.Sleep(5 * time.Millisecond)
		p2.Resolve(20)
	}()

	combined := AllOf[int](fut1, p2, task)
	res := combined.Await()
	if res.IsErr() {
		t.Fatalf("expected OK, got: %v", res.MustErr())
	}

	want := []int{10, 20, 30}
	if !reflect.DeepEqual(res.MustGet(), want) {
		t.Fatalf("got %v, want %v", res.MustGet(), want)
	}
}

func TestAllOf_FailFastAndCancel(t *testing.T) {
	p1, fut1 := NewPromise[int](context.Background())
	errFail := errors.New("boom")

	taskSlow := Launch(Config{}, 0, func(co *Co, _ int) adt.Result[int] {
		select {
		case <-time.After(200 * time.Millisecond):
			return adt.OK(999)
		case <-co.Done():
			return adt.Err[int](co.Err())
		}
	})

	go func() {
		time.Sleep(10 * time.Millisecond)
		fut1.Cancel(errFail)
	}()

	combined := AllOf[int](p1, taskSlow)
	start := time.Now()
	res := combined.Await()
	elapsed := time.Since(start)

	if res.IsOK() {
		t.Fatalf("expected error, got OK: %v", res.MustGet())
	}
	if !errors.Is(res.MustErr(), errFail) {
		t.Fatalf("got %v, want %v", res.MustErr(), errFail)
	}
	if elapsed > 100*time.Millisecond {
		t.Fatalf("AllOf did not fail fast, took %v", elapsed)
	}

	slowRes := taskSlow.Await()
	if slowRes.IsOK() {
		t.Fatalf("expected taskSlow to be cancelled, got OK: %v", slowRes.MustGet())
	}
}

func TestAllOf_ExternalCancelPropagation(t *testing.T) {
	p1, _ := NewPromise[int](context.Background())
	p2, _ := NewPromise[int](context.Background())

	combined := AllOf[int](p1, p2)
	customErr := errors.New("caller cancelled")

	go func() {
		time.Sleep(10 * time.Millisecond)
		combined.Cancel(customErr)
	}()

	res := combined.Await()
	if res.IsOK() {
		t.Fatalf("expected error, got: %v", res.MustGet())
	}
	if !errors.Is(res.MustErr(), customErr) {
		t.Fatalf("got %v, want %v", res.MustErr(), customErr)
	}

	r1 := p1.Future().Await()
	if !errors.Is(r1.MustErr(), customErr) {
		t.Fatalf("p1 cancelled err got %v, want %v", r1.MustErr(), customErr)
	}
}

func TestAllSettled_Empty(t *testing.T) {
	fut := AllSettled[int]()
	res := fut.Await()
	if res.IsErr() {
		t.Fatalf("expected OK, got: %v", res.MustErr())
	}
	if len(res.MustGet()) != 0 {
		t.Fatalf("expected empty slice, got: %v", res.MustGet())
	}
}

func TestAllSettled_MixedResults(t *testing.T) {
	p1, _ := NewPromise[int](context.Background())
	p2, _ := NewPromise[int](context.Background())
	err2 := errors.New("err in p2")

	task := Launch(Config{}, 300, func(_ *Co, in int) adt.Result[int] {
		return adt.OK(in)
	})

	go func() {
		time.Sleep(10 * time.Millisecond)
		p1.Resolve(100)
	}()
	go func() {
		time.Sleep(5 * time.Millisecond)
		p2.Reject(err2)
	}()

	combined := AllSettled[int](p1, p2, task)
	res := combined.Await()
	if res.IsErr() {
		t.Fatalf("expected AllSettled to succeed with results, got: %v", res.MustErr())
	}

	results := res.MustGet()
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	if results[0].MustGet() != 100 {
		t.Errorf("result[0] got %v, want 100", results[0])
	}
	if !errors.Is(results[1].MustErr(), err2) {
		t.Errorf("result[1] got err %v, want %v", results[1].MustErr(), err2)
	}
	if results[2].MustGet() != 300 {
		t.Errorf("result[2] got %v, want 300", results[2])
	}
}

func TestRace_Empty(t *testing.T) {
	fut := Race[int]()
	res := fut.Await()
	if res.IsOK() {
		t.Fatalf("expected error for empty Race, got %v", res.MustGet())
	}
	if !errors.Is(res.MustErr(), ErrEmptySources) {
		t.Fatalf("got %v, want ErrEmptySources", res.MustErr())
	}
}

func TestRace_FirstSuccessWins(t *testing.T) {
	p1, _ := NewPromise[string](context.Background())
	p2, _ := NewPromise[string](context.Background())

	go func() {
		time.Sleep(5 * time.Millisecond)
		p1.Resolve("winner")
	}()
	go func() {
		time.Sleep(50 * time.Millisecond)
		p2.Resolve("loser")
	}()

	res := Race[string](p1, p2).Await()
	if res.IsErr() {
		t.Fatalf("expected OK, got: %v", res.MustErr())
	}
	if res.MustGet() != "winner" {
		t.Fatalf("got %s, want winner", res.MustGet())
	}

	p2Res := p2.Future().Await()
	if p2Res.IsOK() {
		t.Fatalf("expected loser to be cancelled, got OK: %v", p2Res.MustGet())
	}
}

func TestRace_FirstFailureWins(t *testing.T) {
	p1, _ := NewPromise[int](context.Background())
	p2, _ := NewPromise[int](context.Background())
	errFail := errors.New("fast failure")

	go func() {
		time.Sleep(5 * time.Millisecond)
		p1.Reject(errFail)
	}()
	go func() {
		time.Sleep(50 * time.Millisecond)
		p2.Resolve(42)
	}()

	res := Race[int](p1, p2).Await()
	if res.IsOK() {
		t.Fatalf("expected Err, got: %v", res.MustGet())
	}
	if !errors.Is(res.MustErr(), errFail) {
		t.Fatalf("got %v, want %v", res.MustErr(), errFail)
	}

	p2Res := p2.Future().Await()
	if p2Res.IsOK() {
		t.Fatalf("expected p2 to be cancelled, got OK: %v", p2Res.MustGet())
	}
}

func TestAnyOf_Empty(t *testing.T) {
	fut := AnyOf[int]()
	res := fut.Await()
	if res.IsOK() {
		t.Fatalf("expected error for empty AnyOf, got %v", res.MustGet())
	}
	if !errors.Is(res.MustErr(), ErrEmptySources) {
		t.Fatalf("got %v, want ErrEmptySources", res.MustErr())
	}
}

func TestAnyOf_FirstSuccessAfterFailure(t *testing.T) {
	p1, _ := NewPromise[string](context.Background())
	p2, _ := NewPromise[string](context.Background())
	p3, _ := NewPromise[string](context.Background())

	go func() {
		time.Sleep(5 * time.Millisecond)
		p1.Reject(errors.New("p1 failed"))
	}()
	go func() {
		time.Sleep(15 * time.Millisecond)
		p2.Resolve("success")
	}()
	go func() {
		time.Sleep(50 * time.Millisecond)
		p3.Resolve("too late")
	}()

	res := AnyOf[string](p1, p2, p3).Await()
	if res.IsErr() {
		t.Fatalf("expected OK, got: %v", res.MustErr())
	}
	if res.MustGet() != "success" {
		t.Fatalf("got %s, want success", res.MustGet())
	}

	p3Res := p3.Future().Await()
	if p3Res.IsOK() {
		t.Fatalf("expected p3 to be cancelled, got OK: %v", p3Res.MustGet())
	}
}

func TestAnyOf_AllFailed(t *testing.T) {
	p1, _ := NewPromise[int](context.Background())
	p2, _ := NewPromise[int](context.Background())
	e1 := errors.New("err 1")
	e2 := errors.New("err 2")

	go func() {
		p1.Reject(e1)
		p2.Reject(e2)
	}()

	res := AnyOf[int](p1, p2).Await()
	if res.IsOK() {
		t.Fatalf("expected error when all fail, got: %v", res.MustGet())
	}
	if !errors.Is(res.MustErr(), ErrAllFailed) {
		t.Fatalf("got %v, want ErrAllFailed", res.MustErr())
	}
	if !errors.Is(res.MustErr(), e1) || !errors.Is(res.MustErr(), e2) {
		t.Fatalf("expected error to wrap both e1 and e2, got: %v", res.MustErr())
	}
}

func TestZip2_Success(t *testing.T) {
	p1, _ := NewPromise[int](context.Background())
	p2, _ := NewPromise[string](context.Background())

	go func() {
		time.Sleep(10 * time.Millisecond)
		p1.Resolve(42)
	}()
	go func() {
		time.Sleep(5 * time.Millisecond)
		p2.Resolve("hello")
	}()

	fut := Zip2[int, string](p1, p2)
	res := fut.Await()
	if res.IsErr() {
		t.Fatalf("expected OK, got: %v", res.MustErr())
	}
	pair := res.MustGet()
	if pair.First() != 42 || pair.Second() != "hello" {
		t.Fatalf("got (%d, %q), want (42, \"hello\")", pair.First(), pair.Second())
	}
}

func TestZip2_FailFast(t *testing.T) {
	p1, _ := NewPromise[int](context.Background())
	p2, _ := NewPromise[string](context.Background())
	errBoom := errors.New("boom")

	go func() {
		time.Sleep(5 * time.Millisecond)
		p1.Reject(errBoom)
	}()

	fut := Zip2[int, string](p1, p2)
	res := fut.Await()
	if res.IsOK() {
		t.Fatalf("expected Err, got: %v", res.MustGet())
	}
	if !errors.Is(res.MustErr(), errBoom) {
		t.Fatalf("got %v, want %v", res.MustErr(), errBoom)
	}

	p2Res := p2.Future().Await()
	if p2Res.IsOK() {
		t.Fatalf("expected p2 to be cancelled, got OK: %v", p2Res.MustGet())
	}
}

func TestZip3_Success(t *testing.T) {
	p1, _ := NewPromise[int](context.Background())
	p2, _ := NewPromise[string](context.Background())
	p3, _ := NewPromise[bool](context.Background())

	go func() {
		p1.Resolve(1)
		p2.Resolve("two")
		p3.Resolve(true)
	}()

	fut := Zip3[int, string, bool](p1, p2, p3)
	res := fut.Await()
	if res.IsErr() {
		t.Fatalf("expected OK, got: %v", res.MustErr())
	}
	triple := res.MustGet()
	if triple.First() != 1 || triple.Second() != "two" || !triple.Third() {
		t.Fatalf("got %v, want (1, two, true)", triple)
	}
}

func TestZip3_FailFast(t *testing.T) {
	p1, _ := NewPromise[int](context.Background())
	p2, _ := NewPromise[string](context.Background())
	p3, _ := NewPromise[bool](context.Background())
	errBoom := errors.New("boom3")

	go func() {
		p2.Reject(errBoom)
	}()

	fut := Zip3[int, string, bool](p1, p2, p3)
	res := fut.Await()
	if res.IsOK() {
		t.Fatalf("expected Err, got: %v", res.MustGet())
	}
	if !errors.Is(res.MustErr(), errBoom) {
		t.Fatalf("got %v, want %v", res.MustErr(), errBoom)
	}

	if p1.Future().Await().IsOK() {
		t.Fatalf("expected p1 to be cancelled")
	}
	if p3.Future().Await().IsOK() {
		t.Fatalf("expected p3 to be cancelled")
	}
}
