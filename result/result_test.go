package result

import (
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
