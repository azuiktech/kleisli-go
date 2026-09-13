package ds

import (
	"iter"
	"slices"

	"github.com/azuiktech/kleisli-go/adt"
	"github.com/azuiktech/kleisli-go/async"
)

// SyncRingBuffer is a thread-safe RingBuffer backed by async.Sync.
// All methods are safe for concurrent use. Construct via GuardedRing.
type SyncRingBuffer[T any] struct {
	async.Sync[RingBuffer[T]]
}

// GuardedRing creates a thread-safe RingBuffer with the given capacity.
func GuardedRing[T any](capacity int) *SyncRingBuffer[T] {
	return &SyncRingBuffer[T]{Sync: async.Of(Ring[T](capacity))}
}

func (s *SyncRingBuffer[T]) Push(v T) {
	s.Write(func(r *RingBuffer[T]) { r.Push(v) })
}

func (s *SyncRingBuffer[T]) Pop() adt.Option[T] {
	return s.Mutate(func(r *RingBuffer[T]) adt.Option[T] { return r.Pop() })
}

func (s *SyncRingBuffer[T]) Peek() adt.Option[T] {
	return s.Fetch(func(r RingBuffer[T]) adt.Option[T] { return r.Peek() })
}

func (s *SyncRingBuffer[T]) At(i int) adt.Option[T] {
	return s.Fetch(func(r RingBuffer[T]) adt.Option[T] { return r.At(i) })
}

func (s *SyncRingBuffer[T]) Len() int {
	return s.Fetch(func(r RingBuffer[T]) int { return r.Len() })
}

func (s *SyncRingBuffer[T]) Cap() int {
	return s.Fetch(func(r RingBuffer[T]) int { return r.Cap() })
}

func (s *SyncRingBuffer[T]) Full() bool {
	return s.Fetch(func(r RingBuffer[T]) bool { return r.Full() })
}

func (s *SyncRingBuffer[T]) Empty() bool {
	return s.Fetch(func(r RingBuffer[T]) bool { return r.Empty() })
}

// Segments returns copies of the two contiguous slices while holding the read
// lock. The returned slices are independent of the internal array.
func (s *SyncRingBuffer[T]) Segments() ([]T, []T) {
	type segs struct{ first, second []T }
	res := s.Fetch(func(r RingBuffer[T]) segs {
		f, sec := r.Segments()
		return segs{slices.Clone(f), slices.Clone(sec)}
	})
	return res.first, res.second
}

// Linearize returns a copy of the buffer's live content in logical order.
func (s *SyncRingBuffer[T]) Linearize() []T {
	return s.Fetch(func(r RingBuffer[T]) []T { return r.Linearize() })
}

// All returns an iterator over a snapshot of the buffer taken under the read
// lock. Subsequent mutations do not affect the iterator.
func (s *SyncRingBuffer[T]) All() iter.Seq[T] {
	snap := s.Linearize()
	return slices.Values(snap)
}

// Clear removes all elements under the write lock.
func (s *SyncRingBuffer[T]) Clear() {
	s.Write(func(r *RingBuffer[T]) { r.Clear() })
}
