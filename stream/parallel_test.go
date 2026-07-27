package stream

import (
	"reflect"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/azuiktech/kleisli-go/async"
)

func TestToPipe_FromPipe_RoundTrips(t *testing.T) {
	got := FromPipe(Of([]int{1, 2, 3}).ToPipe()).Collect()
	want := []int{1, 2, 3}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("FromPipe(ToPipe()).Collect() = %v, want %v", got, want)
	}
}

func TestRegion_RunsAnyPipeOperation(t *testing.T) {
	// Region isn't just for Parallel — this exercises Buffer, an
	// operation Stream.Parallel's own sugar doesn't cover.
	got := Of([]int{1, 2, 3}).
		Region(func(p async.Pipe[int]) async.Pipe[int] {
			return p.Buffer(2)
		}).
		Collect()
	want := []int{1, 2, 3}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Region(Buffer).Collect() = %v, want %v", got, want)
	}
}

func TestStream_Parallel_ProcessesEveryItem(t *testing.T) {
	got := Of([]int{1, 2, 3, 4, 5}).Parallel(3, func(n int) int { return n * n }).Collect()
	sort.Ints(got)
	want := []int{1, 4, 9, 16, 25}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Parallel().Collect() (sorted) = %v, want %v", got, want)
	}
}

func TestStream_Parallel_WorkersRunConcurrently(t *testing.T) {
	const n = 4
	var wg sync.WaitGroup
	wg.Add(n)
	rendezvous := func(x int) int {
		wg.Done()
		wg.Wait()
		return x
	}

	done := make(chan []int, 1)
	go func() { done <- Of([]int{1, 2, 3, 4}).Parallel(n, rendezvous).Collect() }()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Parallel never finished — workers did not run concurrently")
	}
}

func TestRegion_OrderedParallel_PreservesInputOrder(t *testing.T) {
	// Region + Enumerate/Ordered restores order Parallel alone doesn't
	// guarantee — the escape hatch Stream.Parallel's own doc points to.
	items := []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}
	scramble := func(item async.Indexed[int]) async.Indexed[int] {
		time.Sleep(time.Duration(10-item.Value) * time.Millisecond)
		return item
	}

	got := Of(items).
		Region(func(p async.Pipe[int]) async.Pipe[int] {
			return async.Ordered(async.Enumerate(p).Parallel(4, scramble))
		}).
		Collect()

	if !reflect.DeepEqual(got, items) {
		t.Errorf("ordered Region.Collect() = %v, want %v", got, items)
	}
}
