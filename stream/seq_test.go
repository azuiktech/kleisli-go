package stream

import (
	"reflect"
	"slices"
	"testing"

	"github.com/azuiktech/kleisli-go/adt"
)

func TestFromSeq_Collect_RoundTrips(t *testing.T) {
	got := FromSeq(slices.Values([]int{1, 2, 3})).Collect()
	want := []int{1, 2, 3}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Collect() = %v, want %v", got, want)
	}
}

func TestSeq_Filter(t *testing.T) {
	got := FromSeq(slices.Values([]int{1, 2, 3, 4, 5, 6})).
		Filter(func(n int) bool { return n%2 == 0 }).
		Collect()
	want := []int{2, 4, 6}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Filter().Collect() = %v, want %v", got, want)
	}
}

func TestSeq_Map(t *testing.T) {
	got := FromSeq(slices.Values([]int{1, 2, 3})).
		Map(func(n int) string { return string(rune('a' + n - 1)) }).
		Collect()
	want := []string{"a", "b", "c"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Map().Collect() = %v, want %v", got, want)
	}
}

func TestSeq_FlatMap(t *testing.T) {
	got := FromSeq(slices.Values([]int{1, 2, 3})).
		FlatMap(func(n int) []int { return []int{n, n * 10} }).
		Collect()
	want := []int{1, 10, 2, 20, 3, 30}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("FlatMap().Collect() = %v, want %v", got, want)
	}
}

func TestSeq_Take_StopsPullingUpstream(t *testing.T) {
	calls := 0
	source := func(yield func(int) bool) {
		for i := 1; i <= 1000; i++ {
			calls++
			if !yield(i) {
				return
			}
		}
	}
	got := FromSeq(source).Take(3).Collect()
	want := []int{1, 2, 3}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Take(3).Collect() = %v, want %v", got, want)
	}
	if calls != 3 {
		t.Errorf("source produced %d items, want exactly 3 (Take must stop pulling upstream)", calls)
	}
}

func TestSeq_Skip(t *testing.T) {
	got := FromSeq(slices.Values([]int{1, 2, 3, 4, 5})).Skip(2).Collect()
	want := []int{3, 4, 5}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Skip(2).Collect() = %v, want %v", got, want)
	}
}

func TestSeq_Each(t *testing.T) {
	var got []int
	FromSeq(slices.Values([]int{1, 2, 3})).Each(func(n int) { got = append(got, n) })
	want := []int{1, 2, 3}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Each() collected %v, want %v", got, want)
	}
}

func TestSeq_Any_ShortCircuits(t *testing.T) {
	calls := 0
	source := func(yield func(int) bool) {
		for i := 1; i <= 1000; i++ {
			calls++
			if !yield(i) {
				return
			}
		}
	}
	got := FromSeq(source).Any(func(n int) bool { return n == 3 })
	if !got {
		t.Error("Any() = false, want true")
	}
	if calls != 3 {
		t.Errorf("source produced %d items, want exactly 3 (Any must stop at the match)", calls)
	}
}

func TestSeq_All(t *testing.T) {
	got := FromSeq(slices.Values([]int{2, 4, 6})).All(func(n int) bool { return n%2 == 0 })
	if !got {
		t.Error("All() = false, want true")
	}
	got = FromSeq(slices.Values([]int{2, 4, 5})).All(func(n int) bool { return n%2 == 0 })
	if got {
		t.Error("All() = true, want false")
	}
}

func TestSeq_First_ShortCircuitsAcrossFilterAndMap(t *testing.T) {
	// The whole point of Seq over Stream: this must NOT process all 1000
	// upstream items just because Filter and Map are chained before First.
	calls := 0
	source := func(yield func(int) bool) {
		for i := 1; i <= 1000; i++ {
			calls++
			if !yield(i) {
				return
			}
		}
	}
	got := FromSeq(source).
		Filter(func(n int) bool { return n%2 == 0 }).
		Map(func(n int) string { return string(rune('0' + n%10)) }).
		First(func(s string) bool { return s == "4" })
	if got.IsNone() || got.MustGet() != "4" {
		t.Fatalf("First() = %v, want Some(\"4\")", got)
	}
	if calls != 4 {
		t.Errorf("source produced %d items, want exactly 4 — Filter/Map/First must short-circuit end to end", calls)
	}
}

func TestSeq_Reduce(t *testing.T) {
	got := FromSeq(slices.Values([]int{1, 2, 3, 4})).Reduce(0, func(acc, n int) int { return acc + n })
	if got != 10 {
		t.Errorf("Reduce() = %d, want 10", got)
	}
}

