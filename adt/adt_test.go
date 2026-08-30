package adt_test

import (
	"errors"
	"testing"

	"github.com/azuiktech/kleisli-go/adt"
)

var errBoom = errors.New("boom")
var errOther = errors.New("other")

// ── Unit ──────────────────────────────────────────────────────────────────────

func TestVoid_is_unit(t *testing.T) {
	var zero adt.Unit
	if adt.Void != zero {
		t.Fatal("Void should equal the zero Unit value")
	}
}

func TestVoid_in_result(t *testing.T) {
	r := adt.OK(adt.Void)
	if !r.IsOK() {
		t.Fatal("OK(Void) should be OK")
	}
}

// ── Result constructors ────────────────────────────────────────────────────────

func TestResult_OK_and_Err(t *testing.T) {
	r := adt.OK(42)
	if !r.IsOK() || r.IsErr() {
		t.Fatal("OK should be OK and not Err")
	}
	e := adt.Err[int](errBoom)
	if e.IsOK() || !e.IsErr() {
		t.Fatal("Err should be Err and not OK")
	}
}

func TestResult_From(t *testing.T) {
	r := adt.From(42, nil)
	if v := r.MustGet(); v != 42 {
		t.Fatalf("From OK: want 42, got %d", v)
	}
	r2 := adt.From(0, errBoom)
	if !r2.IsErr() {
		t.Fatal("From with error should be Err")
	}
}

func TestResult_MustGet_panics_on_err(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("MustGet on Err should panic")
		}
	}()
	adt.Err[int](errBoom).MustGet()
}

func TestResult_MustErr(t *testing.T) {
	if got := adt.Err[int](errBoom).MustErr(); got != errBoom {
		t.Errorf("MustErr = %v, want %v", got, errBoom)
	}
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("MustErr on OK should panic")
		}
	}()
	adt.OK(1).MustErr()
}

func TestResult_Expect(t *testing.T) {
	if got := adt.OK(42).Expect("msg"); got != 42 {
		t.Fatalf("Expect on OK = %d, want 42", got)
	}
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("Expect on Err should panic")
		}
		err := r.(error)
		if !errors.Is(err, errBoom) {
			t.Errorf("panic should wrap errBoom, got %v", err)
		}
	}()
	adt.Err[int](errBoom).Expect("loading")
}

func TestResult_OrElse(t *testing.T) {
	if got := adt.OK(1).OrElse(99); got != 1 {
		t.Fatalf("OrElse on OK: want 1, got %d", got)
	}
	if got := adt.Err[int](errBoom).OrElse(99); got != 99 {
		t.Fatalf("OrElse on Err: want 99, got %d", got)
	}
}

// ── Result combinators ────────────────────────────────────────────────────────

func TestResult_Map(t *testing.T) {
	r := adt.OK(3).Map(func(n int) string { return "x" })
	if r.MustGet() != "x" {
		t.Fatal("Map should transform value")
	}
	e := adt.Err[int](errBoom).Map(func(n int) string { return "x" })
	if !e.IsErr() {
		t.Fatal("Map on Err should propagate error")
	}
}

func TestResult_FlatMap(t *testing.T) {
	double := func(n int) adt.Result[int] { return adt.OK(n * 2) }
	if got := adt.OK(5).FlatMap(double).MustGet(); got != 10 {
		t.Fatalf("FlatMap: want 10, got %d", got)
	}
	if !adt.Err[int](errBoom).FlatMap(double).IsErr() {
		t.Fatal("FlatMap on Err should short-circuit")
	}
}

func TestResult_Then(t *testing.T) {
	r := adt.OK(7).Then(func(n int) (string, error) { return "ok", nil })
	if r.MustGet() != "ok" {
		t.Fatal("Then on OK with nil error should succeed")
	}
	r2 := adt.OK(7).Then(func(n int) (string, error) { return "", errBoom })
	if !r2.IsErr() {
		t.Fatal("Then with non-nil error should fail")
	}
}

func TestResult_Fold(t *testing.T) {
	got := adt.OK(10).Fold(func(n int) string { return "ok" }, func(e error) string { return "err" })
	if got != "ok" {
		t.Fatal("Fold on OK should call onOK")
	}
	got2 := adt.Err[int](errBoom).Fold(func(n int) string { return "ok" }, func(e error) string { return "err" })
	if got2 != "err" {
		t.Fatal("Fold on Err should call onErr")
	}
}

