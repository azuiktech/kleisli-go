package stream

import (
	"testing"
)

func TestNumberStream_SumProductMeanMinMax(t *testing.T) {
	ns := Of([]int{1, 2, 3, 4}).MapToNumber(func(n int) int { return n })

	if got := ns.Sum(); got != 10 {
		t.Errorf("Sum() = %d, want 10", got)
	}
	if got := ns.Product(); got != 24 {
		t.Errorf("Product() = %d, want 24", got)
	}
	if mean := ns.Mean(); mean.IsNone() || mean.MustGet() != 2.5 {
		t.Errorf("Mean() = %v, want Some(2.5)", mean)
	}
	if got := ns.Min(); got.IsNone() || got.MustGet() != 1 {
		t.Errorf("Min() = %v, want Some(1)", got)
	}
	if got := ns.Max(); got.IsNone() || got.MustGet() != 4 {
		t.Errorf("Max() = %v, want Some(4)", got)
	}
}

func TestNumberStream_Mean_EmptyIsNone(t *testing.T) {
	mean := Of([]int{}).MapToNumber(func(n int) int { return n }).Mean()
	if mean.IsSome() {
		t.Errorf("Mean() on empty Stream = %v, want None", mean)
	}
}

func TestNumberSeq_SumProductMeanMinMax(t *testing.T) {
	ns := SeqOf(1, 2, 3, 4).MapToNumber(func(n int) int { return n })

	if got := ns.Sum(); got != 10 {
		t.Errorf("Sum() = %d, want 10", got)
	}
	if got := ns.Product(); got != 24 {
		t.Errorf("Product() = %d, want 24", got)
	}
	if mean := ns.Mean(); mean.IsNone() || mean.MustGet() != 2.5 {
		t.Errorf("Mean() = %v, want Some(2.5)", mean)
	}
	if got := ns.Min(); got.IsNone() || got.MustGet() != 1 {
		t.Errorf("Min() = %v, want Some(1)", got)
	}
	if got := ns.Max(); got.IsNone() || got.MustGet() != 4 {
		t.Errorf("Max() = %v, want Some(4)", got)
	}
}

func TestNumberSeq_Mean_EmptyIsNone(t *testing.T) {
	mean := SeqOf[int]().MapToNumber(func(n int) int { return n }).Mean()
	if mean.IsSome() {
		t.Errorf("Mean() on empty Seq = %v, want None", mean)
	}
}