func TestSeq_MapMulti(t *testing.T) {
	got := FromSeq(slices.Values([]int{1, 2, 3, 4})).
		MapMulti(func(n int, emit func(int)) {
			if n%2 == 0 {
				emit(n)
				emit(n)
			}
		}).
		Collect()
	want := []int{2, 2, 4, 4}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("MapMulti().Collect() = %v, want %v", got, want)
	}
}

func TestSeq_FilterMap(t *testing.T) {
	got := FromSeq(slices.Values([]string{"1", "x", "3"})).
		FilterMap(func(s string) (int, bool) {
			switch s {
			case "1":
				return 1, true
			case "3":
				return 3, true
			default:
				return 0, false
			}
		}).
		Collect()
	want := []int{1, 3}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("FilterMap().Collect() = %v, want %v", got, want)
	}
}

func TestSeq_TakeWhile_StopsPullingUpstream(t *testing.T) {
	calls := 0
	source := func(yield func(int) bool) {
		for _, v := range []int{1, 2, 3, 100, 4, 5} {
			calls++
			if !yield(v) {
				return
			}
		}
	}
	got := FromSeq(source).TakeWhile(func(n int) bool { return n < 10 }).Collect()
	want := []int{1, 2, 3}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("TakeWhile().Collect() = %v, want %v", got, want)
	}
	if calls != 4 {
		t.Errorf("source produced %d items, want exactly 4 (stops at 100, never touches 4,5)", calls)
	}
}

func TestSeq_DropWhile(t *testing.T) {
	got := FromSeq(slices.Values([]int{1, 2, 3, 4, 1})).DropWhile(func(n int) bool { return n < 3 }).Collect()
	want := []int{3, 4, 1}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("DropWhile().Collect() = %v, want %v", got, want)
	}
}

func TestSeq_DistinctBy(t *testing.T) {
	got := FromSeq(slices.Values([]string{"a", "bb", "c", "dd", "eee"})).
		DistinctBy(func(s string) int { return len(s) }).
		Collect()
	want := []string{"a", "bb", "eee"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("DistinctBy().Collect() = %v, want %v", got, want)
	}
}

func TestDistinctSeq(t *testing.T) {
	got := DistinctSeq(FromSeq(slices.Values([]int{1, 2, 2, 3, 1}))).Collect()
	want := []int{1, 2, 3}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("DistinctSeq().Collect() = %v, want %v", got, want)
	}
}

