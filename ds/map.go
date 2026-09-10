package ds

import (
	"iter"
	"maps"

	"github.com/azuiktech/kleisli-go/adt"
)

// Entry represents a key-value pair.
type Entry[K comparable, V any] struct {
	Key   K
	Value V
}

// Map is an encapsulated generic hash map data structure.
// Passed by value; mutations affect the underlying map in place.
type Map[K comparable, V any] struct {
	data map[K]V
}

// NewMap constructs an initialized Map containing the given entries.
func NewMap[K comparable, V any](entries ...Entry[K, V]) Map[K, V] {
	m := Map[K, V]{data: make(map[K]V, len(entries))}
	for _, e := range entries {
		m.data[e.Key] = e.Value
	}
	return m
}

// MapOf constructs an initialized Map containing the given entries.
func MapOf[K comparable, V any](entries ...Entry[K, V]) Map[K, V] {
	return NewMap(entries...)
}

// MapWithCapacity constructs an empty Map with preallocated capacity.
func MapWithCapacity[K comparable, V any](capacity int) Map[K, V] {
	return Map[K, V]{data: make(map[K]V, capacity)}
}

// FromMap constructs a Map containing all key-value pairs from m.
func FromMap[K comparable, V any](m map[K]V) Map[K, V] {
	res := MapWithCapacity[K, V](len(m))
	maps.Copy(res.data, m)
	return res
}

// FromSeq2 constructs a Map by consuming an iter.Seq2[K, V].
func FromSeq2[K comparable, V any](seq iter.Seq2[K, V]) Map[K, V] {
	m := Map[K, V]{data: make(map[K]V)}
	for k, v := range seq {
		m.data[k] = v
	}
	return m
}

// Put inserts or updates a key-value pair in place and returns m for chaining.
func (m Map[K, V]) Put(key K, value V) Map[K, V] {
	m.data[key] = value
	return m
}

// PutIfAbsent inserts value only if key is not already present.
// Returns the existing or newly inserted value, and true if inserted.
func (m Map[K, V]) PutIfAbsent(key K, value V) (V, bool) {
	if existing, ok := m.data[key]; ok {
		return existing, false
	}
	m.data[key] = value
	return value, true
}

// PutAll inserts all given entries into m in place and returns m.
func (m Map[K, V]) PutAll(entries ...Entry[K, V]) Map[K, V] {
	for _, e := range entries {
		m.data[e.Key] = e.Value
	}
	return m
}

// Delete removes key from m in place and returns m.
func (m Map[K, V]) Delete(key K) Map[K, V] {
	delete(m.data, key)
	return m
}

// DeleteFunc removes any entry where pred(key, val) is true and returns m.
func (m Map[K, V]) DeleteFunc(pred func(K, V) bool) Map[K, V] {
	maps.DeleteFunc(m.data, pred)
	return m
}

// Clear removes all entries from m in place and returns m.
func (m Map[K, V]) Clear() Map[K, V] {
	clear(m.data)
	return m
}

// Get returns Some(value) if key is present, or None. Safe on uninitialized Map.
func (m Map[K, V]) Get(key K) adt.Option[V] {
	if v, ok := m.data[key]; ok {
		return adt.Some(v)
	}
	return adt.None[V]()
}

// GetOr returns the value for key, or fallback if absent.
func (m Map[K, V]) GetOr(key K, fallback V) V {
	if v, ok := m.data[key]; ok {
		return v
	}
	return fallback
}

// GetOrElse returns the value for key, or calls fn() if absent.
func (m Map[K, V]) GetOrElse(key K, fn func() V) V {
	if v, ok := m.data[key]; ok {
		return v
	}
	return fn()
}

// Contains reports whether key is present in m.
func (m Map[K, V]) Contains(key K) bool {
	_, ok := m.data[key]
	return ok
}

// Len returns the count of entries in m (0 if uninitialized).
func (m Map[K, V]) Len() int {
	return len(m.data)
}

// Empty reports whether m has 0 entries or is uninitialized.
func (m Map[K, V]) Empty() bool {
	return len(m.data) == 0
}

// Clone returns an independent shallow copy of m. Never returns nil backing map.
func (m Map[K, V]) Clone() Map[K, V] {
	res := MapWithCapacity[K, V](m.Len())
	maps.Copy(res.data, m.data)
	return res
}

// EqualFunc reports whether m and other have the same keys and matching values according to eq.
func (m Map[K, V]) EqualFunc(other Map[K, V], eq func(V, V) bool) bool {
	if m.Len() != other.Len() {
		return false
	}
	return maps.EqualFunc(m.data, other.data, eq)
}

// ToMap returns the entries as a standard Go map[K]V. Never returns nil.
func (m Map[K, V]) ToMap() map[K]V {
	res := make(map[K]V, m.Len())
	maps.Copy(res, m.data)
	return res
}

// ToSlice returns all entries as a slice. Never returns nil.
func (m Map[K, V]) ToSlice() []Entry[K, V] {
	res := make([]Entry[K, V], 0, m.Len())
	for k, v := range m.data {
		res = append(res, Entry[K, V]{Key: k, Value: v})
	}
	return res
}

// Keys returns an iterator yielding all keys in m.
func (m Map[K, V]) Keys() iter.Seq[K] {
	return func(yield func(K) bool) {
		for k := range m.data {
			if !yield(k) {
				return
			}
		}
	}
}

// Values returns an iterator yielding all values in m.
func (m Map[K, V]) Values() iter.Seq[V] {
	return func(yield func(V) bool) {
		for _, v := range m.data {
			if !yield(v) {
				return
			}
		}
	}
}

// All returns an iter.Seq2[K, V] iterator yielding (key, value) pairs.
func (m Map[K, V]) All() iter.Seq2[K, V] {
	return func(yield func(K, V) bool) {
		for k, v := range m.data {
			if !yield(k, v) {
				return
			}
		}
	}
}
