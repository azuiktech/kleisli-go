package async

import (
	"context"
	"reflect"
	"sort"
	"sync"
	"testing"
	"time"

	"golang.org/x/time/rate"
)

func TestFrom_ProducesItemsInOrder(t *testing.T) {
	got := From([]int{1, 2, 3, 4}).Collect()
	want := []int{1, 2, 3, 4}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("From().Collect() = %v, want %v", got, want)
	}
}

func TestGo_AwaitReturnsFnsResult(t *testing.T) {
	got := Go(func() int { return 42 }).Await()
	if got != 42 {
		t.Errorf("Go(fn).Await() = %d, want 42", got)
	}
}

func TestMap_TransformsEveryItem_PreservesOrder(t *testing.T) {
	got := From([]int{1, 2, 3}).Map(func(n int) int { return n * 2 }).Collect()
	want := []int{2, 4, 6}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Map() = %v, want %v", got, want)
	}
}

func TestParallel_ProcessesEveryItem(t *testing.T) {
	got := From([]int{1, 2, 3, 4, 5}).Parallel(3, func(n int) int { return n * n }).Collect()
	sort.Ints(got)
	want := []int{1, 4, 9, 16, 25}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Parallel().Collect() (sorted) = %v, want %v", got, want)
	}
}

func TestParallel_WorkersRunConcurrently(t *testing.T) {
	// Each worker blocks until it observes every one of the n workers has
	// started — this deadlocks forever under a sequential (non-pooled)
	// implementation, proving Parallel genuinely runs workers at once.
	const n = 4
	var wg sync.WaitGroup
	wg.Add(n)
	rendezvous := func(x int) int {
		wg.Done()
		wg.Wait()
		return x
	}

	done := make(chan []int, 1)
	go func() { done <- From([]int{1, 2, 3, 4}).Parallel(n, rendezvous).Collect() }()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Parallel never finished — workers did not run concurrently")
	}
}

func TestBuffer_PreservesItemsAndOrder(t *testing.T) {
	got := From([]int{1, 2, 3}).Buffer(2).Collect()
	want := []int{1, 2, 3}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Buffer(2).Collect() = %v, want %v", got, want)
	}
}

func TestFork_EveryBranchSeesEveryItem(t *testing.T) {
	branches := From([]int{1, 2, 3}).Fork(2)
	if len(branches) != 2 {
		t.Fatalf("Fork(2) returned %d branches, want 2", len(branches))
	}

	var a, b []int
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); a = branches[0].Collect() }()
	go func() { defer wg.Done(); b = branches[1].Collect() }()
	wg.Wait()

	want := []int{1, 2, 3}
	if !reflect.DeepEqual(a, want) {
		t.Errorf("branch 0 = %v, want %v", a, want)
	}
	if !reflect.DeepEqual(b, want) {
		t.Errorf("branch 1 = %v, want %v", b, want)
	}
}

func TestMerge_CombinesEveryItemFromEverySource(t *testing.T) {
	got := Merge(From([]int{1, 2}), From([]int{3, 4}), From([]int{5})).Collect()
	sort.Ints(got)
	want := []int{1, 2, 3, 4, 5}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Merge().Collect() (sorted) = %v, want %v", got, want)
	}
}

func TestRateLimit_PassesEveryItemThrough(t *testing.T) {
	lim := rate.NewLimiter(rate.Inf, 0) // unbounded — correctness only, not timing
	got := From([]int{1, 2, 3}).RateLimit(context.Background(), lim).Collect()
	want := []int{1, 2, 3}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("RateLimit().Collect() = %v, want %v", got, want)
	}
}

func TestRateLimit_StopsEarlyOnContextCancellation(t *testing.T) {
	lim := rate.NewLimiter(rate.Every(time.Hour), 1) // first token free, then a very long wait
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already cancelled — the second item's Wait must fail immediately

	got := From([]int{1, 2, 3}).RateLimit(ctx, lim).Collect()
	if len(got) > 1 {
		t.Errorf("RateLimit with a cancelled ctx let %d items through, want at most 1", len(got))
	}
}

func TestWindow_BatchesIntoFixedChunks(t *testing.T) {
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
			got := Window(From(tt.items), tt.n).Collect()
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Window(%d) = %v, want %v", tt.n, got, tt.want)
			}
		})
	}
}

func TestEnumerate_TagsWithPosition(t *testing.T) {
	got := Enumerate(From([]string{"a", "b", "c"})).Collect()
	want := []Indexed[string]{{0, "a"}, {1, "b"}, {2, "c"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Enumerate().Collect() = %+v, want %+v", got, want)
	}
}

func TestOrdered_RestoresOrderAfterParallel(t *testing.T) {
	items := []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}

	// Parallel scrambles arrival order: earlier items sleep longer, so
	// they finish last unless Ordered puts them back in place.
	scramble := func(item Indexed[int]) Indexed[int] {
		time.Sleep(time.Duration(10-item.Value) * time.Millisecond)
		return Indexed[int]{Index: item.Index, Value: item.Value}
	}

	got := Ordered(Enumerate(From(items)).Parallel(4, scramble)).Collect()
	if !reflect.DeepEqual(got, items) {
		t.Errorf("Ordered().Collect() = %v, want %v (original order restored)", got, items)
	}
}

func TestReduce_FoldsEveryItem(t *testing.T) {
	got := From([]int{1, 2, 3, 4}).Reduce(0, func(acc, n int) int { return acc + n })
	if got != 10 {
		t.Errorf("Reduce() = %d, want 10", got)
	}
}

func TestEach_CallsFnOnEveryItemInOrder(t *testing.T) {
	// Each drains from the calling goroutine — no concurrency of its own
	// — so order matches From's own in-order production exactly.
	var got []int
	From([]int{1, 2, 3}).Each(func(n int) { got = append(got, n) })
	want := []int{1, 2, 3}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Each() collected %v, want %v", got, want)
	}
}