func TestEnumerateSeq(t *testing.T) {
	got := EnumerateSeq(FromSeq(slices.Values([]string{"a", "b", "c"}))).Collect()
	want := []Indexed[string]{{0, "a"}, {1, "b"}, {2, "c"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("EnumerateSeq().Collect() = %+v, want %+v", got, want)
	}
}

func TestZipSeq(t *testing.T) {
	got := ZipSeq(FromSeq(slices.Values([]int{1, 2, 3})), FromSeq(slices.Values([]string{"a", "b"}))).Collect()
	want := []Pair[int, string]{{1, "a"}, {2, "b"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ZipSeq().Collect() = %+v, want %+v", got, want)
	}
}

func TestSeq_Gather_ScanWindowFold(t *testing.T) {
	scanned := FromSeq(slices.Values([]int{1, 2, 3, 4})).Gather(Scan(0, func(acc, n int) int { return acc + n })).Collect()
	if want := []int{1, 3, 6, 10}; !reflect.DeepEqual(scanned, want) {
		t.Errorf("Gather(Scan).Collect() = %v, want %v", scanned, want)
	}

	windowed := FromSeq(slices.Values([]int{1, 2, 3, 4, 5})).Gather(WindowFixed[int](2)).Collect()
	if want := [][]int{{1, 2}, {3, 4}, {5}}; !reflect.DeepEqual(windowed, want) {
		t.Errorf("Gather(WindowFixed).Collect() = %v, want %v", windowed, want)
	}

	folded := FromSeq(slices.Values([]int{1, 2, 3})).Gather(Fold(0, func(acc, n int) int { return acc + n })).Collect()
	if want := []int{6}; !reflect.DeepEqual(folded, want) {
		t.Errorf("Gather(Fold).Collect() = %v, want %v", folded, want)
	}
}

// TestStreamAndSeq_AgreeOnResults proves the shared-implementation
// property directly: Stream and Seq, given the same input and the same
// operation, must produce identical results — that's the whole reason
// they share an engine instead of two independently maintained loops.
func TestStreamAndSeq_AgreeOnResults(t *testing.T) {
	items := []int{5, 3, 8, 1, 9, 2, 7, 4, 6}
	double := func(n int) int { return n * 2 }
	even := func(n int) bool { return n%2 == 0 }

	streamResult := Of(items).Filter(even).Map(double).Collect()
	seqResult := FromSeq(slices.Values(items)).Filter(even).Map(double).Collect()

	if !reflect.DeepEqual(streamResult, seqResult) {
		t.Errorf("Stream result %v != Seq result %v for the same Filter+Map pipeline", streamResult, seqResult)
	}
}

func TestFromSlice_IsOf(t *testing.T) {
	got := FromSlice([]int{1, 2, 3}).Collect()
	want := []int{1, 2, 3}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("FromSlice().Collect() = %v, want %v", got, want)
	}
}

func TestToStream_Materializes(t *testing.T) {
	got := FromSeq(slices.Values([]int{1, 2, 3})).ToStream().Collect()
	if want := []int{1, 2, 3}; !reflect.DeepEqual(got, want) {
		t.Errorf("ToStream().Collect() = %v, want %v", got, want)
	}
}

func TestSeqOfOption(t *testing.T) {
	if got := SeqOfOption(adt.Some(7)).Collect(); !reflect.DeepEqual(got, []int{7}) {
		t.Errorf("SeqOfOption(Some) = %v, want [7]", got)
	}
	if got := SeqOfOption(adt.None[int]()).Collect(); len(got) != 0 {
		t.Errorf("SeqOfOption(None) = %v, want []", got)
	}
}

func TestSeqOfMap(t *testing.T) {
	m := map[string]int{"a": 1, "b": 2}
	got := SeqOfMap(m).ToStream().SortBy(func(p Pair[string, int]) string { return p.First }).Collect()
	want := []Pair[string, int]{{"a", 1}, {"b", 2}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("SeqOfMap() sorted = %+v, want %+v", got, want)
	}
}

func TestSeq_ToPipe_ProducesAllItems(t *testing.T) {
	got := FromSeq(slices.Values([]int{1, 2, 3})).ToPipe().Collect()
	if want := []int{1, 2, 3}; !reflect.DeepEqual(got, want) {
		t.Errorf("Seq.ToPipe().Collect() = %v, want %v", got, want)
	}
}

func TestFlattenSeq(t *testing.T) {
	got := FlattenSeq(FromSeq(slices.Values([][]int{{1, 2}, {3}, {4, 5, 6}}))).Collect()
	if want := []int{1, 2, 3, 4, 5, 6}; !reflect.DeepEqual(got, want) {
		t.Errorf("FlattenSeq() = %v, want %v", got, want)
	}
}

func TestSeq_MinBy(t *testing.T) {
	type item struct {
		Name string
		N    int
	}
	items := []item{{"b", 2}, {"a", 1}, {"c", 3}}

	got := FromSeq(slices.Values(items)).MinBy(func(i item) int { return i.N })
	if got.IsNone() || got.MustGet().N != 1 {
		t.Errorf("Seq.MinBy() = %+v, want Some({a 1})", got)
	}

	if FromSeq(slices.Values([]item{})).MinBy(func(i item) int { return i.N }).IsSome() {
		t.Error("Seq.MinBy() on empty Seq returned Some")
	}
}

func TestSeq_MaxBy(t *testing.T) {
	type item struct {
		Name string
		N    int
	}
	items := []item{{"b", 2}, {"a", 1}, {"c", 3}}

	got := FromSeq(slices.Values(items)).MaxBy(func(i item) int { return i.N })
	if got.IsNone() || got.MustGet().N != 3 {
		t.Errorf("Seq.MaxBy() = %+v, want Some({c 3})", got)
	}

	if FromSeq(slices.Values([]item{})).MaxBy(func(i item) int { return i.N }).IsSome() {
		t.Error("Seq.MaxBy() on empty Seq returned Some")
	}
}

func TestSeq_ForEach_DrainsSequence(t *testing.T) {
	var got []int
	FromSeq(slices.Values([]int{1, 2, 3})).ForEach(func(v int) { got = append(got, v) })
	if !reflect.DeepEqual(got, []int{1, 2, 3}) {
		t.Errorf("ForEach collected %v, want [1 2 3]", got)
	}
}

func TestSeq_Tap_IsLazyAndPassesThrough(t *testing.T) {
	var tapped []int
	got := FromSeq(slices.Values([]int{1, 2, 3})).
		Tap(func(v int) { tapped = append(tapped, v) }).
		Collect()
	if !reflect.DeepEqual(got, []int{1, 2, 3}) {
		t.Errorf("Tap changed values: got %v", got)
	}
	if !reflect.DeepEqual(tapped, []int{1, 2, 3}) {
		t.Errorf("Tap side-effect saw %v, want [1 2 3]", tapped)
	}
}
