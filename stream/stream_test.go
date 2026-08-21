package stream

import (
	"reflect"
	"testing"

	"github.com/azuiktech/kleisli-go/adt"
)

func TestLast(t *testing.T) {
	got := Of([]int{1, 2, 3, 2, 1}).Last(func(n int) bool { return n == 2 })
	if got.IsNone() || got.MustGet() != 2 {
		t.Errorf("Last() = %v, want Some(2)", got)
	}

	if Of([]int{1, 2, 3}).Last(func(n int) bool { return n == 99 }).IsSome() {
		t.Error("Last() found a match that doesn't exist")
	}
}

func TestLast_ShortCircuitsFromTheEnd(t *testing.T) {
	calls := 0
	items := make([]int, 100)
	for i := range items {
		items[i] = i
	}

	got := Of(items).Last(func(n int) bool {
		calls++
		return n == 99 // the very last element
	})
	if got.IsNone() || got.MustGet() != 99 {
		t.Fatalf("Last() = %v, want Some(99)", got)
	}
	if calls != 1 {
		t.Errorf("fn called %d times, want 1 (Last should scan backward and stop immediately)", calls)
	}
}

func TestGather_EarlyTerminationAndFinish(t *testing.T) {
	// A Gatherer that stops after 3 items and flushes a sentinel via Finish
	// — exercises both the cont=false short-circuit and the Finish hook.
	g := Gatherer[int, int, string]{
		Init: func() int { return 0 },
		Integrate: func(count int, item int, emit func(string)) (int, bool) {
			if count == 3 {
				return count, false
			}
			emit("saw")
			return count + 1, true
		},
		Finish: func(count int, emit func(string)) {
			emit("done-at-" + string(rune('0'+count)))
		},
	}

	got := Of([]int{1, 2, 3, 4, 5, 6}).Gather(g).Collect()
	want := []string{"saw", "saw", "saw", "done-at-3"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Gather() = %v, want %v", got, want)
	}
}

func TestMapMulti(t *testing.T) {
	// emits the value twice for even numbers, zero times for odd — proves
	// MapMulti covers both the "many" and the "zero" cases in one pass.
	got := Of([]int{1, 2, 3, 4}).
		MapMulti(func(n int, emit func(int)) {
			if n%2 == 0 {
				emit(n)
				emit(n)
			}
		}).
		Collect()
	want := []int{2, 2, 4, 4}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("MapMulti() = %v, want %v", got, want)
	}
}

func TestFilterMap(t *testing.T) {
	got := Of([]string{"1", "x", "3", "y", "5"}).
		FilterMap(func(s string) (int, bool) {
			switch s {
			case "1":
				return 1, true
			case "3":
				return 3, true
			case "5":
				return 5, true
			default:
				return 0, false
			}
		}).
		Collect()
	want := []int{1, 3, 5}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("FilterMap() = %v, want %v", got, want)
	}
}

func TestTakeWhile(t *testing.T) {
	got := Of([]int{1, 2, 3, 10, 2, 1}).TakeWhile(func(n int) bool { return n < 5 }).Collect()
	want := []int{1, 2, 3}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("TakeWhile() = %v, want %v", got, want)
	}
}

func TestDropWhile(t *testing.T) {
	got := Of([]int{1, 2, 3, 10, 2, 1}).DropWhile(func(n int) bool { return n < 5 }).Collect()
	want := []int{10, 2, 1}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("DropWhile() = %v, want %v", got, want)
	}
}

func TestEnumerate(t *testing.T) {
	got := Enumerate(Of([]string{"a", "b", "c"})).Collect()
	want := []Indexed[string]{{0, "a"}, {1, "b"}, {2, "c"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Enumerate() = %+v, want %+v", got, want)
	}
}

func TestDistinctBy(t *testing.T) {
	type item struct {
		Key   string
		Label string
	}
	got := Of([]item{{"a", "first"}, {"b", "x"}, {"a", "second"}}).
		DistinctBy(func(i item) string { return i.Key }).
		Collect()
	want := []item{{"a", "first"}, {"b", "x"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("DistinctBy() = %+v, want %+v", got, want)
	}
}

func TestDistinct(t *testing.T) {
	got := Distinct(Of([]int{1, 2, 1, 3, 2, 4})).Collect()
	want := []int{1, 2, 3, 4}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Distinct() = %v, want %v", got, want)
	}
}

func TestScan(t *testing.T) {
	got := Of([]int{1, 2, 3, 4}).Gather(Scan(0, func(acc, n int) int { return acc + n })).Collect()
	want := []int{1, 3, 6, 10}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Scan() = %v, want %v", got, want)
	}
}