func TestResult_MapErr(t *testing.T) {
	r := adt.Err[int](errBoom).MapErr(func(e error) error { return errOther })
	if !errors.Is(r.MustErr(), errOther) {
		t.Fatal("MapErr should replace error")
	}
}

func TestResult_MapErrf(t *testing.T) {
	r := adt.Err[int](errBoom).MapErrf("loading user %s", "abc")
	if !errors.Is(r.MustErr(), errBoom) {
		t.Fatal("MapErrf should wrap errBoom")
	}
}

func TestResult_Recover(t *testing.T) {
	r := adt.Err[int](errBoom).Recover(func(e error) adt.Result[int] { return adt.OK(99) })
	if r.MustGet() != 99 {
		t.Fatal("Recover should return fallback")
	}
	r2 := adt.OK(1).Recover(func(e error) adt.Result[int] { return adt.OK(99) })
	if r2.MustGet() != 1 {
		t.Fatal("Recover on OK should pass through")
	}
}

func TestResult_Tap(t *testing.T) {
	called := false
	adt.OK(1).Tap(func(n int) { called = true })
	if !called {
		t.Fatal("Tap on OK should call fn")
	}
	called = false
	adt.Err[int](errBoom).Tap(func(n int) { called = true })
	if called {
		t.Fatal("Tap on Err should not call fn")
	}
}

func TestResult_TapErr(t *testing.T) {
	called := false
	adt.Err[int](errBoom).TapErr(func(e error) { called = true })
	if !called {
		t.Fatal("TapErr on Err should call fn")
	}
	called = false
	adt.OK(1).TapErr(func(e error) { called = true })
	if called {
		t.Fatal("TapErr on OK should not call fn")
	}
}

// ── Result.ToOption ────────────────────────────────────────────────────────────

func TestResult_ToOption_ok_becomes_some(t *testing.T) {
	o := adt.OK(42).ToOption()
	if !o.IsSome() {
		t.Fatal("OK.ToOption should be Some")
	}
	if o.MustGet() != 42 {
		t.Fatalf("ToOption value: want 42, got %d", o.MustGet())
	}
}

func TestResult_ToOption_err_becomes_none(t *testing.T) {
	o := adt.Err[int](errBoom).ToOption()
	if !o.IsNone() {
		t.Fatal("Err.ToOption should be None")
	}
}

func TestResult_ToOption_nil_ptr_becomes_none(t *testing.T) {
	var p *int
	o := adt.OK(p).ToOption()
	if !o.IsNone() {
		t.Fatal("OK(nil pointer).ToOption should be None (nil is not a valid Some)")
	}
}

// ── Results namespace ─────────────────────────────────────────────────────────

func TestResults_Zip2(t *testing.T) {
	r := adt.Results.Zip2(adt.OK(1), adt.OK(2), func(a, b int) int { return a + b })
	if r.MustGet() != 3 {
		t.Fatalf("Zip2: want 3, got %d", r.MustGet())
	}
	r2 := adt.Results.Zip2(adt.Err[int](errBoom), adt.OK(2), func(a, b int) int { return a + b })
	if !r2.IsErr() {
		t.Fatal("Zip2 with first Err should fail")
	}
}

func TestResults_Zip3(t *testing.T) {
	r := adt.Results.Zip3(adt.OK(1), adt.OK(2), adt.OK(3), func(a, b, c int) int { return a + b + c })
	if r.MustGet() != 6 {
		t.Fatalf("Zip3: want 6, got %d", r.MustGet())
	}
}

func TestResults_Flatten(t *testing.T) {
	inner := adt.OK(42)
	outer := adt.OK(inner)
	if got := adt.Results.Flatten(outer).MustGet(); got != 42 {
		t.Fatalf("Flatten: want 42, got %d", got)
	}
	errOuter := adt.Err[adt.Result[int]](errBoom)
	if !adt.Results.Flatten(errOuter).IsErr() {
		t.Fatal("Flatten Err outer should fail")
	}
}

func TestResults_Contains(t *testing.T) {
	if !adt.Results.Contains(adt.OK(5), 5) {
		t.Fatal("Contains should be true for matching value")
	}
	if adt.Results.Contains(adt.OK(5), 6) {
		t.Fatal("Contains should be false for non-matching value")
	}
	if adt.Results.Contains(adt.Err[int](errBoom), 5) {
		t.Fatal("Contains on Err should be false")
	}
}

