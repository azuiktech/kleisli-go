package result

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

var errBoom = errors.New("boom")

func TestExpect(t *testing.T) {
	if got := OK(42).Expect("loading answer"); got != 42 {
		t.Errorf("Expect on OK = %d, want 42", got)
	}

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("Expect on Err did not panic")
		}
		err, ok := r.(error)
		if !ok {
			t.Fatalf("panic value = %v (%T), want error", r, r)
		}
		if !errors.Is(err, errBoom) {
			t.Errorf("panic error = %v, want wrapping %v", err, errBoom)
		}
		if got, want := err.Error(), "loading config: boom"; got != want {
			t.Errorf("panic message = %q, want %q", got, want)
		}
	}()
	Err[int](errBoom).Expect("loading config")
}

func TestFold(t *testing.T) {
	onOK := func(v int) string { return "ok" }
	onErr := func(err error) string { return "err:" + err.Error() }

	if got := OK(1).Fold(onOK, onErr); got != "ok" {
		t.Errorf("Fold on OK = %q, want %q", got, "ok")
	}
	if got := Err[int](errBoom).Fold(onOK, onErr); got != "err:boom" {
		t.Errorf("Fold on Err = %q, want %q", got, "err:boom")
	}
}

func TestRecover(t *testing.T) {
	tests := []struct {
		name    string
		start   Result[int]
		recover func(error) Result[int]
		wantOK  bool
		wantVal int
	}{
		{"success passes through unchanged", OK(1), func(error) Result[int] { return OK(99) }, true, 1},
		{"failure recovers to success", Err[int](errBoom), func(error) Result[int] { return OK(2) }, true, 2},
		{"failure recovers to another failure", Err[int](errBoom), func(error) Result[int] { return Err[int](errors.New("still broken")) }, false, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.start.Recover(tt.recover)
			if got.IsOK() != tt.wantOK {
				t.Errorf("IsOK() = %v, want %v", got.IsOK(), tt.wantOK)
			}
			if tt.wantOK && got.Val() != tt.wantVal {
				t.Errorf("Val() = %d, want %d", got.Val(), tt.wantVal)
			}
		})
	}
}

func TestZip2(t *testing.T) {
	tests := []struct {
		name    string
		a       Result[int]
		b       Result[string]
		wantOK  bool
		wantVal string
	}{
		{"both succeed", OK(2), OK("x"), true, "xx"},
		{"first fails", Err[int](errBoom), OK("x"), false, ""},
		{"second fails", OK(2), Err[string](errBoom), false, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repeat := func(n int, s string) string { return strings.Repeat(s, n) }
			got := Zip2(tt.a, tt.b, repeat)
			if got.IsOK() != tt.wantOK {
				t.Errorf("IsOK() = %v, want %v", got.IsOK(), tt.wantOK)
			}
			if tt.wantOK && got.Val() != tt.wantVal {
				t.Errorf("Val() = %q, want %q", got.Val(), tt.wantVal)
			}
		})
	}
}

func TestZip3(t *testing.T) {
	sum := func(a, b, c int) int { return a + b + c }

	tests := []struct {
		name    string
		a, b, c Result[int]
		wantOK  bool
		wantVal int
	}{
		{"all succeed", OK(1), OK(2), OK(3), true, 6},
		{"middle fails", OK(1), Err[int](errBoom), OK(3), false, 0},
		{"last fails", OK(1), OK(2), Err[int](errBoom), false, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Zip3(tt.a, tt.b, tt.c, sum)
			if got.IsOK() != tt.wantOK {
				t.Errorf("IsOK() = %v, want %v", got.IsOK(), tt.wantOK)
			}
			if tt.wantOK && got.Val() != tt.wantVal {
				t.Errorf("Val() = %d, want %d", got.Val(), tt.wantVal)
			}
		})
	}
}

func TestSequence(t *testing.T) {
	tests := []struct {
		name    string
		results []Result[int]
		wantOK  bool
		wantVal []int
	}{
		{"all succeed", []Result[int]{OK(1), OK(2), OK(3)}, true, []int{1, 2, 3}},
		{"empty slice succeeds with empty result", []Result[int]{}, true, []int{}},
		{"one failure fails the whole sequence", []Result[int]{OK(1), Err[int](errBoom), OK(3)}, false, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Sequence(tt.results)
			if got.IsOK() != tt.wantOK {
				t.Errorf("IsOK() = %v, want %v", got.IsOK(), tt.wantOK)
				return
			}
			if !tt.wantOK {
				return
			}
			gotVal := got.Val()
			if len(gotVal) != len(tt.wantVal) {
				t.Fatalf("Val() = %v, want %v", gotVal, tt.wantVal)
			}
			for i := range gotVal {
				if gotVal[i] != tt.wantVal[i] {
					t.Errorf("Val()[%d] = %d, want %d", i, gotVal[i], tt.wantVal[i])
				}
			}
		})
	}
}

func TestSuccessesAndFailures(t *testing.T) {
	results := []Result[int]{OK(1), Err[int](errBoom), OK(2), Err[int](errors.New("also broken"))}

	successes := Successes(results)
	if len(successes) != 2 || successes[0] != 1 || successes[1] != 2 {
		t.Errorf("Successes() = %v, want [1 2]", successes)
	}

	failures := Failures(results)
	if len(failures) != 2 || !errors.Is(failures[0], errBoom) {
		t.Errorf("Failures() = %v, want 2 errors starting with %v", failures, errBoom)
	}
}

