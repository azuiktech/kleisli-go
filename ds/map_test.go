package ds_test

import (
	"maps"
	"slices"
	"testing"

	"github.com/azuiktech/kleisli-go/ds"
	"github.com/azuiktech/kleisli-go/stream"
)

func TestMap_Constructors(t *testing.T) {
	t.Run("NewMap and MapOf", func(t *testing.T) {
		m := ds.NewMap(
			ds.Entry[string, int]{Key: "a", Value: 1},
			ds.Entry[string, int]{Key: "b", Value: 2},
			ds.Entry[string, int]{Key: "a", Value: 10}, // duplicate overwrite
		)
		if m.Len() != 2 {
			t.Fatalf("expected len 2, got %d", m.Len())
		}
		if m.GetOr("a", 0) != 10 || m.GetOr("b", 0) != 2 {
			t.Fatalf("unexpected values in map: %v", m.ToMap())
		}

		m2 := ds.MapOf(
			ds.Entry[int, string]{Key: 1, Value: "one"},
			ds.Entry[int, string]{Key: 2, Value: "two"},
		)
		if m2.Len() != 2 || m2.GetOr(1, "") != "one" {
			t.Fatalf("unexpected MapOf result: %v", m2.ToMap())
		}
	})

	t.Run("MapWithCapacity", func(t *testing.T) {
		m := ds.MapWithCapacity[string, int](10)
		if m.Len() != 0 || !m.Empty() {
			t.Fatalf("expected empty map, got len %d", m.Len())
		}
		m.Put("k1", 100)
		if m.Len() != 1 || m.GetOr("k1", 0) != 100 {
			t.Fatalf("expected len 1, got %d", m.Len())
		}
	})

	t.Run("FromMap", func(t *testing.T) {
		raw := map[string]int{"x": 10, "y": 20}
		m := ds.FromMap(raw)
		if m.Len() != 2 {
			t.Fatalf("expected len 2, got %d", m.Len())
		}
		// Mutating raw map must not affect m
		raw["z"] = 30
		if m.Contains("z") {
			t.Fatal("FromMap must make independent copy")
		}
	})

	t.Run("FromSeq2", func(t *testing.T) {
		raw := map[string]int{"alpha": 1, "beta": 2}
		m := ds.FromSeq2(maps.All(raw))
		if m.Len() != 2 || m.GetOr("alpha", 0) != 1 || m.GetOr("beta", 0) != 2 {
			t.Fatalf("unexpected FromSeq2 result: %v", m.ToMap())
		}
	})
}

func TestMap_NilAndZeroSafety(t *testing.T) {
	var m ds.Map[string, int]

	if m.Len() != 0 {
		t.Fatalf("zero map Len() want 0, got %d", m.Len())
	}
	if !m.Empty() {
		t.Fatal("zero map Empty() want true, got false")
	}
	if m.Contains("any") {
		t.Fatal("zero map Contains want false, got true")
	}
	if opt := m.Get("any"); opt.IsSome() {
		t.Fatal("zero map Get want None, got Some")
	}
	if v := m.GetOr("any", 42); v != 42 {
		t.Fatalf("zero map GetOr want fallback 42, got %d", v)
	}
	if v := m.GetOrElse("any", func() int { return 99 }); v != 99 {
		t.Fatalf("zero map GetOrElse want 99, got %d", v)
	}
	if sl := m.ToSlice(); sl == nil || len(sl) != 0 {
		t.Fatalf("zero map ToSlice() want non-nil empty slice, got %v", sl)
	}
	if mp := m.ToMap(); mp == nil || len(mp) != 0 {
		t.Fatalf("zero map ToMap() want non-nil empty map, got %v", mp)
	}

	clone := m.Clone()
	if clone.Len() != 0 || !clone.Empty() {
		t.Fatalf("zero map Clone() want empty map, got len %d", clone.Len())
	}

	var iterCount int
	for range m.All() {
		iterCount++
	}
	if iterCount != 0 {
		t.Fatalf("zero map All() want 0, got %d", iterCount)
	}
}

