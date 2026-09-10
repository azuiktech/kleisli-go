package ds_test

import (
	"slices"
	"testing"

	"github.com/azuiktech/kleisli-go/ds"
	"github.com/azuiktech/kleisli-go/stream"
)

func TestVec_Constructors(t *testing.T) {
	t.Run("NewVec and VecOf", func(t *testing.T) {
		v := ds.NewVec(1, 2, 3)
		if v.Len() != 3 {
			t.Fatalf("expected len 3, got %d", v.Len())
		}
		if v.GetOr(0, 0) != 1 || v.GetOr(1, 0) != 2 || v.GetOr(2, 0) != 3 {
			t.Fatalf("unexpected vec contents: %v", v.ToSlice())
		}

		v2 := ds.VecOf("x", "y")
		if v2.Len() != 2 || v2.GetOr(0, "") != "x" {
			t.Fatalf("unexpected VecOf result: %v", v2.ToSlice())
		}
	})

	t.Run("VecWithCapacity", func(t *testing.T) {
		v := ds.VecWithCapacity[int](10)
		if v.Len() != 0 || !v.Empty() {
			t.Fatalf("expected empty vec, got len %d", v.Len())
		}
		if v.Cap() < 10 {
			t.Fatalf("expected cap >= 10, got %d", v.Cap())
		}
		v.Push(100)
		if v.Len() != 1 || v.GetOr(0, 0) != 100 {
			t.Fatalf("unexpected elements after push: %v", v.ToSlice())
		}
	})

	t.Run("VecFromSlice", func(t *testing.T) {
		raw := []int{10, 20, 30}
		v := ds.VecFromSlice(raw)
		if v.Len() != 3 {
			t.Fatalf("expected len 3, got %d", v.Len())
		}
		// Mutating raw slice must not affect v
		raw[0] = 999
		if v.GetOr(0, 0) == 999 {
			t.Fatal("VecFromSlice must clone backing array")
		}
	})

	t.Run("VecFromSeq", func(t *testing.T) {
		seq := slices.Values([]string{"a", "b", "c"})
		v := ds.VecFromSeq(seq)
		if v.Len() != 3 || v.GetOr(0, "") != "a" || v.GetOr(2, "") != "c" {
			t.Fatalf("unexpected VecFromSeq result: %v", v.ToSlice())
		}
	})
}

func TestVec_NilAndZeroSafety(t *testing.T) {
	var v ds.Vec[int]

	if v.Len() != 0 {
		t.Fatalf("zero vec Len() want 0, got %d", v.Len())
	}
	if v.Cap() != 0 {
		t.Fatalf("zero vec Cap() want 0, got %d", v.Cap())
	}
	if !v.Empty() {
		t.Fatal("zero vec Empty() want true, got false")
	}
	if opt := v.Get(0); opt.IsSome() {
		t.Fatal("zero vec Get(0) want None, got Some")
	}
	if val := v.GetOr(0, 42); val != 42 {
		t.Fatalf("zero vec GetOr want 42, got %d", val)
	}
	if opt := v.First(); opt.IsSome() {
		t.Fatal("zero vec First() want None, got Some")
	}
	if opt := v.Last(); opt.IsSome() {
		t.Fatal("zero vec Last() want None, got Some")
	}
	if sl := v.ToSlice(); sl == nil || len(sl) != 0 {
		t.Fatalf("zero vec ToSlice() want non-nil empty slice, got %v", sl)
	}

	clone := v.Clone()
	if clone.Len() != 0 || !clone.Empty() {
		t.Fatalf("zero vec Clone() want empty vec, got len %d", clone.Len())
	}

	var iterCount int
	for range v.All() {
		iterCount++
	}
	if iterCount != 0 {
		t.Fatalf("zero vec All() want 0 iterations, got %d", iterCount)
	}

	for range v.AllWithIndex() {
		iterCount++
	}
	if iterCount != 0 {
		t.Fatalf("zero vec AllWithIndex() want 0 iterations, got %d", iterCount)
	}

	// Lazy auto-initialization on Push
	v.Push(42)
	if v.Len() != 1 || v.GetOr(0, 0) != 42 {
		t.Fatal("uninitialized vec Push should auto-initialize")
	}
}

