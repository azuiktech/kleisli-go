package async

import (
	"context"
	"iter"
	"reflect"
	"sort"
	"sync"
	"testing"
	"time"

	"golang.org/x/time/rate"

	"github.com/azuiktech/kleisli-go/adt"
)

func TestFrom_ProducesItemsInOrder(t *testing.T) {
	got := From([]int{1, 2, 3, 4}).Collect()
	want := []int{1, 2, 3, 4}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("From().Collect() = %v, want %v", got, want)
	}
}

func TestGo_AwaitReturnsFnsResult(t *testing.T) {
	o := Go(func() int { return 42 }).Await()
	if !o.IsSome() || o.MustGet() != 42 {
		t.Errorf("Go(fn).Await() = %v, want Some(42)", o)
	}
}

func TestAwait_ReturnNoneWhenDrained(t *testing.T) {
	if From([]int{}).Await().IsSome() {
		t.Error("Await on empty pipe should return None")
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

func TestFork_UnreadBranchStallsSharedPump(t *testing.T) {
	// Fork's doc: every item is sent to branch 0, then branch 1, in order,
	// via one shared pump. A branch nobody reads therefore blocks that
	// pump — and every sibling, including branches that are being read —
	// from making further progress.
	ctx, cancel := context.WithCancelCause(context.Background())
	defer cancel(nil)

	branches := FromContext(ctx, []int{1, 2, 3}).Fork(2)
	if len(branches) != 2 {
		t.Fatalf("Fork(2) returned %d branches, want 2", len(branches))
	}

	firstItem := make(chan int, 1)
	go branches[0].Each(func(n int) { firstItem <- n })

	select {
	case n := <-firstItem:
		if n != 1 {
			t.Fatalf("branch 0 first item = %d, want 1", n)
		}
	case <-time.After(time.Second):
		t.Fatal("branch 0 never received its first item")
	}

	// Branch 1 is never read. The pump must now be stuck delivering item
	// 1 to branch 1, so branch 0 must never see a second item either.
	select {
	case n := <-firstItem:
		t.Fatalf("branch 0 received a second item (%d); the unread branch 1 should have stalled the shared pump", n)
	case <-time.After(200 * time.Millisecond):
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

func naturalNumbers() iter.Seq[int] {
	return func(yield func(int) bool) {
		for i := 0; ; i++ {
			if !yield(i) {
				return
			}
		}
	}
}

func TestMerge_OnlyFirstPipesContextGovernsCancellation(t *testing.T) {
	// Merge's doc: the first pipe's context governs all merged goroutines
	// — cancelling any other source pipe's own context must not affect
	// the merged output, only cancelling pipes[0]'s context does.
	ctx1, cancel1 := context.WithCancel(context.Background())
	ctx2, cancel2 := context.WithCancel(context.Background())
	defer cancel1()
	defer cancel2()

	p1 := FromIterContext(ctx1, naturalNumbers())
	p2 := FromIterContext(ctx2, naturalNumbers())
	merged := Merge(p1, p2)

	// A persistent reader is required: Each's callback would otherwise
	// block a send once the test stops actively consuming, which would
	// mask the very termination this test checks for. progress carries a
	// non-blocking heartbeat for each item actually consumed.
	received := make(chan int)
	progress := make(chan struct{}, 1)
	done := make(chan struct{})
	go func() {
		defer close(done)
		merged.Each(func(n int) { received <- n })
	}()
	go func() {
		for range received {
			select {
			case progress <- struct{}{}:
			default:
			}
		}
	}()

	awaitProgress := func(timeout time.Duration) bool {
		select {
		case <-progress:
			return true
		case <-time.After(timeout):
			return false
		}
	}

	for i := 0; i < 4; i++ {
		if !awaitProgress(time.Second) {
			t.Fatal("merged pipe produced no items before any cancellation")
		}
	}

	cancel2() // cancel a non-first pipe's own context

	for i := 0; i < 4; i++ {
		if !awaitProgress(time.Second) {
			t.Fatal("merged output stalled after cancelling a non-first pipe's context")
		}
	}

	cancel1() // cancel pipes[0]'s context — the one Merge actually uses

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("merged output did not terminate after cancelling pipes[0]'s context")
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
	want := []adt.Indexed[string]{{Index: 0, Value: "a"}, {Index: 1, Value: "b"}, {Index: 2, Value: "c"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Enumerate().Collect() = %+v, want %+v", got, want)
	}
}

func TestOrdered_RestoresOrderAfterParallel(t *testing.T) {
	items := []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}

	// Parallel scrambles arrival order: earlier items sleep longer, so
	// they finish last unless Ordered puts them back in place.
	scramble := func(item adt.Indexed[int]) adt.Indexed[int] {
		time.Sleep(time.Duration(10-item.Value) * time.Millisecond)
		return adt.Indexed[int]{Index: item.Index, Value: item.Value}
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

func TestFromIter_ProducesItemsInOrder(t *testing.T) {
	source := func(yield func(int) bool) {
		for _, v := range []int{1, 2, 3} {
			if !yield(v) {
				return
			}
		}
	}
	got := FromIter(source).Collect()
	want := []int{1, 2, 3}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("FromIter().Collect() = %v, want %v", got, want)
	}
}

func TestFromIter_EarlyStop(t *testing.T) {
	produced := 0
	source := func(yield func(int) bool) {
		for i := 1; i <= 10; i++ {
			produced++
			if !yield(i) {
				return
			}
		}
	}
	got := FromIter(source).Parallel(1, func(n int) int { return n }).Collect()
	_ = got
	// Can't assert exact produced count without Take, but confirm it ran at all.
	if produced == 0 {
		t.Error("FromIter source was never pulled")
	}
}

func TestParallel_PanicsOnZeroWorkers(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("Parallel(0) should panic")
		}
	}()
	From([]int{1}).Parallel(0, func(n int) int { return n })
}

func TestWindow_PanicsOnZeroSize(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("Window(p, 0) should panic")
		}
	}()
	Window(From([]int{1}), 0)
}

func TestOrderedN_StopsWhenBufferExceeded(t *testing.T) {
	// Items 1,2,3 arrive out of order — index 0 never arrives so 1 and 2
	// pile up. With maxPending=1, the pipeline should stop before collecting all.
	p := FromContext(context.Background(), []adt.Indexed[int]{
		{Index: 1, Value: 10},
		{Index: 2, Value: 20},
		{Index: 3, Value: 30},
	})
	got := OrderedN(p, 1).Collect()
	// Index 0 is absent; after 1 item accumulates in pending, OrderedN stops.
	if len(got) > 0 {
		t.Errorf("OrderedN with exceeded buffer should produce no output, got %v", got)
	}
}