func TestFold(t *testing.T) {
	got := Of([]int{1, 2, 3, 4}).Gather(Fold(0, func(acc, n int) int { return acc + n })).Collect()
	want := []int{10}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Fold() = %v, want %v (a single, final accumulator)", got, want)
	}
}

func TestFold_ChainsIntoFlatMap(t *testing.T) {
	// The whole reason Fold exists over Reduce: the accumulator keeps
	// flowing as a Stream[U] instead of ending the pipeline.
	got := Of([]int{1, 2, 3}).
		Gather(Fold(0, func(acc, n int) int { return acc + n })).
		FlatMap(func(seed int) []int { return []int{seed, seed * 2} }).
		Collect()
	want := []int{6, 12}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Fold->FlatMap = %v, want %v", got, want)
	}
}

func TestWindowFixed(t *testing.T) {
	tests := []struct {
		name  string
		items []int
		n     int
		want  [][]int
	}{
		{"evenly divides", []int{1, 2, 3, 4}, 2, [][]int{{1, 2}, {3, 4}}},
		{"trailing short chunk", []int{1, 2, 3, 4, 5}, 2, [][]int{{1, 2}, {3, 4}, {5}}},
		{"n larger than input", []int{1, 2}, 5, [][]int{{1, 2}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Of(tt.items).Gather(WindowFixed[int](tt.n)).Collect()
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("WindowFixed(%d) = %v, want %v", tt.n, got, tt.want)
			}
		})
	}
}

func TestWindowSliding(t *testing.T) {
	got := Of([]int{1, 2, 3, 4}).Gather(WindowSliding[int](2)).Collect()
	want := [][]int{{1, 2}, {2, 3}, {3, 4}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("WindowSliding(2) = %v, want %v", got, want)
	}

	// snapshots must not alias each other or the internal buffer.
	if &got[0][0] == &got[1][0] {
		t.Error("WindowSliding windows share backing arrays, want independent copies")
	}
}

func TestSortBy(t *testing.T) {
	type item struct {
		Name string
		N    int
	}
	items := []item{{"c", 3}, {"a", 1}, {"b", 2}}

	asc := Of(items).SortBy(func(i item) int { return i.N }).Collect()
	if want := []item{{"a", 1}, {"b", 2}, {"c", 3}}; !reflect.DeepEqual(asc, want) {
		t.Errorf("SortBy() = %+v, want %+v", asc, want)
	}

	desc := Of(items).SortByDesc(func(i item) int { return i.N }).Collect()
	if want := []item{{"c", 3}, {"b", 2}, {"a", 1}}; !reflect.DeepEqual(desc, want) {
		t.Errorf("SortByDesc() = %+v, want %+v", desc, want)
	}

	// original slice must be untouched.
	if items[0].Name != "c" {
		t.Errorf("SortBy mutated the source slice: %+v", items)
	}
}

func TestSortBy_StableOnEqualKeys(t *testing.T) {
	type item struct {
		Key   int
		Label string // distinguishes otherwise-equal-key elements
	}
	items := []item{{1, "a"}, {2, "x"}, {1, "b"}, {2, "y"}, {1, "c"}}

	got := Of(items).SortBy(func(i item) int { return i.Key }).Collect()
	want := []item{{1, "a"}, {1, "b"}, {1, "c"}, {2, "x"}, {2, "y"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("SortBy() = %+v, want %+v (equal-key elements must keep original relative order)", got, want)
	}
}

func TestSortByCached_MatchesSortByAndCallsFnOnceEach(t *testing.T) {
	type item struct {
		Key   int
		Label string
	}
	items := []item{{3, "c"}, {1, "a"}, {2, "x"}, {1, "b"}}

	calls := 0
	keyFn := func(i item) int {
		calls++
		return i.Key
	}

	got := Of(items).SortByCached(keyFn).Collect()
	want := Of(items).SortBy(func(i item) int { return i.Key }).Collect()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("SortByCached() = %+v, want same result as SortBy() = %+v", got, want)
	}
	if calls != len(items) {
		t.Errorf("key fn called %d times, want exactly %d (once per element)", calls, len(items))
	}
}

func TestSortByDescCached(t *testing.T) {
	got := Of([]int{3, 1, 2}).SortByDescCached(func(n int) int { return n }).Collect()
	want := []int{3, 2, 1}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("SortByDescCached() = %v, want %v", got, want)
	}
}

