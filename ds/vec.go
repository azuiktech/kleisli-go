package ds

import (
	"iter"
	"slices"

	"github.com/azuiktech/kleisli-go/adt"
)

// Vec is an encapsulated generic dynamic array data structure.
// Protected by noCopy marker to prevent value copying and slice sharing.
type Vec[T any] struct {
	_ noCopy
	data []T
}

// NewVec constructs an initialized Vec containing the given items.
func NewVec[T any](items ...T) *Vec[T] {
	v := &Vec[T]{data: make([]T, len(items))}
	copy(v.data, items)
	return v
}

// VecOf constructs an initialized Vec containing the given items.
func VecOf[T any](items ...T) *Vec[T] {
	return NewVec(items...)
}

// VecWithCapacity constructs an empty Vec with preallocated capacity.
func VecWithCapacity[T any](capacity int) *Vec[T] {
	return &Vec[T]{data: make([]T, 0, capacity)}
}

// VecFromSlice constructs a Vec containing all elements from slice.
func VecFromSlice[T any](slice []T) *Vec[T] {
	return NewVec(slice...)
}

// VecFromSeq constructs a Vec by consuming an iter.Seq[T].
func VecFromSeq[T any](seq iter.Seq[T]) *Vec[T] {
	v := &Vec[T]{data: make([]T, 0)}
	for item := range seq {
		v.Push(item)
	}
	return v
}

// Push appends item to the end of v in place.
func (v *Vec[T]) Push(item T) {
	v.data = append(v.data, item)
}

// PushAll appends all items to the end of v in place.
func (v *Vec[T]) PushAll(items ...T) {
	v.data = append(v.data, items...)
}

// Pop removes and returns the last element from v, or None if empty.
func (v *Vec[T]) Pop() adt.Option[T] {
	if v == nil || len(v.data) == 0 {
		return adt.None[T]()
	}
	lastIdx := len(v.data) - 1
	item := v.data[lastIdx]
	var zero T
	v.data[lastIdx] = zero
	v.data = v.data[:lastIdx]
	return adt.Some(item)
}

// Get returns Some(element) at index if within bounds [0, Len()), or None.
func (v *Vec[T]) Get(index int) adt.Option[T] {
	if v == nil || index < 0 || index >= len(v.data) {
		return adt.None[T]()
	}
	return adt.Some(v.data[index])
}

// GetOr returns the element at index, or fallback if out of bounds.
func (v *Vec[T]) GetOr(index int, fallback T) T {
	if v == nil || index < 0 || index >= len(v.data) {
		return fallback
	}
	return v.data[index]
}

// Set updates the element at index if within bounds [0, Len()) and returns true.
// Returns false if index is out of bounds.
func (v *Vec[T]) Set(index int, item T) bool {
	if v == nil || index < 0 || index >= len(v.data) {
		return false
	}
	v.data[index] = item
	return true
}

// First returns Some(element) at index 0, or None if empty.
func (v *Vec[T]) First() adt.Option[T] {
	if v == nil || len(v.data) == 0 {
		return adt.None[T]()
	}
	return adt.Some(v.data[0])
}

// Last returns Some(last element), or None if empty.
func (v *Vec[T]) Last() adt.Option[T] {
	if v == nil || len(v.data) == 0 {
		return adt.None[T]()
	}
	return adt.Some(v.data[len(v.data)-1])
}

// Insert inserts item at index [0, Len()], shifting subsequent elements right.
// Returns true on success, or false if index is out of bounds.
func (v *Vec[T]) Insert(index int, item T) bool {
	if v == nil || index < 0 || index > len(v.data) {
		return false
	}
	v.data = slices.Insert(v.data, index, item)
	return true
}

// DeleteAt removes and returns the element at index, shifting subsequent elements left.
// Returns None if index is out of bounds.
func (v *Vec[T]) DeleteAt(index int) adt.Option[T] {
	if v == nil || index < 0 || index >= len(v.data) {
		return adt.None[T]()
	}
	item := v.data[index]
	v.data = slices.Delete(v.data, index, index+1)
	return adt.Some(item)
}

// DeleteFunc removes all elements in place where pred(item) is true.
func (v *Vec[T]) DeleteFunc(pred func(T) bool) {
	if v != nil {
		v.data = slices.DeleteFunc(v.data, pred)
	}
}

// Truncate shrinks v to size n in place. If n >= Len(), it is a no-op.
// If n < 0, n is treated as 0.
func (v *Vec[T]) Truncate(n int) {
	if v == nil {
		return
	}
	if n < 0 {
		n = 0
	}
	if n < len(v.data) {
		var zero T
		for i := n; i < len(v.data); i++ {
			v.data[i] = zero
		}
		v.data = v.data[:n]
	}
}

// Reverse reverses all elements in v in place.
func (v *Vec[T]) Reverse() {
	if v != nil {
		slices.Reverse(v.data)
	}
}

// Clear removes all elements from v in place while preserving capacity.
func (v *Vec[T]) Clear() {
	if v == nil {
		return
	}
	var zero T
	for i := range v.data {
		v.data[i] = zero
	}
	v.data = v.data[:0]
}

// ContainsFunc reports whether any element in v satisfies pred.
func (v *Vec[T]) ContainsFunc(pred func(T) bool) bool {
	if v == nil {
		return false
	}
	return slices.ContainsFunc(v.data, pred)
}

// IndexOfFunc returns the index of the first element satisfying pred, or -1.
func (v *Vec[T]) IndexOfFunc(pred func(T) bool) int {
	if v == nil {
		return -1
	}
	return slices.IndexFunc(v.data, pred)
}

// Len returns the count of elements in v (0 if uninitialized).
func (v *Vec[T]) Len() int {
	if v == nil {
		return 0
	}
	return len(v.data)
}

// Cap returns the capacity of v (0 if uninitialized).
func (v *Vec[T]) Cap() int {
	if v == nil {
		return 0
	}
	return cap(v.data)
}

// Empty reports whether v has 0 elements or is uninitialized.
func (v *Vec[T]) Empty() bool {
	if v == nil {
		return true
	}
	return len(v.data) == 0
}

// Clone returns an independent shallow copy of v.
func (v *Vec[T]) Clone() *Vec[T] {
	if v == nil {
		return VecWithCapacity[T](0)
	}
	res := VecWithCapacity[T](len(v.data))
	res.data = append(res.data, v.data...)
	return res
}

// ToSlice returns all elements as a standard Go slice. Never returns nil.
func (v *Vec[T]) ToSlice() []T {
	if v == nil || len(v.data) == 0 {
		return make([]T, 0)
	}
	res := make([]T, len(v.data))
	copy(res, v.data)
	return res
}

// All returns an iter.Seq[T] iterator yielding elements from index 0 to Len()-1.
func (v *Vec[T]) All() iter.Seq[T] {
	return func(yield func(T) bool) {
		if v == nil {
			return
		}
		for _, item := range v.data {
			if !yield(item) {
				return
			}
		}
	}
}

// AllWithIndex returns an iter.Seq2[int, T] iterator yielding (index, element) pairs.
func (v *Vec[T]) AllWithIndex() iter.Seq2[int, T] {
	return func(yield func(int, T) bool) {
		if v == nil {
			return
		}
		for i, item := range v.data {
			if !yield(i, item) {
				return
			}
		}
	}
}
