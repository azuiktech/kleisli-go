package async_test

import (
	"sync"
	"testing"

	"github.com/azuiktech/kleisli-go/async"
)

type counter struct{ n int }

func TestNewHandle_read_and_write(t *testing.T) {
	h := async.NewHandle(counter{})
	h.Write(func(c *counter) { c.n = 42 })
	got := h.Map(func(c counter) int { return c.n })
	if got != 42 {
		t.Fatalf("want 42, got %d", got)
	}
}

func TestHandle_copies_share_state(t *testing.T) {
	h1 := async.NewHandle(counter{})
	h2 := h1 // value copy — still points to the same Sync

	h1.Write(func(c *counter) { c.n = 99 })
	got := h2.Map(func(c counter) int { return c.n })
	if got != 99 {
		t.Fatalf("h2 should see h1's write: want 99, got %d", got)
	}
}

func TestHandle_mutate_returns_value(t *testing.T) {
	h := async.NewHandle(counter{n: 10})
	prev := h.Mutate(func(c *counter) int { old := c.n; c.n = 0; return old })
	if prev != 10 {
		t.Fatalf("want previous 10, got %d", prev)
	}
	if got := h.Map(func(c counter) int { return c.n }); got != 0 {
		t.Fatalf("want 0 after reset, got %d", got)
	}
}

func TestHandle_concurrent_writes(t *testing.T) {
	h := async.NewHandle(counter{})
	var wg sync.WaitGroup
	const N = 500
	wg.Add(N)
	for range N {
		go func() {
			defer wg.Done()
			h.Write(func(c *counter) { c.n++ })
		}()
	}
	wg.Wait()
	if got := h.Map(func(c counter) int { return c.n }); got != N {
		t.Fatalf("want %d, got %d", N, got)
	}
}

// Two struct types embedding the same Handle — the pimpl use case.
type adder   struct{ async.Handle[counter] }
type resetter struct{ async.Handle[counter] }

func (a adder)    Add()   { a.Write(func(c *counter) { c.n++ }) }
func (r resetter) Reset() { r.Write(func(c *counter) { c.n = 0 }) }

func TestHandle_multiple_types_share_state(t *testing.T) {
	h := async.NewHandle(counter{})
	add   := adder{h}
	reset := resetter{h}

	add.Add()
	add.Add()
	add.Add()

	if got := h.Map(func(c counter) int { return c.n }); got != 3 {
		t.Fatalf("want 3, got %d", got)
	}

	reset.Reset()

	if got := h.Map(func(c counter) int { return c.n }); got != 0 {
		t.Fatalf("want 0 after reset, got %d", got)
	}
}