func TestPartition(t *testing.T) {
	even, odd := Of([]int{1, 2, 3, 4, 5, 6}).Partition(func(n int) bool { return n%2 == 0 })
	if got, want := even.Collect(), []int{2, 4, 6}; !reflect.DeepEqual(got, want) {
		t.Errorf("Partition() matched = %v, want %v", got, want)
	}
	if got, want := odd.Collect(), []int{1, 3, 5}; !reflect.DeepEqual(got, want) {
		t.Errorf("Partition() unmatched = %v, want %v", got, want)
	}
}

func TestOfMap(t *testing.T) {
	m := map[string]int{"a": 1, "b": 2, "c": 3}
	got := OfMap(m).SortBy(func(p Pair[string, int]) string { return p.First }).Collect()
	want := []Pair[string, int]{{"a", 1}, {"b", 2}, {"c", 3}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("OfMap() = %+v, want %+v", got, want)
	}
}

func TestOfMap_Empty(t *testing.T) {
	got := OfMap(map[string]int{}).Collect()
	if len(got) != 0 {
		t.Errorf("OfMap(empty map) = %+v, want empty", got)
	}
}

func TestZip(t *testing.T) {
	tests := []struct {
		name string
		a    []string
		b    []int
		want []Pair[string, int]
	}{
		{"equal length", []string{"a", "b", "c"}, []int{1, 2, 3}, []Pair[string, int]{{"a", 1}, {"b", 2}, {"c", 3}}},
		{"a shorter", []string{"a"}, []int{1, 2, 3}, []Pair[string, int]{{"a", 1}}},
		{"b shorter", []string{"a", "b", "c"}, []int{1}, []Pair[string, int]{{"a", 1}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Zip(Of(tt.a), Of(tt.b)).Collect()
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Zip() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestFlatten(t *testing.T) {
	got := Flatten(Of([][]int{{1, 2}, {3}, {4, 5, 6}})).Collect()
	if want := []int{1, 2, 3, 4, 5, 6}; !reflect.DeepEqual(got, want) {
		t.Errorf("Flatten() = %v, want %v", got, want)
	}

	if got := Flatten(Of([][]int{})).Collect(); len(got) != 0 {
		t.Errorf("Flatten(empty) = %v, want []", got)
	}
}

func TestMinBy(t *testing.T) {
	type item struct {
		Name string
		N    int
	}
	items := []item{{"b", 2}, {"a", 1}, {"c", 3}}

	got := Of(items).MinBy(func(i item) int { return i.N })
	if got.IsNone() || got.MustGet().N != 1 {
		t.Errorf("MinBy() = %+v, want Some({a 1})", got)
	}

	if Of([]item{}).MinBy(func(i item) int { return i.N }).IsSome() {
		t.Error("MinBy() on empty Stream returned Some")
	}
}

func TestMaxBy(t *testing.T) {
	type item struct {
		Name string
		N    int
	}
	items := []item{{"b", 2}, {"a", 1}, {"c", 3}}

	got := Of(items).MaxBy(func(i item) int { return i.N })
	if got.IsNone() || got.MustGet().N != 3 {
		t.Errorf("MaxBy() = %+v, want Some({c 3})", got)
	}

	if Of([]item{}).MaxBy(func(i item) int { return i.N }).IsSome() {
		t.Error("MaxBy() on empty Stream returned Some")
	}
}

func TestToSeq_RoundTrips(t *testing.T) {
	got := Of([]int{1, 2, 3}).ToSeq().Collect()
	if want := []int{1, 2, 3}; !reflect.DeepEqual(got, want) {
		t.Errorf("ToSeq().Collect() = %v, want %v", got, want)
	}
}

func TestOfOption(t *testing.T) {
	if got := OfOption(adt.Some(42)).Collect(); !reflect.DeepEqual(got, []int{42}) {
		t.Errorf("OfOption(Some) = %v, want [42]", got)
	}
	if got := OfOption(adt.None[int]()).Collect(); len(got) != 0 {
		t.Errorf("OfOption(None) = %v, want []", got)
	}
}

func TestMinBy_TiesKeepFirst(t *testing.T) {
	type item struct {
		Name string
		N    int
	}
	got := Of([]item{{"a", 1}, {"b", 1}, {"c", 2}}).MinBy(func(i item) int { return i.N }).MustGet()
	if got.Name != "a" {
		t.Errorf("MinBy() tie = %q, want %q (first element must win)", got.Name, "a")
	}
}
