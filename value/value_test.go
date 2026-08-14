package value_test

import (
	"errors"
	"testing"

	"github.com/azuiktech/kleisli-go/value"
)

func TestMust(t *testing.T) {
	got := value.Must("hello", nil)
	if got != "hello" {
		t.Errorf("Must unexpected value: got %q, want %q", got, "hello")
	}

	defer func() {
		r := recover()
		if r == nil {
			t.Errorf("Must expected panic on error, but did not panic")
		}
	}()
	_ = value.Must("fail", errors.New("err"))
}

func TestFallback(t *testing.T) {
	if got := value.Fallback("success", nil, "fallback"); got != "success" {
		t.Errorf("Fallback got %q, want %q", got, "success")
	}
	if got := value.Fallback("success", errors.New("err"), "fallback"); got != "fallback" {
		t.Errorf("Fallback got %q, want %q", got, "fallback")
	}
}

func TestFallbackGet(t *testing.T) {
	if got := value.FallbackGet("ok", nil, func(err error) string { return "fallback" }); got != "ok" {
		t.Errorf("FallbackGet got %q, want %q", got, "ok")
	}
	called := false
	got := value.FallbackGet("ok", errors.New("err"), func(err error) string {
		called = true
		return err.Error()
	})
	if !called || got != "err" {
		t.Errorf("FallbackGet failed: called=%v, got=%q", called, got)
	}
}

func TestCond(t *testing.T) {
	if got := value.Cond(true, 1, 2); got != 1 {
		t.Errorf("Cond(true) got %d, want 1", got)
	}
	if got := value.Cond(false, 1, 2); got != 2 {
		t.Errorf("Cond(false) got %d, want 2", got)
	}
}

func TestCondGet(t *testing.T) {
	trueCalled, falseCalled := false, false
	got := value.CondGet(
		true,
		func() int { trueCalled = true; return 10 },
		func() int { falseCalled = true; return 20 },
	)
	if got != 10 || !trueCalled || falseCalled {
		t.Errorf("CondGet(true) failed: got=%d, trueCalled=%v, falseCalled=%v", got, trueCalled, falseCalled)
	}

	trueCalled, falseCalled = false, false
	got = value.CondGet(
		false,
		func() int { trueCalled = true; return 10 },
		func() int { falseCalled = true; return 20 },
	)
	if got != 20 || trueCalled || !falseCalled {
		t.Errorf("CondGet(false) failed: got=%d, trueCalled=%v, falseCalled=%v", got, trueCalled, falseCalled)
	}
}

func TestPtrAndDeref(t *testing.T) {
	v := 42
	p := value.Ptr(v)
	if p == nil || *p != v {
		t.Fatalf("Ptr failed: got %v", p)
	}

	if got := value.Deref(p, 0); got != 42 {
		t.Errorf("Deref(non-nil) got %d, want 42", got)
	}
	if got := value.Deref[int](nil, 100); got != 100 {
		t.Errorf("Deref(nil) got %d, want 100", got)
	}

	if got := value.DerefGet(p, func() int { return 999 }); got != 42 {
		t.Errorf("DerefGet(non-nil) got %d, want 42", got)
	}
	if got := value.DerefGet[int](nil, func() int { return 999 }); got != 999 {
		t.Errorf("DerefGet(nil) got %d, want 999", got)
	}

	if got := value.DerefZero(p); got != 42 {
		t.Errorf("DerefZero(non-nil) got %d, want 42", got)
	}
	if got := value.DerefZero[int](nil); got != 0 {
		t.Errorf("DerefZero(nil) got %d, want 0", got)
	}
}

func TestTap(t *testing.T) {
	var sideEffectVal string
	res := value.Tap("input", func(s string) {
		sideEffectVal = s
	})
	if res != "input" || sideEffectVal != "input" {
		t.Errorf("Tap failed: res=%q, sideEffectVal=%q", res, sideEffectVal)
	}
}

func TestPipe(t *testing.T) {
	double := func(x int) int { return x * 2 }
	if got := value.Pipe(21, double); got != 42 {
		t.Errorf("Pipe got %d, want 42", got)
	}
}

func TestZeroAndIsZero(t *testing.T) {
	if got := value.Zero[int](); got != 0 {
		t.Errorf("Zero[int]() got %d, want 0", got)
	}
	if !value.IsZero(0) || !value.IsZero("") {
		t.Errorf("IsZero failed for zero values")
	}
	if value.IsZero(42) || value.IsZero("hello") {
		t.Errorf("IsZero reported true for non-zero values")
	}
}

func TestCoalesce(t *testing.T) {
	if got := value.Coalesce("", "", "first", "second"); got != "first" {
		t.Errorf("Coalesce got %q, want %q", got, "first")
	}
	if got := value.Coalesce[int](0, 0, 0); got != 0 {
		t.Errorf("Coalesce all zeros got %d, want 0", got)
	}
}

func TestWrapErrAndMapErr(t *testing.T) {
	errBoom := errors.New("boom")

	if err := value.WrapErr(nil, "ctx %s", "key"); err != nil {
		t.Errorf("WrapErr(nil) got %v, want nil", err)
	}

	err := value.WrapErr(errBoom, "db secret get %q", "key1")
	if err == nil || err.Error() != `db secret get "key1": boom` {
		t.Errorf("WrapErr got %v, want %q", err, `db secret get "key1": boom`)
	}
	if !errors.Is(err, errBoom) {
		t.Errorf("WrapErr errors.Is(errBoom) = false")
	}

	val, err2 := value.MapErr("data", nil, "ctx %s", "key")
	if val != "data" || err2 != nil {
		t.Errorf("MapErr(nil) got (%v, %v), want (data, nil)", val, err2)
	}

	val3, err3 := value.MapErr("data", errBoom, "db secret get %q", "key2")
	if val3 != "data" || err3 == nil || err3.Error() != `db secret get "key2": boom` {
		t.Errorf("MapErr(err) got (%v, %v)", val3, err3)
	}
	if !errors.Is(err3, errBoom) {
		t.Errorf("MapErr errors.Is(errBoom) = false")
	}
}

