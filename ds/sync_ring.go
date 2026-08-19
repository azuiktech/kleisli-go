package ds

import (
	"iter"
	"slices"

	"github.com/azuiktech/kleisli-go/async"
	"github.com/azuiktech/kleisli-go/option"
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

func (s *SyncRingBuffer[T]) Pop() option.Option[T] {
	return async.Write(&s.Sync, func(r *RingBuffer[T]) option.Option[T] { return r.Pop() })
}

func (s *SyncRingBuffer[T]) Peek() option.Option[T] {
	return async.Read(&s.Sync, func(r *RingBuffer[T]) option.Option[T] { return r.Peek() })
}

func (s *SyncRingBuffer[T]) At(i int) option.Option[T] {
	return async.Read(&s.Sync, func(r *RingBuffer[T]) option.Option[T] { return r.At(i) })
}

func (s *SyncRingBuffer[T]) Len() int {
	return async.Read(&s.Sync, func(r *RingBuffer[T]) int { return r.Len() })
}

func (s *SyncRingBuffer[T]) Cap() int {
	return async.Read(&s.Sync, func(r *RingBuffer[T]) int { return r.Cap() })
}

func (s *SyncRingBuffer[T]) Full() bool {
	return async.Read(&s.Sync, func(r *RingBuffer[T]) bool { return r.Full() })
}

func (s *SyncRingBuffer[T]) Empty() bool {
	return async.Read(&s.Sync, func(r *RingBuffer[T]) bool { return r.Empty() })
}

// Segments returns copies of the two contiguous slices while holding the read
// lock. The returned slices are independent of the internal array.
func (s *SyncRingBuffer[T]) Segments() ([]T, []T) {
	segs := async.Read(&s.Sync, func(r *RingBuffer[T]) [2][]T {
		f, sec := r.Segments()
		return [2][]T{slices.Clone(f), slices.Clone(sec)}
	})
	return segs[0], segs[1]
}

// Linearize returns a copy of the buffer's live content in logical order.
func (s *SyncRingBuffer[T]) Linearize() []T {
	return async.Read(&s.Sync, func(r *RingBuffer[T]) []T { return r.Linearize() })
}

// All returns an iterator over a snapshot of the buffer taken under the read
// lock. Subsequent mutations do not affect the iterator.
func (s *SyncRingBuffer[T]) All() iter.Seq[T] {
	return slices.Values(s.Linearize())
}
