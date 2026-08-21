package ds

import (
	"iter"
	"slices"

	"github.com/azuiktech/kleisli-go/async"
	"github.com/azuiktech/kleisli-go/adt"
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

func (s *SyncRingBuffer[T]) Pop() (out adt.Option[T]) {
	s.Write(func(r *RingBuffer[T]) { out = r.Pop() })
	return
}

func (s *SyncRingBuffer[T]) Peek() (out adt.Option[T]) {
	s.Read(func(r *RingBuffer[T]) { out = r.Peek() })
	return
}

func (s *SyncRingBuffer[T]) At(i int) (out adt.Option[T]) {
	s.Read(func(r *RingBuffer[T]) { out = r.At(i) })
	return
}

func (s *SyncRingBuffer[T]) Len() (n int) {
	s.Read(func(r *RingBuffer[T]) { n = r.Len() })
	return
}

func (s *SyncRingBuffer[T]) Cap() (n int) {
	s.Read(func(r *RingBuffer[T]) { n = r.Cap() })
	return
}

func (s *SyncRingBuffer[T]) Full() (b bool) {
	s.Read(func(r *RingBuffer[T]) { b = r.Full() })
	return
}

func (s *SyncRingBuffer[T]) Empty() (b bool) {
	s.Read(func(r *RingBuffer[T]) { b = r.Empty() })
	return
}

// Segments returns copies of the two contiguous slices while holding the read
// lock. The returned slices are independent of the internal array.
func (s *SyncRingBuffer[T]) Segments() (first, second []T) {
	s.Read(func(r *RingBuffer[T]) {
		f, sec := r.Segments()
		first = slices.Clone(f)
		second = slices.Clone(sec)
	})
	return
}

// Linearize returns a copy of the buffer's live content in logical order.
func (s *SyncRingBuffer[T]) Linearize() (out []T) {
	s.Read(func(r *RingBuffer[T]) { out = r.Linearize() })
	return
}

// All returns an iterator over a snapshot of the buffer taken under the read
// lock. Subsequent mutations do not affect the iterator.
func (s *SyncRingBuffer[T]) All() iter.Seq[T] {
	snap := s.Linearize()
	return slices.Values(snap)
}