func TestVec_InPlaceMutations(t *testing.T) {
	t.Run("Push and PushAll", func(t *testing.T) {
		v := ds.NewVec[int]()
		v.Push(1)
		v.Push(2)
		v.PushAll(3, 4, 5)
		if v.Len() != 5 {
			t.Fatalf("expected len 5, got %d", v.Len())
		}
		if !slices.Equal(v.ToSlice(), []int{1, 2, 3, 4, 5}) {
			t.Fatalf("unexpected slice contents: %v", v.ToSlice())
		}
	})

	t.Run("Pop", func(t *testing.T) {
		v := ds.NewVec(10, 20)
		p1 := v.Pop()
		if !p1.IsSome() || p1.MustGet() != 20 {
			t.Fatalf("Pop() want Some(20), got %v", p1)
		}
		if v.Len() != 1 {
			t.Fatalf("expected len 1 after pop, got %d", v.Len())
		}

		p2 := v.Pop()
		if !p2.IsSome() || p2.MustGet() != 10 {
			t.Fatalf("Pop() want Some(10), got %v", p2)
		}
		if !v.Empty() {
			t.Fatalf("expected empty vec after second pop, got len %d", v.Len())
		}

		p3 := v.Pop()
		if p3.IsSome() {
			t.Fatal("Pop() on empty vec want None, got Some")
		}
	})

	t.Run("Insert", func(t *testing.T) {
		v := ds.NewVec(1, 3)
		if !v.Insert(1, 2) {
			t.Fatal("Insert at index 1 failed")
		}
		if !slices.Equal(v.ToSlice(), []int{1, 2, 3}) {
			t.Fatalf("unexpected after insert: %v", v.ToSlice())
		}

		// Insert at 0 (prepend)
		if !v.Insert(0, 0) {
			t.Fatal("Insert at index 0 failed")
		}
		// Insert at Len() (append)
		if !v.Insert(v.Len(), 4) {
			t.Fatal("Insert at Len() failed")
		}
		if !slices.Equal(v.ToSlice(), []int{0, 1, 2, 3, 4}) {
			t.Fatalf("unexpected after boundary inserts: %v", v.ToSlice())
		}

		// Out of bounds insert
		if v.Insert(-1, 99) || v.Insert(100, 99) {
			t.Fatal("Insert with out of bounds index must return false")
		}
	})

	t.Run("DeleteAt", func(t *testing.T) {
		v := ds.NewVec(10, 20, 30, 40)
		del := v.DeleteAt(1)
		if !del.IsSome() || del.MustGet() != 20 {
			t.Fatalf("DeleteAt(1) want Some(20), got %v", del)
		}
		if !slices.Equal(v.ToSlice(), []int{10, 30, 40}) {
			t.Fatalf("unexpected after DeleteAt: %v", v.ToSlice())
		}

		// Out of bounds
		if v.DeleteAt(-1).IsSome() || v.DeleteAt(10).IsSome() {
			t.Fatal("DeleteAt out of bounds must return None")
		}
	})

	t.Run("DeleteFunc", func(t *testing.T) {
		v := ds.NewVec(1, 2, 3, 4, 5, 6)
		v.DeleteFunc(func(n int) bool {
			return n%2 == 0 // delete evens
		})
		if !slices.Equal(v.ToSlice(), []int{1, 3, 5}) {
			t.Fatalf("unexpected after DeleteFunc: %v", v.ToSlice())
		}
	})

	t.Run("Truncate", func(t *testing.T) {
		v := ds.NewVec(1, 2, 3, 4, 5)
		v.Truncate(3)
		if !slices.Equal(v.ToSlice(), []int{1, 2, 3}) {
			t.Fatalf("unexpected after Truncate(3): %v", v.ToSlice())
		}
		// Truncate larger is a no-op
		v.Truncate(10)
		if v.Len() != 3 {
			t.Fatalf("Truncate(10) should be no-op, got len %d", v.Len())
		}
		// Truncate negative clears
		v.Truncate(-1)
		if !v.Empty() {
			t.Fatalf("Truncate(-1) should clear vec, got len %d", v.Len())
		}
	})

	t.Run("Reverse", func(t *testing.T) {
		v := ds.NewVec(1, 2, 3)
		v.Reverse()
		if !slices.Equal(v.ToSlice(), []int{3, 2, 1}) {
			t.Fatalf("unexpected after Reverse: %v", v.ToSlice())
		}
	})

	t.Run("Set", func(t *testing.T) {
		v := ds.NewVec("a", "b", "c")
		if !v.Set(1, "B") {
			t.Fatal("Set(1, 'B') failed")
		}
		if v.GetOr(1, "") != "B" {
			t.Fatalf("Set failed, got %s", v.GetOr(1, ""))
		}
		if v.Set(-1, "x") || v.Set(3, "x") {
			t.Fatal("Set out of bounds must return false")
		}
	})

	t.Run("Clear", func(t *testing.T) {
		v := ds.NewVec(1, 2, 3)
		initialCap := v.Cap()
		v.Clear()
		if v.Len() != 0 || !v.Empty() {
			t.Fatalf("expected empty vec after Clear, got len %d", v.Len())
		}
		if v.Cap() != initialCap {
			t.Fatalf("Clear should preserve capacity: want %d, got %d", initialCap, v.Cap())
		}
		v.Push(10)
		if v.Len() != 1 || v.GetOr(0, 0) != 10 {
			t.Fatalf("failed to push after Clear: %v", v.ToSlice())
		}
	})
}

