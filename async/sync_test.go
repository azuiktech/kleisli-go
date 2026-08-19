package async_test

import (
	"sync"
	"testing"

	"github.com/azuiktech/kleisli-go/async"
)

func TestSync_WriteRead(t *testing.T) {
	s := async.Of(0)
	s.Write(func(n *int) { *n = 42 })
	var got int
	s.Read(func(n *int) { got = *n })
	if got != 42 {
		t.Fatalf("want 42, got %d", got)
	}
}

func TestSync_ConcurrentWrites(t *testing.T) {
	s := async.Of(0)
	var wg sync.WaitGroup
	for range 1000 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.Write(func(n *int) { *n++ })
		}()
	}
	wg.Wait()
	var got int
	s.Read(func(n *int) { got = *n })
	if got != 1000 {
		t.Fatalf("want 1000, got %d", got)
	}
}

func TestRead_ReturnsValue(t *testing.T) {
	s := async.Of(42)
	got := async.Read(&s, func(n *int) int { return *n })
	if got != 42 {
		t.Fatalf("want 42, got %d", got)
	}
}

func TestWrite_ReturnsValue(t *testing.T) {
	s := async.Of(0)
	prev := async.Write(&s, func(n *int) int { old := *n; *n = 99; return old })
	if prev != 0 {
		t.Fatalf("want previous value 0, got %d", prev)
	}
	if got := async.Read(&s, func(n *int) int { return *n }); got != 99 {
		t.Fatalf("want 99 after write, got %d", got)
	}
}

func TestSync_ConcurrentReads(t *testing.T) {
	s := async.Of(99)
	var wg sync.WaitGroup
	for range 100 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.Read(func(n *int) {
				if *n != 99 {
					t.Errorf("concurrent read saw %d, want 99", *n)
				}
			})
		}()
	}
	wg.Wait()
}