func TestResults_Sequence(t *testing.T) {
	rs := []adt.Result[int]{adt.OK(1), adt.OK(2), adt.OK(3)}
	got := adt.Results.Sequence(rs).MustGet()
	if len(got) != 3 || got[0] != 1 || got[2] != 3 {
		t.Fatalf("Sequence: unexpected %v", got)
	}
	rs2 := []adt.Result[int]{adt.OK(1), adt.Err[int](errBoom), adt.OK(3)}
	if !adt.Results.Sequence(rs2).IsErr() {
		t.Fatal("Sequence with Err should fail")
	}
}

func TestSuccesses_and_Failures(t *testing.T) {
	rs := []adt.Result[int]{adt.OK(1), adt.Err[int](errBoom), adt.OK(3)}
	if got := adt.Successes(rs); len(got) != 2 || got[0] != 1 || got[1] != 3 {
		t.Fatalf("Successes: want [1 3], got %v", got)
	}
	if got := adt.Failures(rs); len(got) != 1 || !errors.Is(got[0], errBoom) {
		t.Fatalf("Failures: want [errBoom], got %v", got)
	}
}

// ── Option constructors ───────────────────────────────────────────────────────

func TestOption_Some_and_None(t *testing.T) {
	o := adt.Some(42)
	if !o.IsSome() || o.IsNone() {
		t.Fatal("Some should be IsSome and not IsNone")
	}
	n := adt.None[int]()
	if n.IsSome() || !n.IsNone() {
		t.Fatal("None should be IsNone")
	}
}

func TestOption_Some_allows_nil(t *testing.T) {
	var p *int
	o := adt.Some(p)
	if !o.IsSome() {
		t.Fatal("Some(nil pointer) should be IsSome — ok discriminant distinguishes it from None")
	}
	if o.MustGet() != nil {
		t.Fatal("Some(nil pointer).MustGet() should return nil")
	}
}

func TestOption_Opt(t *testing.T) {
	var p *int
	if !adt.Opt(p).IsNone() {
		t.Fatal("Opt(nil) should be None")
	}
	v := 42
	o := adt.Opt(&v)
	if !o.IsSome() || *o.MustGet() != 42 {
		t.Fatal("Opt(non-nil) should be Some")
	}
}

func TestOption_FromOk(t *testing.T) {
	if !adt.FromOk(1, true).IsSome() {
		t.Fatal("FromOk(_, true) should be Some")
	}
	if !adt.FromOk(0, false).IsNone() {
		t.Fatal("FromOk(_, false) should be None")
	}
}

func TestOption_OptNonZero(t *testing.T) {
	if !adt.OptNonZero(5).IsSome() {
		t.Fatal("OptNonZero(non-zero) should be Some")
	}
	if !adt.OptNonZero(0).IsNone() {
		t.Fatal("OptNonZero(0) should be None")
	}
}

func TestOption_FromSlice(t *testing.T) {
	s := []int{10, 20, 30}
	if got := adt.FromSlice(s, 1).MustGet(); got != 20 {
		t.Fatalf("FromSlice: want 20, got %d", got)
	}
	if !adt.FromSlice(s, 5).IsNone() {
		t.Fatal("FromSlice out of bounds should be None")
	}
}

func TestOption_FromResult(t *testing.T) {
	o := adt.FromResult(adt.OK(7))
	if !o.IsSome() || o.MustGet() != 7 {
		t.Fatal("FromResult on OK should be Some")
	}
	o2 := adt.FromResult(adt.Err[int](errBoom))
	if !o2.IsNone() {
		t.Fatal("FromResult on Err should be None")
	}
}

// ── Option combinators ────────────────────────────────────────────────────────

func TestOption_Map(t *testing.T) {
	o := adt.Some(3).Map(func(n int) string { return "x" })
	if o.MustGet() != "x" {
		t.Fatal("Map should transform value")
	}
	if !adt.None[int]().Map(func(n int) string { return "x" }).IsNone() {
		t.Fatal("Map on None should propagate absence")
	}
}

func TestOption_FlatMap(t *testing.T) {
	double := func(n int) adt.Option[int] { return adt.Some(n * 2) }
	if got := adt.Some(4).FlatMap(double).MustGet(); got != 8 {
		t.Fatalf("FlatMap: want 8, got %d", got)
	}
	if !adt.None[int]().FlatMap(double).IsNone() {
		t.Fatal("FlatMap on None should short-circuit")
	}
}

