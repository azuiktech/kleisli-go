package adt_test

import (
	"encoding/json"
	"testing"

	"github.com/azuiktech/kleisli-go/adt"
)

func TestPairOf_and_accessors(t *testing.T) {
	p := adt.PairOf("hello", 42)
	if got := p.First(); got != "hello" {
		t.Fatalf("p.First() = %q, want %q", got, "hello")
	}
	if got := p.Second(); got != 42 {
		t.Fatalf("p.Second() = %d, want 42", got)
	}
}

func TestPair_Swap(t *testing.T) {
	p := adt.PairOf(1, "one")
	swapped := p.Swap()
	if swapped.First() != "one" || swapped.Second() != 1 {
		t.Fatalf("Swap() = (%v, %v), want (one, 1)", swapped.First(), swapped.Second())
	}
}

func TestPair_Unpack(t *testing.T) {
	p := adt.PairOf("foo", true)
	a, b := p.Unpack()
	if a != "foo" || !b {
		t.Fatalf("Unpack() = (%v, %v), want (foo, true)", a, b)
	}
}

func TestPair_JSON_roundtrip(t *testing.T) {
	p := adt.PairOf("alpha", 123)
	data, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var decoded adt.Pair[string, int]
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if decoded.First() != "alpha" || decoded.Second() != 123 {
		t.Fatalf("decoded = (%v, %v), want (alpha, 123)", decoded.First(), decoded.Second())
	}
}

func TestPair_JSON_invalid(t *testing.T) {
	var p adt.Pair[string, int]
	if err := json.Unmarshal([]byte("invalid json"), &p); err == nil {
		t.Fatal("Unmarshal invalid json should return error")
	}
}

func TestPair_Equality(t *testing.T) {
	p1 := adt.PairOf("a", 1)
	p2 := adt.PairOf("a", 1)
	p3 := adt.PairOf("a", 2)
	if p1 != p2 {
		t.Fatal("identical Pairs should be equal")
	}
	if p1 == p3 {
		t.Fatal("different Pairs should not be equal")
	}
}

func TestTripleOf_and_accessors(t *testing.T) {
	tr := adt.TripleOf("first", 2, 3.14)
	if got := tr.First(); got != "first" {
		t.Fatalf("tr.First() = %q, want %q", got, "first")
	}
	if got := tr.Second(); got != 2 {
		t.Fatalf("tr.Second() = %d, want 2", got)
	}
	if got := tr.Third(); got != 3.14 {
		t.Fatalf("tr.Third() = %v, want 3.14", got)
	}
}

func TestTriple_Unpack(t *testing.T) {
	tr := adt.TripleOf(10, 20, 30)
	a, b, c := tr.Unpack()
	if a != 10 || b != 20 || c != 30 {
		t.Fatalf("Unpack() = (%v, %v, %v), want (10, 20, 30)", a, b, c)
	}
}

func TestTriple_JSON_roundtrip(t *testing.T) {
	tr := adt.TripleOf("x", 42, true)
	data, err := json.Marshal(tr)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var decoded adt.Triple[string, int, bool]
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if decoded.First() != "x" || decoded.Second() != 42 || !decoded.Third() {
		t.Fatalf("decoded = (%v, %v, %v), want (x, 42, true)", decoded.First(), decoded.Second(), decoded.Third())
	}
}

func TestTriple_JSON_invalid(t *testing.T) {
	var tr adt.Triple[string, int, bool]
	if err := json.Unmarshal([]byte("invalid json"), &tr); err == nil {
		t.Fatal("Unmarshal invalid json should return error")
	}
}

func TestTriple_Equality(t *testing.T) {
	tr1 := adt.TripleOf("a", 1, true)
	tr2 := adt.TripleOf("a", 1, true)
	tr3 := adt.TripleOf("a", 1, false)
	if tr1 != tr2 {
		t.Fatal("identical Triples should be equal")
	}
	if tr1 == tr3 {
		t.Fatal("different Triples should not be equal")
	}
}
