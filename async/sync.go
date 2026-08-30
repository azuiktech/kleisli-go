package async

import "sync"

// Sync wraps any value with a read/write mutex, providing safe concurrent
// access via Write and Read closures. It is not tied to any specific type —
// embed it in a typed wrapper to expose ergonomic per-method locking.
type Sync[C any] struct {
	mu    sync.RWMutex
	inner C
}

// Of wraps inner in a Sync.
func Of[C any](inner C) Sync[C] {
	return Sync[C]{inner: inner}
}

// Write acquires the write lock, calls f, then releases the lock. Use for
// mutations with no return value.
func (s *Sync[C]) Write(f func(*C)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	f(&s.inner)
}

// Read acquires the read lock, passes a snapshot of the value to f, then
// releases the lock. The snapshot is a shallow copy — fields that are
// themselves pointers, maps, or slices share the underlying memory with
// the protected value; do not mutate through them. Multiple readers may
// proceed concurrently.
func (s *Sync[C]) Read(f func(C)) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	f(s.inner)
}

// Mutate acquires the write lock, calls f, and returns its result.
func (s *Sync[C]) Mutate[R any](f func(*C) R) R {
	s.mu.Lock()
	defer s.mu.Unlock()
	return f(&s.inner)
}

// Map acquires the read lock, calls f with a snapshot of the value, and
// returns its result. The snapshot is a shallow copy — see Read for the
// same caveat on pointer/map/slice fields. Multiple readers may proceed
// concurrently.
func (s *Sync[C]) Map[R any](f func(C) R) R {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return f(s.inner)
}