func TestVec_AccessAndQueries(t *testing.T) {
	v := ds.NewVec(10, 20, 30)

	if opt := v.Get(1); !opt.IsSome() || opt.MustGet() != 20 {
		t.Fatalf("Get(1) want Some(20), got %v", opt)
	}
	if opt := v.Get(5); opt.IsSome() {
		t.Fatalf("Get(5) want None, got %v", opt)
	}

	if f := v.First(); !f.IsSome() || f.MustGet() != 10 {
		t.Fatalf("First() want Some(10), got %v", f)
	}
	if l := v.Last(); !l.IsSome() || l.MustGet() != 30 {
		t.Fatalf("Last() want Some(30), got %v", l)
	}

	if !v.ContainsFunc(func(n int) bool { return n == 20 }) {
		t.Fatal("ContainsFunc for 20 want true, got false")
	}
	if v.ContainsFunc(func(n int) bool { return n == 99 }) {
		t.Fatal("ContainsFunc for 99 want false, got true")
	}

	if idx := v.IndexOfFunc(func(n int) bool { return n == 30 }); idx != 2 {
		t.Fatalf("IndexOfFunc for 30 want 2, got %d", idx)
	}
	if idx := v.IndexOfFunc(func(n int) bool { return n == 99 }); idx != -1 {
		t.Fatalf("IndexOfFunc for non-existent want -1, got %d", idx)
	}
}

func TestVec_Clone(t *testing.T) {
	v1 := ds.NewVec(1, 2, 3)
	v2 := v1.Clone()

	if !slices.Equal(v1.ToSlice(), v2.ToSlice()) {
		t.Fatal("clone must match original")
	}

	v2.Push(4)
	if v1.Len() != 3 || v2.Len() != 4 {
		t.Fatal("mutating clone must not affect original")
	}
}

func TestVec_IteratorsAndStreamIntegration(t *testing.T) {
	v := ds.NewVec(10, 20, 30)

	// All() iterator
	var collected []int
	for item := range v.All() {
		collected = append(collected, item)
	}
	if !slices.Equal(collected, []int{10, 20, 30}) {
		t.Fatalf("unexpected All() results: %v", collected)
	}

	// AllWithIndex() iterator
	var indices []int
	var elements []int
	for idx, val := range v.AllWithIndex() {
		indices = append(indices, idx)
		elements = append(elements, val)
	}
	if !slices.Equal(indices, []int{0, 1, 2}) || !slices.Equal(elements, []int{10, 20, 30}) {
		t.Fatalf("unexpected AllWithIndex results: %v, %v", indices, elements)
	}

	// Stream integration
	doubledEvens := stream.FromSeq(v.All()).
		Filter(func(n int) bool { return n > 15 }).
		Map(func(n int) int { return n * 2 }).
		Collect()
	if !slices.Equal(doubledEvens, []int{40, 60}) {
		t.Fatalf("stream integration want [40 60], got %v", doubledEvens)
	}
}
