// Package stream provides a generic Stream[T] type for functional pipeline
// operations over in-memory slices.
//
// Map, FlatMap, Reduce, GroupBy, and ToMap use Go 1.27 generic methods,
// enabling type-changing transformations via method chaining without free
// functions or wrapper types.
//
//	totals := stream.Of(invoices).
//	    Filter(func(inv Invoice) bool { return inv.Status == Unpaid }).
//	    GroupBy[string](func(inv Invoice) string { return inv.ClientID })
package stream

// Stream wraps a slice for lazy-style pipeline operations.
// All operations are eager (evaluated immediately).
type Stream[T any] struct {
	items []T
}

// Of wraps a slice in a Stream. The original slice is not copied.
func Of[T any](items []T) Stream[T] { return Stream[T]{items: items} }

// Empty returns a Stream with no elements.
func Empty[T any]() Stream[T] { return Stream[T]{} }

// Filter returns a Stream containing only elements for which fn returns true.
func (s Stream[T]) Filter(fn func(T) bool) Stream[T] {
	out := make([]T, 0, len(s.items))
	for _, v := range s.items {
		if fn(v) {
			out = append(out, v)
		}
	}
	return Stream[T]{items: out}
}

// Each calls fn on every element. Returns s unchanged for chaining.
func (s Stream[T]) Each(fn func(T)) Stream[T] {
	for _, v := range s.items {
		fn(v)
	}
	return s
}

// Any reports whether fn returns true for at least one element.
func (s Stream[T]) Any(fn func(T) bool) bool {
	for _, v := range s.items {
		if fn(v) {
			return true
		}
	}
	return false
}

// All reports whether fn returns true for every element.
func (s Stream[T]) All(fn func(T) bool) bool {
	for _, v := range s.items {
		if !fn(v) {
			return false
		}
	}
	return true
}

// First returns the first element satisfying fn, or (zero, false).
func (s Stream[T]) First(fn func(T) bool) (T, bool) {
	for _, v := range s.items {
		if fn(v) {
			return v, true
		}
	}
	var zero T
	return zero, false
}

// Last returns the last element satisfying fn, or (zero, false).
func (s Stream[T]) Last(fn func(T) bool) (T, bool) {
	var (
		found T
		ok    bool
	)
	for _, v := range s.items {
		if fn(v) {
			found, ok = v, true
		}
	}
	return found, ok
}

// Take returns a Stream containing at most n leading elements.
func (s Stream[T]) Take(n int) Stream[T] {
	if n >= len(s.items) {
		return s
	}
	return Stream[T]{items: s.items[:n]}
}

// Skip returns a Stream with the first n elements removed.
func (s Stream[T]) Skip(n int) Stream[T] {
	if n >= len(s.items) {
		return Empty[T]()
	}
	return Stream[T]{items: s.items[n:]}
}

// Reverse returns a Stream with elements in reversed order.
func (s Stream[T]) Reverse() Stream[T] {
	out := make([]T, len(s.items))
	for i, v := range s.items {
		out[len(s.items)-1-i] = v
	}
	return Stream[T]{items: out}
}

// Len returns the number of elements.
func (s Stream[T]) Len() int { return len(s.items) }

// Collect returns the underlying slice.
func (s Stream[T]) Collect() []T { return s.items }

// --- Go 1.27 generic methods ---

// Map transforms each element into type U.
//
// Requires Go 1.27 (generic method type parameter).
func (s Stream[T]) Map[U any](fn func(T) U) Stream[U] {
	out := make([]U, len(s.items))
	for i, v := range s.items {
		out[i] = fn(v)
	}
	return Stream[U]{items: out}
}

// FlatMap maps each element to a slice of U and concatenates the results.
//
// Requires Go 1.27 (generic method type parameter).
func (s Stream[T]) FlatMap[U any](fn func(T) []U) Stream[U] {
	out := make([]U, 0, len(s.items))
	for _, v := range s.items {
		out = append(out, fn(v)...)
	}
	return Stream[U]{items: out}
}

// Reduce folds every element into an accumulator of type U.
//
// Requires Go 1.27 (generic method type parameter).
func (s Stream[T]) Reduce[U any](initial U, fn func(U, T) U) U {
	acc := initial
	for _, v := range s.items {
		acc = fn(acc, v)
	}
	return acc
}

// GroupBy partitions elements into a map keyed by K.
// Elements with the same key are collected in insertion order.
//
// Requires Go 1.27 (generic method type parameter).
func (s Stream[T]) GroupBy[K comparable](fn func(T) K) map[K][]T {
	out := make(map[K][]T)
	for _, v := range s.items {
		k := fn(v)
		out[k] = append(out[k], v)
	}
	return out
}

// ToMap converts the Stream into a map[K]V by extracting a key and value
// from each element. Later elements overwrite earlier ones on key collision.
//
// Requires Go 1.27 (generic method type parameter).
func (s Stream[T]) ToMap[K comparable, V any](fn func(T) (K, V)) map[K]V {
	out := make(map[K]V, len(s.items))
	for _, v := range s.items {
		k, val := fn(v)
		out[k] = val
	}
	return out
}
