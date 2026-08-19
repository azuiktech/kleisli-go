package algo

import "slices"

// FindRotated binary-searches two sorted partitions of a rotated sequence.
// first is the older/lower partition (s[pivot:]); second is the newer/higher
// partition (s[:pivot]). Logical order is first → second.
//
// Returns a logical index n:
//
//	n ∈ [0, len(first))              — found in first at offset n
//	n ∈ [len(first), len(first)+len(second)) — found in second at offset n-len(first)
//	len(first)+len(second)           — not found
//
// Aligns directly with ds.RingBuffer.Segments():
//
//	first, second := ring.Segments()
//	n := algo.FindRotated(first, second, val, comp)
func FindRotated[T, V any](first, second []T, val V, comp func(T, V) int) int {
	if i, ok := slices.BinarySearchFunc(first, val, comp); ok {
		return i
	}
	if i, ok := slices.BinarySearchFunc(second, val, comp); ok {
		return len(first) + i
	}
	return len(first) + len(second)
}

// After returns elements strictly after logical index n in circular order.
// n must be a valid index returned by FindRotated (not the not-found sentinel).
//
// Found in first  (n < len(first)):  returns (first[n+1:], second) — bitonic.
// Found in second (n ≥ len(first)):  returns (second[m+1:], nil)   — monotonic.
func After[T any](first, second []T, n int) ([]T, []T) {
	nf := len(first)
	if n < nf {
		return first[n+1:], second
	}
	return second[n-nf+1:], nil
}

// Before returns elements strictly before logical index n in circular order.
// n must be a valid index returned by FindRotated (not the not-found sentinel).
//
// Found in first  (n < len(first)):  returns (first[:n], nil)      — monotonic.
// Found in second (n ≥ len(first)):  returns (first, second[:m])   — bitonic.
func Before[T any](first, second []T, n int) ([]T, []T) {
	nf := len(first)
	if n < nf {
		return first[:n], nil
	}
	m := n - nf
	if m == 0 {
		return first, nil
	}
	return first, second[:m]
}

// FindRotatedWithPivot searches s[pivot:] then s[:pivot] for val and returns
// a physical index into s. Returns len(s) when not found.
func FindRotatedWithPivot[T, V any](s []T, pivot int, val V, comp func(T, V) int) int {
	n := FindRotated(s[pivot:], s[:pivot], val, comp)
	total := len(s)
	if n == total {
		return total
	}
	nf := total - pivot
	if n < nf {
		return pivot + n
	}
	return n - nf
}

// AfterWithPivot returns elements strictly after physical index pos in
// circular order, treating s[pivot:] as the older partition.
func AfterWithPivot[T any](s []T, pos, pivot int) ([]T, []T) {
	n := pos - pivot
	if n < 0 {
		n += len(s)
	}
	return After(s[pivot:], s[:pivot], n)
}

// BeforeWithPivot returns elements strictly before physical index pos in
// circular order, treating s[pivot:] as the older partition.
func BeforeWithPivot[T any](s []T, pos, pivot int) ([]T, []T) {
	n := pos - pivot
	if n < 0 {
		n += len(s)
	}
	return Before(s[pivot:], s[:pivot], n)
}
