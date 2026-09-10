package ds

import (
	"iter"
	"maps"

	"github.com/azuiktech/kleisli-go/adt"
)

// Set is an encapsulated generic hash set data structure.
// Passed by value; mutations affect the underlying map in place.
type Set[T comparable] struct {
	data map[T]adt.Unit
}

// NewSet constructs an initialized Set containing the given items.
func NewSet[T comparable](items ...T) Set[T] {
	s := Set[T]{data: make(map[T]adt.Unit, len(items))}
	for _, item := range items {
		s.data[item] = adt.Void
	}
	return s
}

// SetOf constructs an initialized Set containing the given items.
func SetOf[T comparable](items ...T) Set[T] {
	return NewSet(items...)
}

// SetWithCapacity constructs an empty Set with preallocated capacity.
func SetWithCapacity[T comparable](capacity int) Set[T] {
	return Set[T]{data: make(map[T]adt.Unit, capacity)}
}

// FromSlice constructs a Set containing all unique items from slice.
func FromSlice[T comparable](slice []T) Set[T] {
	return NewSet(slice...)
}

// FromSeq constructs a Set by consuming an iter.Seq[T].
func FromSeq[T comparable](seq iter.Seq[T]) Set[T] {
	s := Set[T]{data: make(map[T]adt.Unit)}
	for item := range seq {
		s.data[item] = adt.Void
	}
	return s
}

// Add inserts item into the set and returns s for fluent chaining.
func (s Set[T]) Add(item T) Set[T] {
	s.data[item] = adt.Void
	return s
}

// AddAll inserts all items into s and returns s.
func (s Set[T]) AddAll(items ...T) Set[T] {
	for _, item := range items {
		s.data[item] = adt.Void
	}
	return s
}

// Delete removes item from s and returns s.
func (s Set[T]) Delete(item T) Set[T] {
	delete(s.data, item)
	return s
}

// DeleteFunc removes any element in place where pred(item) is true.
func (s Set[T]) DeleteFunc(pred func(T) bool) Set[T] {
	maps.DeleteFunc(s.data, func(item T, _ adt.Unit) bool {
		return pred(item)
	})
	return s
}

// Clear removes all elements from s and returns s.
func (s Set[T]) Clear() Set[T] {
	clear(s.data)
	return s
}

// Contains reports whether item is present in the set.
func (s Set[T]) Contains(item T) bool {
	_, ok := s.data[item]
	return ok
}

// Len returns the count of elements in s (0 if s is empty or uninitialized).
func (s Set[T]) Len() int {
	return len(s.data)
}

// Empty reports whether s has 0 elements or is uninitialized.
func (s Set[T]) Empty() bool {
	return len(s.data) == 0
}

// Clone returns an independent shallow copy of s. Never returns nil.
func (s Set[T]) Clone() Set[T] {
	res := SetWithCapacity[T](s.Len())
	maps.Copy(res.data, s.data)
	return res
}

// Equal reports whether s and other contain the exact same elements.
func (s Set[T]) Equal(other Set[T]) bool {
	if s.Len() != other.Len() {
		return false
	}
	for item := range s.data {
		if !other.Contains(item) {
			return false
		}
	}
	return true
}

// ToSlice returns all elements of s as a slice. Never returns nil.
func (s Set[T]) ToSlice() []T {
	res := make([]T, 0, len(s.data))
	for item := range s.data {
		res = append(res, item)
	}
	return res
}

// All returns an iter.Seq[T] iterator yielding all elements in s.
func (s Set[T]) All() iter.Seq[T] {
	return func(yield func(T) bool) {
		for item := range s.data {
			if !yield(item) {
				return
			}
		}
	}
}

// Union returns a new Set containing all elements present in either s or other (A ∪ B).
func (s Set[T]) Union(other Set[T]) Set[T] {
	res := SetWithCapacity[T](s.Len() + other.Len())
	maps.Copy(res.data, s.data)
	maps.Copy(res.data, other.data)
	return res
}

// Intersect returns a new Set containing only elements present in both s and other (A ∩ B).
func (s Set[T]) Intersect(other Set[T]) Set[T] {
	small, large := s, other
	if small.Len() > large.Len() {
		small, large = other, s
	}
	res := SetWithCapacity[T](0)
	for item := range small.data {
		if large.Contains(item) {
			res.data[item] = adt.Void
		}
	}
	return res
}

// Diff returns a new Set containing elements present in s but not other (A \ B).
func (s Set[T]) Diff(other Set[T]) Set[T] {
	res := SetWithCapacity[T](0)
	for item := range s.data {
		if !other.Contains(item) {
			res.data[item] = adt.Void
		}
	}
	return res
}

// SymmetricDiff returns a new Set containing elements in either s or other, but not both.
func (s Set[T]) SymmetricDiff(other Set[T]) Set[T] {
	res := SetWithCapacity[T](0)
	for item := range s.data {
		if !other.Contains(item) {
			res.data[item] = adt.Void
		}
	}
	for item := range other.data {
		if !s.Contains(item) {
			res.data[item] = adt.Void
		}
	}
	return res
}

// IsSubset reports whether all elements of s are contained in other (A ⊆ B).
func (s Set[T]) IsSubset(other Set[T]) bool {
	if s.Len() > other.Len() {
		return false
	}
	for item := range s.data {
		if !other.Contains(item) {
			return false
		}
	}
	return true
}

// IsSuperset reports whether all elements of other are contained in s (A ⊇ B).
func (s Set[T]) IsSuperset(other Set[T]) bool {
	return other.IsSubset(s)
}

// IsDisjoint reports whether s and other share no common elements (A ∩ B = ∅).
func (s Set[T]) IsDisjoint(other Set[T]) bool {
	small, large := s, other
	if small.Len() > large.Len() {
		small, large = other, s
	}
	for item := range small.data {
		if large.Contains(item) {
			return false
		}
	}
	return true
}