func TestOption_Fold(t *testing.T) {
	got := adt.Some(1).Fold(func(n int) string { return "some" }, func() string { return "none" })
	if got != "some" {
		t.Fatal("Fold on Some should call onSome")
	}
	got2 := adt.None[int]().Fold(func(n int) string { return "some" }, func() string { return "none" })
	if got2 != "none" {
		t.Fatal("Fold on None should call onNone")
	}
}

func TestOption_Filter(t *testing.T) {
	o := adt.Some(5).Filter(func(n int) bool { return n > 3 })
	if !o.IsSome() {
		t.Fatal("Filter passing should keep Some")
	}
	o2 := adt.Some(1).Filter(func(n int) bool { return n > 3 })
	if !o2.IsNone() {
		t.Fatal("Filter failing should become None")
	}
}

func TestOption_ToResult(t *testing.T) {
	r := adt.Some(7).ToResult(errBoom)
	if !r.IsOK() || r.MustGet() != 7 {
		t.Fatal("ToResult on Some should be OK")
	}
	r2 := adt.None[int]().ToResult(errBoom)
	if !r2.IsErr() || !errors.Is(r2.MustErr(), errBoom) {
		t.Fatal("ToResult on None should be Err with provided error")
	}
}

func TestOption_ToResultGet(t *testing.T) {
	r := adt.None[int]().ToResultGet(func() error { return errOther })
	if !r.IsErr() || !errors.Is(r.MustErr(), errOther) {
		t.Fatal("ToResultGet on None should call fn for error")
	}
}

func TestSomes(t *testing.T) {
	os := []adt.Option[int]{adt.Some(1), adt.None[int](), adt.Some(3)}
	if got := adt.Somes(os); len(got) != 2 || got[0] != 1 || got[1] != 3 {
		t.Fatalf("Somes: want [1 3], got %v", got)
	}
}

// ── Options namespace ─────────────────────────────────────────────────────────

func TestOptions_Zip2(t *testing.T) {
	r := adt.Options.Zip2(adt.Some(1), adt.Some(2), func(a, b int) int { return a + b })
	if r.MustGet() != 3 {
		t.Fatalf("Options.Zip2: want 3, got %d", r.MustGet())
	}
	r2 := adt.Options.Zip2(adt.None[int](), adt.Some(2), func(a, b int) int { return a + b })
	if !r2.IsNone() {
		t.Fatal("Options.Zip2 with None should fail")
	}
}

func TestOptions_Flatten(t *testing.T) {
	inner := adt.Some(42)
	outer := adt.Some(inner)
	if got := adt.Options.Flatten(outer).MustGet(); got != 42 {
		t.Fatalf("Options.Flatten: want 42, got %d", got)
	}
	if !adt.Options.Flatten(adt.None[adt.Option[int]]()).IsNone() {
		t.Fatal("Flatten None outer should be None")
	}
}

func TestOptions_Contains(t *testing.T) {
	if !adt.Options.Contains(adt.Some(5), 5) {
		t.Fatal("Contains should be true for matching value")
	}
	if adt.Options.Contains(adt.None[int](), 5) {
		t.Fatal("Contains on None should be false")
	}
}

func TestOptions_Sequence(t *testing.T) {
	os := []adt.Option[int]{adt.Some(1), adt.Some(2)}
	got := adt.Options.Sequence(os).MustGet()
	if len(got) != 2 || got[0] != 1 || got[1] != 2 {
		t.Fatalf("Options.Sequence: unexpected %v", got)
	}
	os2 := []adt.Option[int]{adt.Some(1), adt.None[int]()}
	if !adt.Options.Sequence(os2).IsNone() {
		t.Fatal("Options.Sequence with None should fail")
	}
}

// ── Lazy ──────────────────────────────────────────────────────────────────────

func TestDefer_evaluates_once(t *testing.T) {
	calls := 0
	l := adt.Defer(func() int { calls++; return 42 })
	_ = l.Get()
	_ = l.Get()
	if calls != 1 {
		t.Fatalf("Defer fn should be called once; called %d times", calls)
	}
}

