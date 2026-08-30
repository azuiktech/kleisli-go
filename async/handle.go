package async

// Handle[D] is a value type referencing shared concurrent state D.
// All copies of a Handle share the same *Sync[D] — one allocation of data,
// many handles that can each expose a different method set.
//
// The natural base for in-memory store implementations where D holds the
// actual data (maps, counters, queues). Multiple struct types can embed
// Handle[D] and satisfy different interfaces without any explicit declaration:
//
//	type orgState struct { orgs map[uuid.UUID]Org }
//
//	type InMemOrgs    struct{ Handle[orgState] }
//	type InMemMembers struct{ Handle[orgState] }  // same shared state
//
//	h := NewHandle(orgState{orgs: make(map[uuid.UUID]Org)})
//	orgs    := InMemOrgs{h}
//	members := InMemMembers{h}  // both see the same orgState
type Handle[D any] struct{ impl *Sync[D] }

// NewHandle initializes D and returns a Handle wrapping it.
func NewHandle[D any](init D) Handle[D] {
	s := Of(init)
	return Handle[D]{impl: &s}
}

// Read acquires the read lock and calls fn with a snapshot of the state.
// The snapshot is a shallow copy — see Sync.Read for caveats on
// pointer/map/slice fields. Multiple readers may proceed concurrently.
func (h Handle[D]) Read(fn func(D)) { h.impl.Read(fn) }

// Write acquires the write lock and calls fn.
func (h Handle[D]) Write(fn func(*D)) { h.impl.Write(fn) }

// Map acquires the read lock, calls fn with a snapshot of the state, and
// returns its result. Multiple readers may proceed concurrently.
func (h Handle[D]) Map[R any](fn func(D) R) R { return h.impl.Map(fn) }

// Mutate acquires the write lock, calls fn, and returns its result.
func (h Handle[D]) Mutate[R any](fn func(*D) R) R { return h.impl.Mutate(fn) }