func TestMarshalJSON_WireShape(t *testing.T) {
	if got, err := json.Marshal(OK(42)); err != nil || string(got) != `{"ok":42}` {
		t.Errorf("Marshal(OK(42)) = %s, %v, want {\"ok\":42}, nil", got, err)
	}
	if got, err := json.Marshal(Err[int](errBoom)); err != nil || string(got) != `{"err":"boom"}` {
		t.Errorf("Marshal(Err(boom)) = %s, %v, want {\"err\":\"boom\"}, nil", got, err)
	}
}

func TestJSON_RoundTrip_OK(t *testing.T) {
	data, err := json.Marshal(OK("hello"))
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	var got Result[string]
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if !got.IsOK() || got.Val() != "hello" {
		t.Errorf("round-tripped = IsOK=%v Val=%q, want IsOK=true Val=%q", got.IsOK(), got.Val(), "hello")
	}
}

func TestJSON_RoundTrip_Err(t *testing.T) {
	data, err := json.Marshal(Err[string](errBoom))
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	var got Result[string]
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if !got.IsErr() || got.Error().Error() != "boom" {
		t.Errorf("round-tripped = IsErr=%v Error=%v, want IsErr=true Error=boom", got.IsErr(), got.Error())
	}
}

// TestJSON_RoundTrip_NilPointerSuccess is the case the whole design exists
// for: a success value that is itself a nil pointer (e.g. "no match found")
// must stay distinguishable from a failed lookup after round-tripping.
func TestJSON_RoundTrip_NilPointerSuccess(t *testing.T) {
	var noMatch *int
	data, err := json.Marshal(OK(noMatch))
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	if string(data) != `{"ok":null}` {
		t.Errorf("Marshal(OK(nil *int)) = %s, want {\"ok\":null}", data)
	}

	var got Result[*int]
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if !got.IsOK() || got.Val() != nil {
		t.Errorf("round-tripped = IsOK=%v Val=%v, want IsOK=true Val=nil", got.IsOK(), got.Val())
	}
}

func TestUnmarshalJSON_MalformedInput(t *testing.T) {
	var got Result[int]
	if err := json.Unmarshal([]byte(`{}`), &got); err == nil {
		t.Error("Unmarshal({}) error = nil, want an error for missing ok/err key")
	}
}

func TestOr(t *testing.T) {
	ok1 := OK("primary")
	ok2 := OK("secondary")
	err1 := Err[string](errBoom)

	if got := ok1.Or(ok2); got.Val() != "primary" {
		t.Errorf("ok1.Or(ok2) got %v, want primary", got)
	}
	if got := err1.Or(ok2); got.Val() != "secondary" {
		t.Errorf("err1.Or(ok2) got %v, want secondary", got)
	}
}

func TestFlatten(t *testing.T) {
	nestedOK := OK(OK("inner"))
	if got := Flatten(nestedOK); !got.IsOK() || got.Val() != "inner" {
		t.Errorf("Flatten(OK(OK)) got %v, want OK(inner)", got)
	}

	nestedErrInner := OK(Err[string](errBoom))
	if got := Flatten(nestedErrInner); !got.IsErr() || !errors.Is(got.Error(), errBoom) {
		t.Errorf("Flatten(OK(Err)) got %v, want Err(boom)", got)
	}

	nestedErrOuter := Err[Result[string]](errBoom)
	if got := Flatten(nestedErrOuter); !got.IsErr() || !errors.Is(got.Error(), errBoom) {
		t.Errorf("Flatten(Err) got %v, want Err(boom)", got)
	}
}

func TestContains(t *testing.T) {
	ok := OK("hello")
	err := Err[string](errBoom)

	if !Contains(ok, "hello") {
		t.Errorf("Contains(OK(hello), hello) got false, want true")
	}
	if Contains(ok, "world") {
		t.Errorf("Contains(OK(hello), world) got true, want false")
	}
	if Contains(err, "hello") {
		t.Errorf("Contains(Err, hello) got true, want false")
	}
}

func TestMapErrfAndWrapErr(t *testing.T) {
	ok := OK("secret")
	if got := ok.MapErrf("db secret get %q", "my-key"); !got.IsOK() || got.Val() != "secret" {
		t.Errorf("OK.MapErrf got %v, want OK(secret)", got)
	}

	err := Err[string](errBoom)
	gotErr := err.MapErrf("db secret get %q", "my-key")
	if !gotErr.IsErr() {
		t.Fatalf("Err.MapErrf got OK, want Err")
	}
	if gotErr.Error().Error() != `db secret get "my-key": boom` {
		t.Errorf("MapErrf error message got %q, want %q", gotErr.Error().Error(), `db secret get "my-key": boom`)
	}
	if !errors.Is(gotErr.Error(), errBoom) {
		t.Errorf("MapErrf error wrapping chain broken, errors.Is(errBoom) = false")
	}

	// Testing with explicit %w
	gotErrExplicit := err.WrapErr("failed %q: %w", "my-key")
	if gotErrExplicit.Error().Error() != `failed "my-key": boom` {
		t.Errorf("WrapErr error message got %q, want %q", gotErrExplicit.Error().Error(), `failed "my-key": boom`)
	}
	if !errors.Is(gotErrExplicit.Error(), errBoom) {
		t.Errorf("WrapErr explicit %%w error wrapping chain broken")
	}

}