func TestDefer_zero_value_safe(t *testing.T) {
	var l adt.Lazy[int]
	if got := l.Get(); got != 0 {
		t.Fatalf("zero-value Lazy.Get() should return zero, got %d", got)
	}
}

func TestLazy_Map(t *testing.T) {
	l := adt.Defer(func() int { return 5 }).Map(func(n int) string { return "five" })
	if got := l.Get(); got != "five" {
		t.Fatalf("Lazy.Map: want 'five', got %q", got)
	}
}

func TestLazy_FlatMap(t *testing.T) {
	l := adt.Defer(func() int { return 3 }).FlatMap(func(n int) adt.Lazy[int] {
		return adt.Defer(func() int { return n * 10 })
	})
	if got := l.Get(); got != 30 {
		t.Fatalf("Lazy.FlatMap: want 30, got %d", got)
	}
}

func TestLazy_ToOption(t *testing.T) {
	v := 99
	l := adt.Defer(func() *int { return &v })
	if got := l.ToOption().MustGet(); *got != 99 {
		t.Fatalf("Lazy.ToOption: want 99, got %d", *got)
	}
	l2 := adt.Defer(func() *int { return nil })
	if !l2.ToOption().IsNone() {
		t.Fatal("Lazy.ToOption with nil should be None")
	}
}

func TestLazy_ToResult(t *testing.T) {
	v := 7
	l := adt.Defer(func() *int { return &v })
	if got := l.ToResult(errBoom).MustGet(); *got != 7 {
		t.Fatalf("Lazy.ToResult: want 7, got %d", *got)
	}
	l2 := adt.Defer(func() *int { return nil })
	if !l2.ToResult(errBoom).IsErr() {
		t.Fatal("Lazy.ToResult with nil should be Err")
	}
}

func TestDeferErr(t *testing.T) {
	l := adt.DeferErr(func() (int, error) { return 42, nil })
	if got := l.Get().MustGet(); got != 42 {
		t.Fatalf("DeferErr OK: want 42, got %d", got)
	}
	l2 := adt.DeferErr(func() (int, error) { return 0, errBoom })
	if !l2.Get().IsErr() {
		t.Fatal("DeferErr with error should be Err")
	}
}

func TestMemoize(t *testing.T) {
	calls := 0
	fn := adt.Memoize(func(k int) int { calls++; return k * 2 })
	_ = fn(3)
	_ = fn(3)
	_ = fn(4)
	if calls != 2 {
		t.Fatalf("Memoize: want 2 calls, got %d", calls)
	}
	if got := fn(3); got != 6 {
		t.Fatalf("Memoize fn(3): want 6, got %d", got)
	}
}

// ── Dynamic / Any ─────────────────────────────────────────────────────────────

type dynTestVal struct{ N int }

func init() {
	adt.Register[dynTestVal]("adt_test.dynTestVal")
}

func TestDyn_and_As(t *testing.T) {
	a := adt.Dyn(dynTestVal{N: 7})
	got := adt.As[dynTestVal](a)
	if !got.IsSome() || got.MustGet().N != 7 {
		t.Fatalf("As: want N=7, got %v", got)
	}
}

func TestDyn_unregistered_panics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("Dyn with unregistered type should panic")
		}
	}()
	type unregistered struct{}
	adt.Dyn(unregistered{})
}

func TestDyn_nil_returns_zero(t *testing.T) {
	a := adt.Dyn(nil)
	if a.Value() != nil {
		t.Fatal("Dyn(nil) should return zero Any with nil value")
	}
}

func TestAs_wrong_type_is_none(t *testing.T) {
	a := adt.Dyn(dynTestVal{N: 1})
	got := adt.As[int](a)
	if !got.IsNone() {
		t.Fatal("As with wrong type should be None")
	}
}

// ── Symmetric conversion round-trips ─────────────────────────────────────────

func TestResult_Option_roundtrip(t *testing.T) {
	// Result → Option → Result
	r1 := adt.OK(42)
	o := r1.ToOption()
	r2 := o.ToResult(errBoom)
	if !r2.IsOK() || r2.MustGet() != 42 {
		t.Fatal("Result→Option→Result round-trip failed")
	}

	// Option → Result → Option
	o1 := adt.Some("hello")
	r := o1.ToResult(errBoom)
	o2 := r.ToOption()
	if !o2.IsSome() || o2.MustGet() != "hello" {
		t.Fatal("Option→Result→Option round-trip failed")
	}
}
