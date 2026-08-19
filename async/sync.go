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

// Write acquires the write lock, calls f with a pointer to the inner value,
// then releases the lock. Use for any mutation or read-modify-write.
func (s *Sync[C]) Write(f func(*C)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	f(&s.inner)
}

// Read acquires the read lock, calls f with a pointer to the inner value,
// then releases the lock. Multiple readers may proceed concurrently.
func (s *Sync[C]) Read(f func(*C)) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	f(&s.inner)
}
