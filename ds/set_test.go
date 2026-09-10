package ds_test

import (
	"slices"
	"testing"

	"github.com/azuiktech/kleisli-go/ds"
	"github.com/azuiktech/kleisli-go/stream"
)

func TestSet_Constructors(t *testing.T) {
	t.Run("SetOf", func(t *testing.T) {
		s := ds.SetOf(1, 2, 3, 2, 1)
		if s.Len() != 3 {
			t.Fatalf("expected len 3, got %d", s.Len())
		}
		if !s.Contains(1) || !s.Contains(2) || !s.Contains(3) {
			t.Fatalf("missing elements in set: %v", s.ToSlice())
		}
	})

	t.Run("SetWithCapacity", func(t *testing.T) {
		s := ds.SetWithCapacity[string](10)
		if s.Len() != 0 || !s.Empty() {
			t.Fatalf("expected empty set with capacity, got len %d", s.Len())
		}
		s.Add("alpha").Add("beta")
		if s.Len() != 2 {
			t.Fatalf("expected len 2, got %d", s.Len())
		}
	})

	t.Run("FromSlice", func(t *testing.T) {
		items := []int{10, 20, 30, 20, 10}
		s := ds.FromSlice(items)
		if s.Len() != 3 {
			t.Fatalf("expected len 3, got %d", s.Len())
		}
		if !s.Contains(10) || !s.Contains(20) || !s.Contains(30) {
			t.Fatalf("missing elements from slice: %v", s.ToSlice())
		}
	})

	t.Run("FromSeq", func(t *testing.T) {
		seq := slices.Values([]string{"x", "y", "z", "y"})
		s := ds.FromSeq(seq)
		if s.Len() != 3 {
			t.Fatalf("expected len 3, got %d", s.Len())
		}
		if !s.Contains("x") || !s.Contains("y") || !s.Contains("z") {
			t.Fatalf("missing elements from seq: %v", s.ToSlice())
		}
	})
}

func TestSet_NilSafety(t *testing.T) {
	var s ds.Set[int]

	if s.Len() != 0 {
		t.Fatalf("nil set Len() want 0, got %d", s.Len())
	}
	if !s.Empty() {
		t.Fatal("nil set Empty() want true, got false")
	}
	if s.Contains(42) {
		t.Fatal("nil set Contains(42) want false, got true")
	}
	if slice := s.ToSlice(); slice == nil || len(slice) != 0 {
		t.Fatalf("nil set ToSlice() want non-nil empty slice, got %v (is nil: %v)", slice, slice == nil)
	}

	clone := s.Clone()
	if clone == nil || clone.Len() != 0 {
		t.Fatalf("nil set Clone() want non-nil empty set, got len %d (is nil: %v)", clone.Len(), clone == nil)
	}

	var count int
	for range s.All() {
		count++
	}
	if count != 0 {
		t.Fatalf("nil set All() want 0 iterations, got %d", count)
	}

	empty := ds.SetOf[int]()
	if !s.Equal(empty) {
		t.Fatal("nil set should be equal to empty set")
	}
	if !empty.Equal(s) {
		t.Fatal("empty set should be equal to nil set")
	}
}

func TestSet_Mutations(t *testing.T) {
	t.Run("Add", func(t *testing.T) {
		s := ds.SetOf[int]()
		s.Add(1).Add(2).Add(1) // chaining and duplicate handling
		if s.Len() != 2 {
			t.Fatalf("expected len 2, got %d", s.Len())
		}
		if !s.Contains(1) || !s.Contains(2) {
			t.Fatal("elements not found after Add")
		}
	})

	t.Run("AddAll", func(t *testing.T) {
		s1 := ds.SetOf(1, 2)
		s1.AddAll(3, 4, 2)
		if s1.Len() != 4 {
			t.Fatalf("expected len 4, got %d", s1.Len())
		}

		s2 := ds.SetOf(5, 6)
		s1.AddAll(s2.ToSlice()...)
		if s1.Len() != 6 {
			t.Fatalf("expected len 6, got %d", s1.Len())
		}
		for i := 1; i <= 6; i++ {
			if !s1.Contains(i) {
				t.Fatalf("missing element %d after AddAll", i)
			}
		}
	})

	t.Run("Delete", func(t *testing.T) {
		s := ds.SetOf(10, 20, 30)
		s.Delete(20).Delete(30)
		if s.Len() != 1 {
			t.Fatalf("expected len 1, got %d", s.Len())
		}
		if s.Contains(20) || s.Contains(30) {
			t.Fatal("removed elements still present")
		}
		if !s.Contains(10) {
			t.Fatal("unremoved element missing")
		}

		// Removing non-existent item is a no-op
		s.Delete(999)
		if s.Len() != 1 {
			t.Fatalf("expected len 1 after deleting non-existent, got %d", s.Len())
		}
	})

	t.Run("DeleteFunc", func(t *testing.T) {
		s := ds.SetOf(1, 2, 3, 4, 5, 6)
		s.DeleteFunc(func(n int) bool {
			return n%2 == 0
		})
		if s.Len() != 3 {
			t.Fatalf("expected len 3 after DeleteFunc, got %d", s.Len())
		}
		if s.Contains(2) || s.Contains(4) || s.Contains(6) {
			t.Fatal("even elements not removed by DeleteFunc")
		}
		if !s.Contains(1) || !s.Contains(3) || !s.Contains(5) {
			t.Fatal("odd elements missing after DeleteFunc")
		}
	})

	t.Run("Clear", func(t *testing.T) {
		s := ds.SetOf("a", "b", "c")
		s.Clear()
		if s.Len() != 0 || !s.Empty() {
			t.Fatalf("expected empty set after Clear, got len %d", s.Len())
		}
		if s.Contains("a") {
			t.Fatal("set still contains elements after Clear")
		}
	})
}

