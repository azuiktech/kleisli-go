package ds_test

import (
	"slices"
	"sync"
	"testing"

	"github.com/azuiktech/kleisli-go/ds"
)

func TestSyncRing_BasicOps(t *testing.T) {
	r := ds.GuardedRing[int](3)
	r.Push(1)
	r.Push(2)
	r.Push(3)

	if got := r.Pop().MustGet(); got != 1 {
		t.Fatalf("want 1, got %d", got)
	}
	if r.Len() != 2 {
		t.Fatalf("want len 2, got %d", r.Len())
	}
	if r.Cap() != 3 {
		t.Fatalf("want cap 3, got %d", r.Cap())
	}
}

func TestSyncRing_Overwrite(t *testing.T) {
	r := ds.GuardedRing[int](3)
	r.Push(1)
	r.Push(2)
	r.Push(3)
	r.Push(4)

	got := r.Linearize()
	if !slices.Equal(got, []int{2, 3, 4}) {
		t.Fatalf("want [2 3 4], got %v", got)
	}
}

func TestSyncRing_Segments_Independent(t *testing.T) {
	r := ds.GuardedRing[int](3)
	r.Push(10)
	r.Push(20)
	r.Push(30)
	r.Pop()
	r.Push(40)

	first, second := r.Segments()
	combined := append(first, second...)
	if !slices.Equal(combined, []int{20, 30, 40}) {
		t.Fatalf("want [20 30 40], got %v", combined)
	}

	r.Push(50)
	// Previously returned slices must be unaffected.
	if !slices.Equal(append(first, second...), []int{20, 30, 40}) {
		t.Fatal("Segments slices must be independent copies")
	}
}

func TestSyncRing_All(t *testing.T) {
	r := ds.GuardedRing[string](4)
	r.Push("a")
	r.Push("b")
	r.Push("c")

	var got []string
	for v := range r.All() {
		got = append(got, v)
	}
	if !slices.Equal(got, []string{"a", "b", "c"}) {
		t.Fatalf("want [a b c], got %v", got)
	}
}

func TestSyncRing_Concurrent(t *testing.T) {
	r := ds.GuardedRing[int](512)
	var wg sync.WaitGroup
	for i := range 200 {
		wg.Add(1)
		go func(v int) {
			defer wg.Done()
			r.Push(v)
		}(i)
	}
	for range 50 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r.Pop()
		}()
	}
	wg.Wait()

	if r.Len() < 0 || r.Len() > 512 {
		t.Fatalf("unexpected len %d after concurrent ops", r.Len())
	}
}
