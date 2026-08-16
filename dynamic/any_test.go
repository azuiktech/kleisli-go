package dynamic

import (
	"encoding/json"
	"strings"
	"testing"
)

type point struct {
	X, Y int
}

type label struct {
	Text string
}

func init() {
	Register[point]("dynamic_test.point")
	Register[label]("dynamic_test.label")
}

func TestRegister_PanicsOnDuplicateName(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("Register with a duplicate name did not panic")
		}
	}()
	Register[point]("dynamic_test.point")
}

func TestNew_PanicsOnUnregisteredType(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("New with an unregistered type did not panic")
		}
	}()
	type unregistered struct{}
	New(unregistered{})
}

func TestNew_NilValue_ReturnsZeroAny(t *testing.T) {
	d := New(nil)
	if d.Value() != nil {
		t.Errorf("Value() = %v, want nil", d.Value())
	}
}

func TestAny_MarshalUnmarshal_RoundTrips(t *testing.T) {
	original := New(point{X: 1, Y: 2})

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var decoded Any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	got, ok := decoded.Value().(point)
	if !ok {
		t.Fatalf("Value() = %#v (%T), want a point", decoded.Value(), decoded.Value())
	}
	if got != (point{X: 1, Y: 2}) {
		t.Errorf("Value() = %+v, want {1 2}", got)
	}
}

func TestAny_MarshalUnmarshal_DistinguishesRegisteredTypes(t *testing.T) {
	data, err := json.Marshal(New(label{Text: "hi"}))
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var decoded Any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if _, ok := decoded.Value().(point); ok {
		t.Fatal("decoded a label's JSON as a point")
	}
	got, ok := decoded.Value().(label)
	if !ok || got.Text != "hi" {
		t.Fatalf("Value() = %#v, want label{Text: \"hi\"}", decoded.Value())
	}
}

func TestAny_MarshalJSON_NilValue_WritesNull(t *testing.T) {
	data, err := json.Marshal(New(nil))
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if string(data) != "null" {
		t.Errorf("Marshal(New(nil)) = %s, want null", data)
	}
}

func TestAny_UnmarshalJSON_Null_ReturnsZeroAny(t *testing.T) {
	var d Any
	if err := json.Unmarshal([]byte("null"), &d); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if d.Value() != nil {
		t.Errorf("Value() = %v, want nil", d.Value())
	}
}

func TestAny_UnmarshalJSON_UnknownTypeName_ReturnsError(t *testing.T) {
	var d Any
	err := json.Unmarshal([]byte(`{"@type":"dynamic_test.never_registered","value":{}}`), &d)
	if err == nil {
		t.Fatal("expected an error unmarshaling an unregistered type name")
	}
	if !strings.Contains(err.Error(), "never_registered") {
		t.Errorf("error = %v, want it to name the unregistered type", err)
	}
}

func TestAs_CorrectType_ReturnsSome(t *testing.T) {
	d := New(point{X: 1, Y: 2})
	got := As[point](d)
	if got.IsNone() || got.MustGet() != (point{X: 1, Y: 2}) {
		t.Errorf("As[point] = %v, want Some({1 2})", got)
	}
}

func TestAs_WrongType_ReturnsNone(t *testing.T) {
	d := New(point{X: 1, Y: 2})
	if As[label](d).IsSome() {
		t.Error("As[label] on a point Any returned Some, want None")
	}
}

func TestAs_ZeroAny_ReturnsNone(t *testing.T) {
	if As[point](Any{}).IsSome() {
		t.Error("As[point] on zero Any returned Some, want None")
	}
}

func TestAny_EmbeddedInAStruct_RoundTrips(t *testing.T) {
	type wrapper struct {
		Local Any `json:"local"`
	}
	original := wrapper{Local: New(point{X: 3, Y: 4})}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var decoded wrapper
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	got, ok := decoded.Local.Value().(point)
	if !ok || got != (point{X: 3, Y: 4}) {
		t.Fatalf("Local.Value() = %#v, want point{3 4}", decoded.Local.Value())
	}
}
