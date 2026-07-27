package tacit

import (
	"math"
	"testing"

	"github.com/azuiktech/kleisli-go/stream"
)

func TestThen_ComposesLeftToRight(t *testing.T) {
	double := Fn[int, int](func(x int) int { return x * 2 })
	addOne := Fn[int, int](func(x int) int { return x + 1 })

	got := double.Then(addOne)(5) // (5*2)+1
	if got != 11 {
		t.Errorf("double.Then(addOne)(5) = %d, want 11", got)
	}
}

func TestFork_AppliesBothAndCombines(t *testing.T) {
	square := Fn[int, int](func(x int) int { return x * x })
	negate := Fn[int, int](func(x int) int { return -x })
	add := Fn2[int, int, int](func(a, b int) int { return a + b })

	got := add.Fork(square, negate)(5) // (5*5) + (-5)
	if got != 20 {
		t.Errorf("add.Fork(square, negate)(5) = %d, want 20", got)
	}
}

func TestIdentity(t *testing.T) {
	if got := Identity(42); got != 42 {
		t.Errorf("Identity(42) = %d, want 42", got)
	}
}

func TestFork_WithIdentity_IsDeviationFromDerived(t *testing.T) {
	// Subtract.Fork(Identity, g)(x) = x - g(x) — the shape StdDev itself
	// relies on, isolated and checked on its own.
	subtract := Fn2[int, int, int](func(a, b int) int { return a - b })
	half := Fn[int, int](func(x int) int { return x / 2 })

	got := subtract.Fork(Identity, half)(10) // 10 - 5
	if got != 5 {
		t.Errorf("subtract.Fork(Identity, half)(10) = %d, want 5", got)
	}
}

// TestStdDev is the design's actual proof: standard deviation, built
// entirely by composition, no named intermediate argument anywhere in
// StdDev's own definition — the array-language original this package
// exists to make expressible in Go:
// StdDev ← √ ∘ (+´ ÷ ≠) ∘ (×˜) -⟜(+´ ÷ ≠)
func TestStdDev(t *testing.T) {
	mean := Fn[[]float64, float64](func(xs []float64) float64 {
		return stream.Of(xs).Reduce(0.0, func(a, b float64) float64 { return a + b }) / float64(len(xs))
	})
	subtract := Fn2[[]float64, float64, []float64](func(xs []float64, m float64) []float64 {
		return stream.Of(xs).Map(func(x float64) float64 { return x - m }).Collect()
	})
	square := Fn[[]float64, []float64](func(xs []float64) []float64 {
		return stream.Of(xs).Map(func(x float64) float64 { return x * x }).Collect()
	})
	sqrt := Fn[float64, float64](math.Sqrt)

	stdDev := subtract.Fork(Identity, mean).Then(square).Then(mean).Then(sqrt)

	got := stdDev([]float64{2, 4, 4, 4, 5, 5, 7, 9})
	if got != 2 {
		t.Errorf("stdDev(...) = %v, want 2", got)
	}
}

// TestFn_ComposesDirectlyIntoStreamMap proves the interop claim: a
// composed Fn needs no conversion to serve as the fn argument to
// stream.Map (or, by the same Go assignability rule, async.Pipe.Parallel
// and anything else shaped func(T) U).
func TestFn_ComposesDirectlyIntoStreamMap(t *testing.T) {
	double := Fn[int, int](func(x int) int { return x * 2 })
	addOne := Fn[int, int](func(x int) int { return x + 1 })
	composed := double.Then(addOne)

	got := stream.Of([]int{1, 2, 3}).Map(composed).Collect()
	want := []int{3, 5, 7}
	for i, v := range want {
		if got[i] != v {
			t.Errorf("stream.Of(...).Map(composed).Collect() = %v, want %v", got, want)
			break
		}
	}
}
