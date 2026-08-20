package fn_test

import (
	"errors"
	"testing"

	"github.com/azuiktech/kleisli-go/fn"
)

var errBoom = errors.New("boom")

// ── Value helpers ─────────────────────────────────────────────────────────────

func TestMust_ok(t *testing.T) {
	if got := fn.Must(42, nil); got != 42 {
		t.Fatalf("Must: want 42, got %d", got)
	}
}

func TestMust_panics_on_err(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("Must with error should panic")
		}
	}()
	fn.Must(0, errBoom)
}

func TestWrapErr_nil(t *testing.T) {
	if fn.WrapErr(nil, "msg") != nil {
		t.Fatal("WrapErr(nil) should return nil")
	}
}

func TestWrapErr_appends_w(t *testing.T) {
	err := fn.WrapErr(errBoom, "loading")
	if !errors.Is(err, errBoom) {
		t.Fatal("WrapErr should preserve error chain")
	}
}

func TestMapErr(t *testing.T) {
	v, err := fn.MapErr(42, nil, "ctx")
	if v != 42 || err != nil {
		t.Fatal("MapErr with nil error should pass through")
	}
	_, err2 := fn.MapErr(0, errBoom, "loading")
	if !errors.Is(err2, errBoom) {
		t.Fatal("MapErr should wrap error")
	}
}

func TestFallback(t *testing.T) {
	if fn.Fallback(1, nil, 99) != 1 {
		t.Fatal("Fallback without error should return val")
	}
	if fn.Fallback(0, errBoom, 99) != 99 {
		t.Fatal("Fallback with error should return fallback")
	}
}

func TestCond(t *testing.T) {
	if fn.Cond(true, "a", "b") != "a" {
		t.Fatal("Cond(true) should return ifTrue")
	}
	if fn.Cond(false, "a", "b") != "b" {
		t.Fatal("Cond(false) should return ifFalse")
	}
}

func TestCondGet(t *testing.T) {
	calls := 0
	fn.CondGet(true, func() int { calls++; return 1 }, func() int { calls++; return 2 })
	if calls != 1 {
		t.Fatalf("CondGet(true) should call only ifTrue, called %d branches", calls)
	}
}

func TestPtr(t *testing.T) {
	p := fn.Ptr(42)
	if *p != 42 {
		t.Fatal("Ptr should return pointer to value")
	}
}

func TestDeref(t *testing.T) {
	v := 7
	if fn.Deref(&v, 99) != 7 {
		t.Fatal("Deref non-nil should return *ptr")
	}
	if fn.Deref[int](nil, 99) != 99 {
		t.Fatal("Deref nil should return fallback")
	}
}

func TestDerefZero(t *testing.T) {
	if fn.DerefZero[int](nil) != 0 {
		t.Fatal("DerefZero nil should return zero")
	}
}

func TestTap(t *testing.T) {
	called := false
	got := fn.Tap(42, func(n int) { called = true })
	if !called || got != 42 {
		t.Fatal("Tap should call side effect and return val")
	}
}

func TestPipe(t *testing.T) {
	if got := fn.Pipe(5, func(n int) string { return "five" }); got != "five" {
		t.Fatalf("Pipe: want 'five', got %q", got)
	}
}

func TestZero(t *testing.T) {
	if fn.Zero[int]() != 0 {
		t.Fatal("Zero[int] should return 0")
	}
}

func TestIsZero(t *testing.T) {
	if !fn.IsZero(0) {
		t.Fatal("IsZero(0) should be true")
	}
	if fn.IsZero(1) {
		t.Fatal("IsZero(1) should be false")
	}
}

func TestClamp(t *testing.T) {
	if fn.Clamp(5, 1, 10) != 5 {
		t.Fatal("Clamp within range should return v")
	}
	if fn.Clamp(-5, 1, 10) != 1 {
		t.Fatal("Clamp below lo should return lo")
	}
	if fn.Clamp(15, 1, 10) != 10 {
		t.Fatal("Clamp above hi should return hi")
	}
}

func TestCoalesce(t *testing.T) {
	if fn.Coalesce(0, 0, 3, 4) != 3 {
		t.Fatalf("Coalesce: want 3, got %d", fn.Coalesce(0, 0, 3, 4))
	}
	if fn.Coalesce[int]() != 0 {
		t.Fatal("Coalesce with no args should return zero")
	}
}

// ── Function composition ──────────────────────────────────────────────────────

func TestFn_Then(t *testing.T) {
	double := fn.Fn[int, int](func(n int) int { return n * 2 })
	toString := fn.Fn[int, string](func(n int) string { return "x" })
	composed := double.Then(toString)
	if got := composed(3); got != "x" {
		t.Fatalf("Then: want 'x', got %q", got)
	}
}

func TestFn2_Fork(t *testing.T) {
	add := fn.Fn2[int, int, int](func(a, b int) int { return a + b })
	double := fn.Fn[int, int](func(n int) int { return n * 2 })
	addOne := fn.Fn[int, int](func(n int) int { return n + 1 })
	composed := add.Fork(double, addOne)
	// composed(3) = double(3) + addOne(3) = 6 + 4 = 10
	if got := composed(3); got != 10 {
		t.Fatalf("Fork: want 10, got %d", got)
	}
}

func TestIdentity(t *testing.T) {
	if fn.Identity(42) != 42 {
		t.Fatal("Identity should return its argument")
	}
}

// ── Memoization ───────────────────────────────────────────────────────────────

func TestMemoize(t *testing.T) {
	calls := 0
	f := fn.Memoize(func(k int) int { calls++; return k * 2 })
	_ = f(3)
	_ = f(3)
	_ = f(4)
	if calls != 2 {
		t.Fatalf("Memoize: want 2 calls, got %d", calls)
	}
	if f(3) != 6 {
		t.Fatal("Memoize f(3) should return 6")
	}
}

// ── FindRotated / After / Before ──────────────────────────────────────────────

func TestFindRotated_found_in_first(t *testing.T) {
	first := []int{3, 5, 7}
	second := []int{1, 2}
	n := fn.FindRotated(first, second, 5, func(a, b int) int { return a - b })
	if n != 1 {
		t.Fatalf("FindRotated: want 1, got %d", n)
	}
}

func TestFindRotated_found_in_second(t *testing.T) {
	first := []int{5, 7, 9}
	second := []int{1, 3}
	n := fn.FindRotated(first, second, 3, func(a, b int) int { return a - b })
	if n != 4 {
		t.Fatalf("FindRotated: want 4 (len(first)+1), got %d", n)
	}
}

func TestFindRotated_not_found(t *testing.T) {
	first := []int{5, 7}
	second := []int{1, 3}
	n := fn.FindRotated(first, second, 99, func(a, b int) int { return a - b })
	if n != 4 {
		t.Fatalf("FindRotated not found: want 4, got %d", n)
	}
}

func TestAfter(t *testing.T) {
	first := []int{1, 2, 3}
	second := []int{4, 5}
	a, b := fn.After(first, second, 1)
	if len(a) != 1 || a[0] != 3 || len(b) != 2 {
		t.Fatalf("After in first: unexpected a=%v b=%v", a, b)
	}
}

func TestBefore(t *testing.T) {
	first := []int{1, 2, 3}
	second := []int{4, 5}
	a, b := fn.Before(first, second, 2)
	if len(a) != 2 || a[0] != 1 || b != nil {
		t.Fatalf("Before in first: unexpected a=%v b=%v", a, b)
	}
}
