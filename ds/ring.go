package ds

import (
	"iter"
	"slices"

	"github.com/azuiktech/kleisli-go/option"
)

// RingBuffer is a fixed-capacity circular buffer. Push overwrites the oldest
// element when full. The zero value is unusable — construct via Ring.
type RingBuffer[T any] struct {
	data []T
	head int // index of the oldest element
	size int // number of live elements
}

// Ring creates a RingBuffer with the given capacity.
func Ring[T any](capacity int) RingBuffer[T] {
	return RingBuffer[T]{data: make([]T, capacity)}
}

// Push inserts v at the back. If the buffer is full, the oldest element is
// silently overwritten.
func (r *RingBuffer[T]) Push(v T) {
	if len(r.data) == 0 {
		return
	}
	if r.size == len(r.data) {
		r.data[r.head] = v
		r.head = (r.head + 1) % len(r.data)
	} else {
		r.data[(r.head+r.size)%len(r.data)] = v
		r.size++
	}
}

// Pop removes and returns the oldest element. Returns None when empty.
func (r *RingBuffer[T]) Pop() option.Option[T] {
	if r.size == 0 {
		return option.None[T]()
	}
	v := r.data[r.head]
	r.head = (r.head + 1) % len(r.data)
	r.size--
	return option.Some(v)
}

// Peek returns the oldest element without removing it. Returns None when empty.
func (r *RingBuffer[T]) Peek() option.Option[T] {
	if r.size == 0 {
		return option.None[T]()
	}
	return option.Some(r.data[r.head])
}

// At returns the element at logical index i (0 = oldest). Returns None when
// i is out of range.
func (r *RingBuffer[T]) At(i int) option.Option[T] {
	if i < 0 || i >= r.size {
		return option.None[T]()
	}
	return option.Some(r.data[(r.head+i)%len(r.data)])
}

// Len returns the number of elements currently in the buffer.
func (r *RingBuffer[T]) Len() int { return r.size }

// Cap returns the maximum number of elements the buffer can hold.
func (r *RingBuffer[T]) Cap() int { return len(r.data) }

// Full reports whether the buffer is at capacity.
func (r *RingBuffer[T]) Full() bool { return r.size == len(r.data) }

// Empty reports whether the buffer holds no elements.
func (r *RingBuffer[T]) Empty() bool { return r.size == 0 }

// Segments returns zero-copy views of the two contiguous slices that make up
// the buffer's live content. Either slice may be empty. Elements are in
// logical order: first comes before second, oldest element is at first[0].
//
// The slices are direct windows into the internal array — do not retain them
// after any mutation.
func (r *RingBuffer[T]) Segments() (first, second []T) {
	if r.size == 0 {
		return nil, nil
	}
	end := r.head + r.size
	if end <= len(r.data) {
		return r.data[r.head:end], nil
	}
	return r.data[r.head:], r.data[:end%len(r.data)]
}

// Linearize copies the buffer's live content into a new contiguous slice in
// logical order (oldest first).
func (r *RingBuffer[T]) Linearize() []T {
	first, second := r.Segments()
	return slices.Concat(first, second)
}

// All returns an iterator over the buffer's live elements in logical order
// (oldest first). The iterator is a snapshot — subsequent mutations do not
// affect it.
func (r *RingBuffer[T]) All() iter.Seq[T] {
	return slices.Values(r.Linearize())
}
