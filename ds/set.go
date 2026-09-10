package ds

import (
	"iter"
	"maps"

	"github.com/azuiktech/kleisli-go/adt"
)

// Set is a generic hash set data structure defined directly over map[T]adt.Unit.
type Set[T comparable] map[T]adt.Unit

// SetOf constructs an initialized Set containing the given items.
func SetOf[T comparable](items ...T) Set[T] {
	s := make(Set[T], len(items))
	for _, item := range items {
		s[item] = adt.Void
	}
	return s
}

// SetWithCapacity constructs an empty Set with preallocated capacity.
func SetWithCapacity[T comparable](capacity int) Set[T] {
	return make(Set[T], capacity)
}

// FromSlice constructs a Set containing all unique items from slice.
func FromSlice[T comparable](slice []T) Set[T] {
	return SetOf(slice...)
}

// FromSeq constructs a Set by consuming an iter.Seq[T].
func FromSeq[T comparable](seq iter.Seq[T]) Set[T] {
	s := make(Set[T])
	for item := range seq {
		s[item] = adt.Void
	}
	return s
}

// Add inserts item into the set and returns s for fluent chaining.
func (s Set[T]) Add(item T) Set[T] {
	s[item] = adt.Void
	return s
}

// AddAll inserts all items into s and returns s.
func (s Set[T]) AddAll(items ...T) Set[T] {
	for _, item := range items {
		s[item] = adt.Void
	}
	return s
}

// Delete removes item from s and returns s.
func (s Set[T]) Delete(item T) Set[T] {
	delete(s, item)
	return s
}

// DeleteFunc removes any element in place where pred(item) is true.
func (s Set[T]) DeleteFunc(pred func(T) bool) Set[T] {
	maps.DeleteFunc(s, func(item T, _ adt.Unit) bool {
		return pred(item)
	})
	return s
}

// Clear removes all elements from s using Go clear(s) and returns s.
func (s Set[T]) Clear() Set[T] {
	clear(s)
	return s
}

// Contains reports whether item is present in the set.
func (s Set[T]) Contains(item T) bool {
	_, ok := s[item]
	return ok
}

// Len returns the count of elements in s (0 if s is nil).
func (s Set[T]) Len() int {
	return len(s)
}

// Empty reports whether s has 0 elements or is nil.
func (s Set[T]) Empty() bool {
	return len(s) == 0
}

// Clone returns an independent shallow copy of s. Never returns nil.
func (s Set[T]) Clone() Set[T] {
	res := make(Set[T], len(s))
	for item := range s {
		res[item] = adt.Void
	}
	return res
}

// Equal reports whether s and other contain the exact same elements.
func (s Set[T]) Equal(other Set[T]) bool {
	if len(s) != len(other) {
		return false
	}
	for item := range s {
		if !other.Contains(item) {
			return false
		}
	}
	return true
}

// ToSlice returns all elements of s as a slice. Never returns nil.
func (s Set[T]) ToSlice() []T {
	res := make([]T, 0, len(s))
	for item := range s {
		res = append(res, item)
	}
	return res
}

// All returns an iter.Seq[T] iterator yielding all elements in s.
func (s Set[T]) All() iter.Seq[T] {
	return func(yield func(T) bool) {
		for item := range s {
			if !yield(item) {
				return
			}
		}
	}
}

// Union returns a new Set containing all elements present in either s or other (A ∪ B).
func (s Set[T]) Union(other Set[T]) Set[T] {
	res := make(Set[T], len(s)+len(other))
	for item := range s {
		res[item] = adt.Void
	}
	for item := range other {
		res[item] = adt.Void
	}
	return res
}

// Intersect returns a new Set containing only elements present in both s and other (A ∩ B).
func (s Set[T]) Intersect(other Set[T]) Set[T] {
	small, large := s, other
	if len(small) > len(large) {
		small, large = other, s
	}
	res := make(Set[T])
	for item := range small {
		if large.Contains(item) {
			res[item] = adt.Void
		}
	}
	return res
}

// Diff returns a new Set containing elements present in s but not other (A \ B).
func (s Set[T]) Diff(other Set[T]) Set[T] {
	res := make(Set[T])
	for item := range s {
		if !other.Contains(item) {
			res[item] = adt.Void
		}
	}
	return res
}

// SymmetricDiff returns a new Set containing elements in either s or other, but not both.
func (s Set[T]) SymmetricDiff(other Set[T]) Set[T] {
	res := make(Set[T])
	for item := range s {
		if !other.Contains(item) {
			res[item] = adt.Void
		}
	}
	for item := range other {
		if !s.Contains(item) {
			res[item] = adt.Void
		}
	}
	return res
}

// IsSubset reports whether all elements of s are contained in other (A ⊆ B).
func (s Set[T]) IsSubset(other Set[T]) bool {
	if len(s) > len(other) {
		return false
	}
	for item := range s {
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
	if len(small) > len(large) {
		small, large = other, s
	}
	for item := range small {
		if large.Contains(item) {
			return false
		}
	}
	return true
}