func TestSet_CloneAndEqual(t *testing.T) {
	s1 := ds.SetOf(1, 2, 3)
	s2 := s1.Clone()

	if !s1.Equal(s2) {
		t.Fatal("clone must be equal to original")
	}

	// Mutate clone; original must remain unaffected
	s2.Add(4)
	if s1.Equal(s2) {
		t.Fatal("original and clone should not be equal after clone mutation")
	}
	if s1.Contains(4) {
		t.Fatal("original should not contain element added to clone")
	}

	s3 := ds.SetOf(1, 2, 4)
	if s1.Equal(s3) {
		t.Fatal("differing sets should not be equal")
	}
}

func TestSet_Algebra(t *testing.T) {
	a := ds.SetOf(1, 2, 3)
	b := ds.SetOf(3, 4, 5)

	t.Run("Union", func(t *testing.T) {
		u := a.Union(b)
		if u.Len() != 5 {
			t.Fatalf("Union want len 5, got %d", u.Len())
		}
		for i := 1; i <= 5; i++ {
			if !u.Contains(i) {
				t.Fatalf("Union missing element %d", i)
			}
		}
		// Operands must not be mutated
		if a.Len() != 3 || b.Len() != 3 {
			t.Fatal("Union must not mutate operands")
		}
	})

	t.Run("Intersect", func(t *testing.T) {
		inter := a.Intersect(b)
		if inter.Len() != 1 {
			t.Fatalf("Intersect want len 1, got %d", inter.Len())
		}
		if !inter.Contains(3) {
			t.Fatal("Intersect missing common element 3")
		}
		if a.Len() != 3 || b.Len() != 3 {
			t.Fatal("Intersect must not mutate operands")
		}
	})

	t.Run("Diff", func(t *testing.T) {
		diffAB := a.Diff(b)
		if diffAB.Len() != 2 {
			t.Fatalf("Diff A\\B want len 2, got %d", diffAB.Len())
		}
		if !diffAB.Contains(1) || !diffAB.Contains(2) {
			t.Fatal("Diff A\\B missing elements 1 or 2")
		}
		if diffAB.Contains(3) {
			t.Fatal("Diff A\\B contains element 3 which is in B")
		}

		diffBA := b.Diff(a)
		if diffBA.Len() != 2 || !diffBA.Contains(4) || !diffBA.Contains(5) {
			t.Fatalf("Diff B\\A want {4, 5}, got %v", diffBA.ToSlice())
		}
	})

	t.Run("SymmetricDiff", func(t *testing.T) {
		sym := a.SymmetricDiff(b)
		if sym.Len() != 4 {
			t.Fatalf("SymmetricDiff want len 4, got %d", sym.Len())
		}
		if !sym.Contains(1) || !sym.Contains(2) || !sym.Contains(4) || !sym.Contains(5) {
			t.Fatalf("SymmetricDiff missing elements: %v", sym.ToSlice())
		}
		if sym.Contains(3) {
			t.Fatal("SymmetricDiff must not contain shared element 3")
		}
	})

	t.Run("IsSubset and IsSuperset", func(t *testing.T) {
		sub := ds.SetOf(1, 2)
		if !sub.IsSubset(a) {
			t.Fatal("{1, 2} should be subset of {1, 2, 3}")
		}
		if !a.IsSuperset(sub) {
			t.Fatal("{1, 2, 3} should be superset of {1, 2}")
		}
		if a.IsSubset(sub) {
			t.Fatal("{1, 2, 3} should not be subset of {1, 2}")
		}
		// Reflexivity
		if !a.IsSubset(a) || !a.IsSuperset(a) {
			t.Fatal("Set should be subset and superset of itself")
		}
		// Empty set subset of all
		empty := ds.SetOf[int]()
		if !empty.IsSubset(a) {
			t.Fatal("empty set should be subset of any set")
		}
	})

	t.Run("IsDisjoint", func(t *testing.T) {
		d := ds.SetOf(6, 7)
		if !a.IsDisjoint(d) {
			t.Fatal("{1, 2, 3} and {6, 7} should be disjoint")
		}
		if a.IsDisjoint(b) {
			t.Fatal("{1, 2, 3} and {3, 4, 5} share 3 and should not be disjoint")
		}
		empty := ds.SetOf[int]()
		if !a.IsDisjoint(empty) {
			t.Fatal("any set should be disjoint with empty set")
		}
	})
}

func TestSet_IteratorAndStreamIntegration(t *testing.T) {
	s := ds.SetOf(10, 20, 30)

	// Standard Go range-over-func
	var collected []int
	for v := range s.All() {
		collected = append(collected, v)
	}
	slices.Sort(collected)
	if !slices.Equal(collected, []int{10, 20, 30}) {
		t.Fatalf("range s.All() want [10 20 30], got %v", collected)
	}

	// Kleisli-go stream.FromSeq integration
	evens := stream.FromSeq(s.All()).
		Filter(func(v int) bool { return v > 15 }).
		Collect()
	slices.Sort(evens)
	if !slices.Equal(evens, []int{20, 30}) {
		t.Fatalf("stream.FromSeq want [20 30], got %v", evens)
	}

	// Kleisli-go stream.Of(s.ToSlice()) integration
	sum := stream.Of(s.ToSlice()).
		Reduce(0, func(acc, v int) int { return acc + v })
	if sum != 60 {
		t.Fatalf("stream.Reduce sum want 60, got %d", sum)
	}
}