func TestMap_InPlaceMutations(t *testing.T) {
	t.Run("Put and PutIfAbsent", func(t *testing.T) {
		m := ds.NewMap[string, int]()
		m.Put("a", 1)
		m.Put("b", 2)
		m.Put("a", 10) // overwrite
		if m.Len() != 2 || m.GetOr("a", 0) != 10 {
			t.Fatalf("expected len 2 with a=10, got len %d a=%d", m.Len(), m.GetOr("a", 0))
		}

		val, inserted := m.PutIfAbsent("b", 99)
		if inserted || val != 2 {
			t.Fatalf("PutIfAbsent existing want (2, false), got (%d, %v)", val, inserted)
		}

		val, inserted = m.PutIfAbsent("c", 30)
		if !inserted || val != 30 || m.GetOr("c", 0) != 30 {
			t.Fatalf("PutIfAbsent new want (30, true), got (%d, %v)", val, inserted)
		}
	})

	t.Run("PutAll", func(t *testing.T) {
		m := ds.NewMap(ds.Entry[string, int]{Key: "k1", Value: 1})
		m.PutAll(
			ds.Entry[string, int]{Key: "k2", Value: 2},
			ds.Entry[string, int]{Key: "k3", Value: 3},
		)
		if m.Len() != 3 || m.GetOr("k2", 0) != 2 || m.GetOr("k3", 0) != 3 {
			t.Fatalf("expected len 3 after PutAll, got len %d", m.Len())
		}
	})

	t.Run("Delete and DeleteFunc", func(t *testing.T) {
		m := ds.NewMap(
			ds.Entry[string, int]{Key: "a", Value: 1},
			ds.Entry[string, int]{Key: "b", Value: 2},
			ds.Entry[string, int]{Key: "c", Value: 3},
		)
		m.Delete("b")
		if m.Len() != 2 || m.Contains("b") {
			t.Fatal("Delete failed to remove 'b'")
		}

		m.DeleteFunc(func(k string, v int) bool {
			return v%2 != 0 // delete odd values ('a'=1, 'c'=3)
		})
		if m.Len() != 0 || !m.Empty() {
			t.Fatalf("expected empty map after DeleteFunc, got len %d", m.Len())
		}
	})

	t.Run("Clear", func(t *testing.T) {
		m := ds.NewMap(
			ds.Entry[string, int]{Key: "x", Value: 10},
			ds.Entry[string, int]{Key: "y", Value: 20},
		)
		m.Clear()
		if m.Len() != 0 || !m.Empty() {
			t.Fatalf("expected empty map after Clear, got len %d", m.Len())
		}
		if m.Contains("x") || m.Contains("y") {
			t.Fatal("cleared map still contains keys")
		}

		// Can still Put after Clear
		m.Put("z", 30)
		if m.Len() != 1 || m.GetOr("z", 0) != 30 {
			t.Fatalf("expected len 1 after Put following Clear, got %d", m.Len())
		}
	})
}

func TestMap_CloneAndEqualFunc(t *testing.T) {
	m1 := ds.NewMap(
		ds.Entry[string, int]{Key: "a", Value: 1},
		ds.Entry[string, int]{Key: "b", Value: 2},
	)
	m2 := m1.Clone()

	eq := func(v1, v2 int) bool { return v1 == v2 }
	if !m1.EqualFunc(m2, eq) {
		t.Fatal("clone must be EqualFunc to original")
	}

	// Mutate clone; original unaffected
	m2.Put("c", 3)
	if m1.EqualFunc(m2, eq) {
		t.Fatal("original and clone should not be equal after clone mutation")
	}
	if m1.Contains("c") {
		t.Fatal("original should not contain key added to clone")
	}
}

func TestMap_IteratorsAndStreamIntegration(t *testing.T) {
	m := ds.NewMap(
		ds.Entry[string, int]{Key: "alpha", Value: 10},
		ds.Entry[string, int]{Key: "beta", Value: 20},
		ds.Entry[string, int]{Key: "gamma", Value: 30},
	)

	// Keys iterator
	var keys []string
	for k := range m.Keys() {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	if !slices.Equal(keys, []string{"alpha", "beta", "gamma"}) {
		t.Fatalf("unexpected keys: %v", keys)
	}

	// Values iterator
	var values []int
	for v := range m.Values() {
		values = append(values, v)
	}
	slices.Sort(values)
	if !slices.Equal(values, []int{10, 20, 30}) {
		t.Fatalf("unexpected values: %v", values)
	}

	// Stream integration over Values
	sum := stream.FromSeq(m.Values()).
		Reduce(0, func(acc, v int) int { return acc + v })
	if sum != 60 {
		t.Fatalf("stream.Reduce on map values want 60, got %d", sum)
	}

	// All() Seq2 iteration
	count := 0
	for k, v := range m.All() {
		if m.GetOr(k, 0) != v {
			t.Fatalf("mismatch for key %s: got %d", k, v)
		}
		count++
	}
	if count != 3 {
		t.Fatalf("expected 3 entries in All(), got %d", count)
	}
}
