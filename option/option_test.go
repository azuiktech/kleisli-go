package option

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestSome_PanicsOnNil(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("Some(nil *int) did not panic")
		}
	}()
	var p *int
	Some(p)
}

func TestFrom(t *testing.T) {
	var nilPtr *int
	if got := From(nilPtr); got.IsSome() {
		t.Errorf("From(nil) = Some(%v), want None", got.MustGet())
	}

	v := 42
	got := From(&v)
	if !got.IsSome() || *got.MustGet() != 42 {
		t.Errorf("From(&42) = %v, want Some(42)", got)
	}
}

func TestExpect(t *testing.T) {
	if got := Some(42).Expect("loading answer"); got != 42 {
		t.Errorf("Expect on Some = %d, want 42", got)
	}

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("Expect on None did not panic")
		}
		msg, ok := r.(string)
		if !ok {
			t.Fatalf("panic value = %v (%T), want string", r, r)
		}
		if want := "option: loading config"; msg != want {
			t.Errorf("panic message = %q, want %q", msg, want)
		}
	}()
	None[int]().Expect("loading config")
}

func TestFold(t *testing.T) {
	onSome := func(v int) string { return "some" }
	onNone := func() string { return "none" }

	if got := Some(1).Fold(onSome, onNone); got != "some" {
		t.Errorf("Fold on Some = %q, want %q", got, "some")
	}
	if got := None[int]().Fold(onSome, onNone); got != "none" {
		t.Errorf("Fold on None = %q, want %q", got, "none")
	}
}

func TestFilter(t *testing.T) {
	nonEmpty := func(s string) bool { return s != "" }

	tests := []struct {
		name     string
		start    Option[string]
		wantSome bool
		wantVal  string
	}{
		{"passes the predicate", Some("hi"), true, "hi"},
		{"fails the predicate", Some(" "), false, ""},
		{"already none", None[string](), false, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.start.Filter(func(s string) bool { return nonEmpty(strings.TrimSpace(s)) })
			if got.IsSome() != tt.wantSome {
				t.Errorf("IsSome() = %v, want %v", got.IsSome(), tt.wantSome)
			}
			if tt.wantSome && got.MustGet() != tt.wantVal {
				t.Errorf("MustGet() = %q, want %q", got.MustGet(), tt.wantVal)
			}
		})
	}
}

func TestZip2(t *testing.T) {
	tests := []struct {
		name     string
		a        Option[int]
		b        Option[string]
		wantSome bool
		wantVal  string
	}{
		{"both present", Some(2), Some("x"), true, "xx"},
		{"first absent", None[int](), Some("x"), false, ""},
		{"second absent", Some(2), None[string](), false, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repeat := func(n int, s string) string { return strings.Repeat(s, n) }
			got := Zip2(tt.a, tt.b, repeat)
			if got.IsSome() != tt.wantSome {
				t.Errorf("IsSome() = %v, want %v", got.IsSome(), tt.wantSome)
			}
			if tt.wantSome && got.MustGet() != tt.wantVal {
				t.Errorf("MustGet() = %q, want %q", got.MustGet(), tt.wantVal)
			}
		})
	}
}

func TestZip3(t *testing.T) {
	sum := func(a, b, c int) int { return a + b + c }

	tests := []struct {
		name     string
		a, b, c  Option[int]
		wantSome bool
		wantVal  int
	}{
		{"all present", Some(1), Some(2), Some(3), true, 6},
		{"middle absent", Some(1), None[int](), Some(3), false, 0},
		{"last absent", Some(1), Some(2), None[int](), false, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Zip3(tt.a, tt.b, tt.c, sum)
			if got.IsSome() != tt.wantSome {
				t.Errorf("IsSome() = %v, want %v", got.IsSome(), tt.wantSome)
			}
			if tt.wantSome && got.MustGet() != tt.wantVal {
				t.Errorf("MustGet() = %d, want %d", got.MustGet(), tt.wantVal)
			}
		})
	}
}

func TestSequence(t *testing.T) {
	tests := []struct {
		name     string
		options  []Option[int]
		wantSome bool
		wantVal  []int
	}{
		{"all present", []Option[int]{Some(1), Some(2), Some(3)}, true, []int{1, 2, 3}},
		{"empty slice succeeds with empty result", []Option[int]{}, true, []int{}},
		{"one absence fails the whole sequence", []Option[int]{Some(1), None[int](), Some(3)}, false, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Sequence(tt.options)
			if got.IsSome() != tt.wantSome {
				t.Errorf("IsSome() = %v, want %v", got.IsSome(), tt.wantSome)
				return
			}
			if !tt.wantSome {
				return
			}
			gotVal := got.MustGet()
			if len(gotVal) != len(tt.wantVal) {
				t.Fatalf("MustGet() = %v, want %v", gotVal, tt.wantVal)
			}
			for i := range gotVal {
				if gotVal[i] != tt.wantVal[i] {
					t.Errorf("MustGet()[%d] = %d, want %d", i, gotVal[i], tt.wantVal[i])
				}
			}
		})
	}
}

func TestSomes(t *testing.T) {
	options := []Option[int]{Some(1), None[int](), Some(2), None[int]()}

	somes := Somes(options)
	if len(somes) != 2 || somes[0] != 1 || somes[1] != 2 {
		t.Errorf("Somes() = %v, want [1 2]", somes)
	}
}

func TestToPtr(t *testing.T) {
	ptr := Some(42).ToPtr()
	if ptr == nil || *ptr != 42 {
		t.Errorf("Some(42).ToPtr() = %v, want pointer to 42", ptr)
	}
	if None[int]().ToPtr() != nil {
		t.Error("None[int]().ToPtr() != nil, want nil")
	}
}

func TestMarshalJSON_ValueOrNull(t *testing.T) {
	if got, err := json.Marshal(Some(42)); err != nil || string(got) != `42` {
		t.Errorf("Marshal(Some(42)) = %s, %v, want 42, nil", got, err)
	}
	if got, err := json.Marshal(None[int]()); err != nil || string(got) != `null` {
		t.Errorf("Marshal(None) = %s, %v, want null, nil", got, err)
	}
}

func TestJSON_RoundTrip_Some(t *testing.T) {
	data, err := json.Marshal(Some("hello"))
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	var got Option[string]
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if !got.IsSome() || got.MustGet() != "hello" {
		t.Errorf("round-tripped = IsSome=%v MustGet=%q, want IsSome=true MustGet=%q", got.IsSome(), got.MustGet(), "hello")
	}
}

func TestJSON_RoundTrip_None(t *testing.T) {
	data, err := json.Marshal(None[string]())
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	var got Option[string]
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if !got.IsNone() {
		t.Errorf("round-tripped = IsNone=%v, want true", got.IsNone())
	}
}

func TestUnmarshalJSON_ExplicitNull(t *testing.T) {
	var got Option[*int]
	if err := json.Unmarshal([]byte(`null`), &got); err != nil {
		t.Fatalf("Unmarshal(null) error = %v", err)
	}
	if !got.IsNone() {
		t.Errorf("Unmarshal(null) = %v, want None", got)
	}
}
