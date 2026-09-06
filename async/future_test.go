package async

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
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
