package ds_test

import (
	"slices"
	"testing"

	"github.com/azuiktech/kleisli-go/ds"
)

func TestRing_PushPop(t *testing.T) {
	r := ds.Ring[int](3)
	r.Push(1)
	r.Push(2)
	r.Push(3)

	if got := r.Pop().MustGet(); got != 1 {
		t.Fatalf("want 1, got %d", got)
	}
	if got := r.Pop().MustGet(); got != 2 {
		t.Fatalf("want 2, got %d", got)
	}
}

func TestRing_Overwrite(t *testing.T) {
	r := ds.Ring[int](3)
	r.Push(1)
	r.Push(2)
	r.Push(3)
	r.Push(4) // overwrites 1

	got := r.Linearize()
	want := []int{2, 3, 4}
	if !slices.Equal(got, want) {
		t.Fatalf("want %v, got %v", want, got)
	}
}

func TestRing_PopEmpty(t *testing.T) {
	r := ds.Ring[int](2)
	if r.Pop().IsSome() {
		t.Fatal("expected None on empty pop")
	}
}

func TestRing_Peek(t *testing.T) {
	r := ds.Ring[int](3)
	r.Push(10)
	r.Push(20)

	if got := r.Peek().MustGet(); got != 10 {
		t.Fatalf("want 10, got %d", got)
	}
	if r.Len() != 2 {
		t.Fatalf("Peek must not remove elements")
	}
}

func TestRing_At(t *testing.T) {
	r := ds.Ring[int](4)
	r.Push(1)
	r.Push(2)
	r.Push(3)

	if got := r.At(0).MustGet(); got != 1 {
		t.Fatalf("want 1, got %d", got)
	}
	if got := r.At(2).MustGet(); got != 3 {
		t.Fatalf("want 3, got %d", got)
	}
	if r.At(3).IsSome() {
		t.Fatal("expected None for out-of-range At")
	}
}

func TestRing_FullEmpty(t *testing.T) {
	r := ds.Ring[int](2)
	if !r.Empty() {
		t.Fatal("new ring must be empty")
	}
	r.Push(1)
	r.Push(2)
	if !r.Full() {
		t.Fatal("ring must be full after pushing Cap elements")
	}
}

func TestRing_Segments_Contiguous(t *testing.T) {
	r := ds.Ring[int](4)
	r.Push(1)
	r.Push(2)
	r.Push(3)

	first, second := r.Segments()
	if !slices.Equal(first, []int{1, 2, 3}) {
		t.Fatalf("first want [1 2 3], got %v", first)
	}
	if len(second) != 0 {
		t.Fatalf("second should be empty, got %v", second)
	}
}

func TestRing_Segments_Wrapped(t *testing.T) {
	r := ds.Ring[int](3)
	r.Push(1)
	r.Push(2)
	r.Push(3)
	r.Pop() // head moves to index 1
	r.Push(4)

	first, second := r.Segments()
	combined := append(first, second...)
	if !slices.Equal(combined, []int{2, 3, 4}) {
		t.Fatalf("want [2 3 4], got %v", combined)
	}
}

func TestRing_Linearize(t *testing.T) {
	r := ds.Ring[int](3)
	r.Push(1)
	r.Push(2)
	r.Push(3)
	r.Pop()
	r.Push(4)

	got := r.Linearize()
	if !slices.Equal(got, []int{2, 3, 4}) {
		t.Fatalf("want [2 3 4], got %v", got)
	}
}

func TestRing_All(t *testing.T) {
	r := ds.Ring[int](4)
	r.Push(10)
	r.Push(20)
	r.Push(30)

	var got []int
	for v := range r.All() {
		got = append(got, v)
	}
	if !slices.Equal(got, []int{10, 20, 30}) {
		t.Fatalf("want [10 20 30], got %v", got)
	}
}
