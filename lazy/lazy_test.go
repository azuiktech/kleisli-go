package lazy_test

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/azuiktech/kleisli-go/lazy"
)

func TestLazy_EvaluatesOnceAndThreadSafe(t *testing.T) {
	var count int64
	l := lazy.New(func() int {
		atomic.AddInt64(&count, 1)
		return 42
	})

	var wg sync.WaitGroup
	results := make([]int, 100)
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			results[idx] = l.Get()
		}(i)
	}
	wg.Wait()

	if atomic.LoadInt64(&count) != 1 {
		t.Errorf("expected evaluation count to be 1, got %d", count)
	}
	for i, res := range results {
		if res != 42 {
			t.Errorf("results[%d] got %d, want 42", i, res)
		}
	}
}

func TestLazy_ZeroValueSafety(t *testing.T) {
	var l lazy.Lazy[int]
	if got := l.Get(); got != 0 {
		t.Errorf("zero value Lazy.Get() got %d, want 0", got)
	}
}

func TestLazy_FromErr(t *testing.T) {
	var count int64
	errDummy := errors.New("dummy error")

	lErr := lazy.FromErr(func() (string, error) {
		atomic.AddInt64(&count, 1)
		return "", errDummy
	})

	res1 := lErr.Get()
	res2 := lErr.Get()

	if atomic.LoadInt64(&count) != 1 {
		t.Errorf("expected evaluation count to be 1, got %d", count)
	}
	_, err1 := res1.Unwrap()
	_, err2 := res2.Unwrap()
	if !res1.IsErr() || err1 != errDummy {
		t.Errorf("res1 unexpected: %v", res1)
	}
	if !res2.IsErr() || err2 != errDummy {
		t.Errorf("res2 unexpected: %v", res2)
	}

}

func TestLazy_MapAndFlatMapDeferred(t *testing.T) {
	var evalCount int64
	l1 := lazy.New(func() int {
		atomic.AddInt64(&evalCount, 1)
		return 10
	})

	var mapCount int64
	l2 := l1.Map(func(x int) int {
		atomic.AddInt64(&mapCount, 1)
		return x * 2
	})

	var flatMapCount int64
	l3 := l2.FlatMap(func(x int) lazy.Lazy[string] {
		atomic.AddInt64(&flatMapCount, 1)
		return lazy.New(func() string { return "done" })
	})

	// Before Get(), nothing should be evaluated
	if atomic.LoadInt64(&evalCount) != 0 || atomic.LoadInt64(&mapCount) != 0 || atomic.LoadInt64(&flatMapCount) != 0 {
		t.Fatalf("transformations executed prematurely before Get()")
	}

	val := l3.Get()
	if val != "done" {
		t.Errorf("l3.Get() got %q, want %q", val, "done")
	}
	if atomic.LoadInt64(&evalCount) != 1 || atomic.LoadInt64(&mapCount) != 1 || atomic.LoadInt64(&flatMapCount) != 1 {
		t.Errorf("evaluation counts mismatched: eval=%d, map=%d, flatMap=%d", evalCount, mapCount, flatMapCount)
	}
}

func TestLazy_ToOption(t *testing.T) {
	l := lazy.New(func() string { return "hello" })
	opt := l.ToOption()
	if !opt.IsSome() || opt.MustGet() != "hello" {
		t.Errorf("ToOption failed: %v", opt)
	}
}

func TestMemoize(t *testing.T) {
	var calls int64
	fn := func(k string) int {
		atomic.AddInt64(&calls, 1)
		return len(k)
	}
	memoized := lazy.Memoize(fn)

	var wg sync.WaitGroup
	// 50 goroutines for key "apple", 50 goroutines for key "banana"
	for i := 0; i < 50; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			if got := memoized("apple"); got != 5 {
				t.Errorf("memoized(\"apple\") got %d, want 5", got)
			}
		}()
		go func() {
			defer wg.Done()
			if got := memoized("banana"); got != 6 {
				t.Errorf("memoized(\"banana\") got %d, want 6", got)
			}
		}()
	}
	wg.Wait()

	if atomic.LoadInt64(&calls) != 2 {
		t.Errorf("expected fn to be called 2 times (once per key), got %d", calls)
	}
}

func TestMemoizeErr(t *testing.T) {
	var calls int64
	errFail := errors.New("fail")
	fn := func(k int) (int, error) {
		atomic.AddInt64(&calls, 1)
		if k < 0 {
			return 0, errFail
		}
		return k * 10, nil
	}
	memoized := lazy.MemoizeErr(fn)

	val1, err1 := memoized(-1)
	val2, err2 := memoized(-1)

	if val1 != 0 || err1 != errFail || val2 != 0 || err2 != errFail {
		t.Errorf("memoizedErr negative key failed")
	}

	val3, err3 := memoized(5)
	if val3 != 50 || err3 != nil {
		t.Errorf("memoizedErr positive key failed: val=%d, err=%v", val3, err3)
	}

	if atomic.LoadInt64(&calls) != 2 {
		t.Errorf("expected 2 calls (for key -1 and 5), got %d", calls)
	}
}
