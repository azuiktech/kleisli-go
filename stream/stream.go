// Package stream provides Stream[T], a generic type for eager pipeline
// operations over in-memory slices, and Seq[T], its lazy, pull-based
// counterpart wrapping the standard library's own iter.Seq[T].
//
// The two share one implementation for every operation that appears on
// both (Filter, Map, TakeWhile, ...) — Stream just runs the same engine
// via slices.Values then collects immediately. Stream additionally
// offers operations that need to see the whole sequence (Reverse, the
// SortBy family, GroupBy, ToMap, Partition, Last, Len) — sound there
// specifically because a slice is always finite and already in memory;
// absent from Seq's method set entirely, since an arbitrary Seq might
// not be finite.
//
// Map, FlatMap, Reduce, GroupBy, and ToMap use Go 1.27 generic methods,
// enabling type-changing transformations via method chaining without free
// functions or wrapper types.
//
//	totals := stream.Of(invoices).
//	    Filter(func(inv Invoice) bool { return inv.Status == Unpaid }).
//	    GroupBy[string](func(inv Invoice) string { return inv.ClientID })
package stream

import (
	"cmp"
	"slices"
)

// Stream wraps a slice for pipeline operations. Every operation it
// shares with Seq is evaluated the same way Seq evaluates it (see
// package doc); Stream's own additional operations (Reverse, sorting,
// grouping, ...) are eager, since they need the whole slice anyway.
type Stream[T any] struct {
	items []T
}

// Of wraps a slice in a Stream. The original slice is not copied — Stream
// shares its backing array with the caller, so mutating the source slice
// after calling Of affects the Stream too.
func Of[T any](items []T) Stream[T] { return Stream[T]{items: items} }

// FromSlice is Of, named to read as Seq's own FromSeq's counterpart.
func FromSlice[T any](items []T) Stream[T] { return Of(items) }

// Empty returns a Stream with no elements.
func Empty[T any]() Stream[T] { return Stream[T]{} }

// OfMap wraps a map's entries as a Stream of Pair — map iteration order is
// randomized by Go itself, so callers needing a deterministic order should
// follow with SortBy/SortByCached on the key.
func OfMap[K comparable, V any](m map[K]V) Stream[Pair[K, V]] {
	pairs := make([]Pair[K, V], 0, len(m))
	for k, v := range m {
		pairs = append(pairs, Pair[K, V]{First: k, Second: v})
	}
	return Stream[Pair[K, V]]{items: pairs}
}

// Filter returns a Stream containing only elements for which fn returns true.
func (s Stream[T]) Filter(fn func(T) bool) Stream[T] {
	return Stream[T]{items: slices.Collect(filterSeq(slices.Values(s.items), fn))}
}

// Each calls fn on every element. Returns s unchanged for chaining.
func (s Stream[T]) Each(fn func(T)) Stream[T] {
	for _, v := range s.items {
		fn(v)
	}
	return s
}

// Any reports whether fn returns true for at least one element.
func (s Stream[T]) Any(fn func(T) bool) bool {
	for _, v := range s.items {
		if fn(v) {
			return true
		}
	}
	return false
}

// All reports whether fn returns true for every element.
func (s Stream[T]) All(fn func(T) bool) bool {
	for _, v := range s.items {
		if !fn(v) {
			return false
		}
	}
	return true
}

// First returns the first element satisfying fn, or (zero, false).
func (s Stream[T]) First(fn func(T) bool) (T, bool) {
	for _, v := range s.items {
		if fn(v) {
			return v, true
		}
	}
	var zero T
	return zero, false
}

// Last returns the last element satisfying fn, or (zero, false). Scans
// backward so a match near the end is found without touching the rest of
// the slice — O(1) best case, matching Rust's DoubleEndedIterator::rfind
// rather than a forward scan that must always reach the end.
func (s Stream[T]) Last(fn func(T) bool) (T, bool) {
	for i := len(s.items) - 1; i >= 0; i-- {
		if fn(s.items[i]) {
			return s.items[i], true
		}
	}
	var zero T
	return zero, false
}

// Take returns a Stream containing at most n leading elements.
func (s Stream[T]) Take(n int) Stream[T] {
	if n >= len(s.items) {
		return s
	}
	return Stream[T]{items: s.items[:n]}
}

// Skip returns a Stream with the first n elements removed.
func (s Stream[T]) Skip(n int) Stream[T] {
	if n >= len(s.items) {
		return Empty[T]()
	}
	return Stream[T]{items: s.items[n:]}
}

// Reverse returns a Stream with elements in reversed order. Sound only
// here, not on Seq: reversing needs to know the last element before it
// can produce the first one, which slices.Backward gets for free (a
// slice already supports backward iteration) but an arbitrary Seq
// can't promise at all.
func (s Stream[T]) Reverse() Stream[T] {
	out := make([]T, 0, len(s.items))
	for _, v := range slices.Backward(s.items) {
		out = append(out, v)
	}
	return Stream[T]{items: out}
}

// Len returns the number of elements.
func (s Stream[T]) Len() int { return len(s.items) }

// Collect returns the underlying slice.
func (s Stream[T]) Collect() []T { return s.items }

// --- Go 1.27 generic methods ---

// Map transforms each element into type U.
//
// Requires Go 1.27 (generic method type parameter).
func (s Stream[T]) Map[U any](fn func(T) U) Stream[U] {
	return Stream[U]{items: slices.Collect(mapSeq(slices.Values(s.items), fn))}
}

// FlatMap maps each element to a slice of U and concatenates the results.
//
// Requires Go 1.27 (generic method type parameter).
func (s Stream[T]) FlatMap[U any](fn func(T) []U) Stream[U] {
	return Stream[U]{items: slices.Collect(flatMapSeq(slices.Values(s.items), fn))}
}

// Reduce folds every element into an accumulator of type U.
//
// Requires Go 1.27 (generic method type parameter).
func (s Stream[T]) Reduce[U any](initial U, fn func(U, T) U) U {
	acc := initial
	for _, v := range s.items {
		acc = fn(acc, v)
	}
	return acc
}

// GroupBy partitions elements into a map keyed by K.
// Elements with the same key are collected in insertion order.
//
// Requires Go 1.27 (generic method type parameter).
func (s Stream[T]) GroupBy[K comparable](fn func(T) K) map[K][]T {
	out := make(map[K][]T)
	for _, v := range s.items {
		k := fn(v)
		out[k] = append(out[k], v)
	}
	return out
}

// ToMap converts the Stream into a map[K]V by extracting a key and value
// from each element. Later elements overwrite earlier ones on key collision.
//
// Requires Go 1.27 (generic method type parameter).
func (s Stream[T]) ToMap[K comparable, V any](fn func(T) (K, V)) map[K]V {
	out := make(map[K]V, len(s.items))
	for _, v := range s.items {
		k, val := fn(v)
		out[k] = val
	}
	return out
}

// --- Gatherer: general stateful intermediate operations ---
//
// Everything above is a fixed operation this package had to hardcode.
// Gatherer (Java's java.util.stream.Gatherer, JEP 485, minus the
// parallel-combiner half — these Streams aren't parallel) is the escape
// hatch for the operation nobody hardcoded: a small, named, reusable value
// — init state, an integrator that consumes one element and emits
// zero-or-more outputs (and reports whether to keep going), an optional
// finisher for trailing state — that plugs into Gather exactly like a
// built-in op. Most of what follows in this section is itself just a
// Gatherer.

// Gatherer is a custom, stateful intermediate operation. A written-once
// Gatherer is a reusable named combinator, same status as a built-in one —
// this is the sanctioned place for a loop to live when nothing built-in
// fits, not license to reach for one in business code.
type Gatherer[T, A, U any] struct {
	Init func() A
	// Integrate processes one item against the current state, may emit
	// zero or more outputs, and returns the next state plus whether to
	// keep consuming (false stops early, same as Java's integrator
	// returning false — Finish still runs against whatever state exists).
	Integrate func(state A, item T, emit func(U)) (next A, cont bool)
	// Finish flushes any trailing state once integration stops. Optional —
	// nil means there's nothing to flush.
	Finish func(state A, emit func(U))
}

// Gather runs g over the Stream — the same engine (gatherSeq) Seq's own
// Gather uses, driven here via slices.Values then collected immediately.
func (s Stream[T]) Gather[A, U any](g Gatherer[T, A, U]) Stream[U] {
	return Stream[U]{items: slices.Collect(gatherSeq(slices.Values(s.items), g))}
}

func statelessGatherer[T, U any](integrate func(item T, emit func(U))) Gatherer[T, struct{}, U] {
	return Gatherer[T, struct{}, U]{
		Init: func() struct{} { return struct{}{} },
		Integrate: func(s struct{}, item T, emit func(U)) (struct{}, bool) {
			integrate(item, emit)
			return s, true
		},
	}
}

// MapMulti: fn gets each element plus an emit callback it can call zero,
// one, or many times — one primitive covering Map (emit once,
// transformed), Filter (emit conditionally, unchanged), FilterMap (emit
// conditionally, transformed), and FlatMap (emit in a loop), without an
// intermediate slice per input element. Java's mapMulti.
func (s Stream[T]) MapMulti[U any](fn func(item T, emit func(U))) Stream[U] {
	return s.Gather(statelessGatherer(fn))
}

// FilterMap keeps and transforms only the elements for which fn's second
// return is true — Rust's filter_map, built on MapMulti.
func (s Stream[T]) FilterMap[U any](fn func(T) (U, bool)) Stream[U] {
	return s.MapMulti(func(item T, emit func(U)) {
		if u, ok := fn(item); ok {
			emit(u)
		}
	})
}

// takeWhileGatherer is TakeWhile's shared engine — Stream.TakeWhile and
// Seq.TakeWhile both just run this via Gather.
func takeWhileGatherer[T any](fn func(T) bool) Gatherer[T, struct{}, T] {
	return Gatherer[T, struct{}, T]{
		Init: func() struct{} { return struct{}{} },
		Integrate: func(state struct{}, item T, emit func(T)) (struct{}, bool) {
			if !fn(item) {
				return state, false
			}
			emit(item)
			return state, true
		},
	}
}

// TakeWhile returns elements up to but not including the first one for
// which fn is false — Java 9+/Rust/C++20's take_while.
func (s Stream[T]) TakeWhile(fn func(T) bool) Stream[T] {
	return s.Gather(takeWhileGatherer(fn))
}

// dropWhileGatherer is DropWhile's shared engine.
func dropWhileGatherer[T any](fn func(T) bool) Gatherer[T, bool, T] {
	return Gatherer[T, bool, T]{
		Init: func() bool { return true }, // still dropping
		Integrate: func(dropping bool, item T, emit func(T)) (bool, bool) {
			if dropping && fn(item) {
				return true, true
			}
			emit(item)
			return false, true
		},
	}
}

// DropWhile skips elements while fn is true, then keeps everything from
// the first failure onward — Java 9+/Rust/C++20's drop_while.
func (s Stream[T]) DropWhile(fn func(T) bool) Stream[T] {
	return s.Gather(dropWhileGatherer(fn))
}

// Indexed pairs an element with its position — Enumerate's element type.
type Indexed[T any] struct {
	Index int
	Value T
}

// enumerateGatherer is Enumerate's shared engine.
func enumerateGatherer[T any]() Gatherer[T, int, Indexed[T]] {
	return Gatherer[T, int, Indexed[T]]{
		Init: func() int { return 0 },
		Integrate: func(i int, item T, emit func(Indexed[T])) (int, bool) {
			emit(Indexed[T]{Index: i, Value: item})
			return i + 1, true
		},
	}
}

// Enumerate pairs each element with its zero-based index — Rust/C++23's
// enumerate. Without this, getting an element's position has no path
// except a raw for, which would quietly break the no-raw-loops rule.
//
// A free function, not a method: the go1.27rc1 toolchain rejects any
// *method* on Stream[T] that returns Stream[Wrapper[T]] (a generic type
// instantiated with another generic type built from the same T) as a
// false "instantiation cycle" — confirmed via a minimal repro to be that
// specific shape, unrelated to Gather itself. The identical logic, called
// as a free function taking Stream[T] as a parameter, compiles cleanly.
func Enumerate[T any](s Stream[T]) Stream[Indexed[T]] {
	return s.Gather(enumerateGatherer[T]())
}

// distinctByGatherer is DistinctBy's shared engine.
func distinctByGatherer[T any, K comparable](fn func(T) K) Gatherer[T, map[K]struct{}, T] {
	return Gatherer[T, map[K]struct{}, T]{
		Init: func() map[K]struct{} { return make(map[K]struct{}) },
		Integrate: func(seen map[K]struct{}, item T, emit func(T)) (map[K]struct{}, bool) {
			k := fn(item)
			if _, ok := seen[k]; !ok {
				seen[k] = struct{}{}
				emit(item)
			}
			return seen, true
		},
	}
}

// DistinctBy keeps only the first element for each key fn extracts — works
// for any T since only the extracted key needs to be comparable.
func (s Stream[T]) DistinctBy[K comparable](fn func(T) K) Stream[T] {
	return s.Gather(distinctByGatherer[T, K](fn))
}

// Distinct keeps only the first occurrence of each element, by direct
// equality. A free function, not a method: Stream[T] is declared T any,
// and a method can't narrow that to comparable — only a function generic
// over its own T can. DistinctBy is the method form for any T.
func Distinct[T comparable](s Stream[T]) Stream[T] {
	return s.DistinctBy(func(t T) T { return t })
}

// Scan emits every intermediate accumulator value — a running Reduce, as
// opposed to Reduce's single final value. Rust's Iterator::scan.
func Scan[T, A any](initial A, fn func(A, T) A) Gatherer[T, A, A] {
	return Gatherer[T, A, A]{
		Init: func() A { return initial },
		Integrate: func(acc A, item T, emit func(A)) (A, bool) {
			acc = fn(acc, item)
			emit(acc)
			return acc, true
		},
	}
}

// Fold is Reduce's non-terminal twin: the same single final accumulator,
// but emitted once at the end instead of returned as a bare value — so
// it keeps flowing as a one-element Stream[U] into FlatMap/Map rather
// than ending the pipeline. Scan's opposite in that sense: Scan emits on
// every step and never at the end; Fold emits only at the end.
func Fold[T, U any](initial U, fn func(U, T) U) Gatherer[T, U, U] {
	return Gatherer[T, U, U]{
		Init: func() U { return initial },
		Integrate: func(acc U, item T, emit func(U)) (U, bool) {
			return fn(acc, item), true
		},
		Finish: func(acc U, emit func(U)) { emit(acc) },
	}
}

// WindowFixed partitions into non-overlapping chunks of n — the last chunk
// may be shorter. Java 24's Gatherers.windowFixed.
func WindowFixed[T any](n int) Gatherer[T, []T, []T] {
	return Gatherer[T, []T, []T]{
		Init: func() []T { return make([]T, 0, n) },
		Integrate: func(window []T, item T, emit func([]T)) ([]T, bool) {
			window = append(window, item)
			if len(window) == n {
				emit(window)
				return make([]T, 0, n), true
			}
			return window, true
		},
		Finish: func(window []T, emit func([]T)) {
			if len(window) > 0 {
				emit(window)
			}
		},
	}
}

// WindowSliding produces every overlapping window of size n — fewer than n
// elements at the very start are not emitted. Java 24's
// Gatherers.windowSliding. Each window is a defensive copy, not a
// zero-copy aliased subslice the way Rust's slice::windows works — a
// deliberate safety-over-performance choice (mutating one window can
// never corrupt its neighbors or the source). Total copying is O(n ×
// size), not O(n) — revisit only if profiling shows it actually matters.
func WindowSliding[T any](n int) Gatherer[T, []T, []T] {
	return Gatherer[T, []T, []T]{
		Init: func() []T { return make([]T, 0, n) },
		Integrate: func(window []T, item T, emit func([]T)) ([]T, bool) {
			window = append(window, item)
			if len(window) > n {
				window = window[1:]
			}
			if len(window) == n {
				snapshot := make([]T, n)
				copy(snapshot, window)
				emit(snapshot)
			}
			return window, true
		},
	}
}

// --- Direct implementations ---
//
// These don't fit Gather's shape (one Stream in, process item-by-item,
// one Stream out) and stay hand-implemented, same tier as Filter/Map.

// SortBy returns a Stream sorted ascending by the key fn extracts. Stable
// — equal-key elements keep their original relative order, matching
// Rust's sort_by_key and Java's Stream.sorted (both documented stable);
// slices.SortFunc alone is not, so this uses SortStableFunc. Calls fn once
// per comparison (O(n log n) calls total) — use SortByCached if fn is
// non-trivial. Sorting needs the whole Stream before it can emit anything,
// unlike every Gatherer above which processes one item at a time.
func (s Stream[T]) SortBy[K cmp.Ordered](fn func(T) K) Stream[T] {
	cmpFn := func(a, b T) int { return cmp.Compare(fn(a), fn(b)) }
	return Stream[T]{items: slices.SortedStableFunc(slices.Values(s.items), cmpFn)}
}

// SortByDesc is SortBy in descending order.
func (s Stream[T]) SortByDesc[K cmp.Ordered](fn func(T) K) Stream[T] {
	cmpFn := func(a, b T) int { return cmp.Compare(fn(b), fn(a)) }
	return Stream[T]{items: slices.SortedStableFunc(slices.Values(s.items), cmpFn)}
}

// keyed pairs an extracted sort key with its source item — SortByCached's
// intermediate representation, so fn runs exactly once per element instead
// of once per comparison.
type keyed[T, K any] struct {
	key  K
	item T
}

// sortByCached is SortBy's shared engine: extract every key once, sort the
// (key, item) pairs by cmpFn, unwrap. Both SortByCached and
// SortByDescCached are this with the comparison direction flipped.
func sortByCached[T any, K cmp.Ordered](items []T, fn func(T) K, cmpFn func(a, b K) int) []T {
	pairs := make([]keyed[T, K], len(items))
	for i, v := range items {
		pairs[i] = keyed[T, K]{key: fn(v), item: v}
	}
	slices.SortStableFunc(pairs, func(a, b keyed[T, K]) int { return cmpFn(a.key, b.key) })
	out := make([]T, len(pairs))
	for i, p := range pairs {
		out[i] = p.item
	}
	return out
}

// SortByCached is SortBy, but extracts each key exactly once (O(n) calls
// to fn) instead of once per comparison (O(n log n)) — Rust's
// sort_by_cached_key. Prefer this over SortBy when fn is non-trivial (a
// computed property, not a field access); SortBy stays cheaper for
// trivial key functions since it skips the intermediate key slice.
func (s Stream[T]) SortByCached[K cmp.Ordered](fn func(T) K) Stream[T] {
	return Stream[T]{items: sortByCached(s.items, fn, func(a, b K) int { return cmp.Compare(a, b) })}
}

// SortByDescCached is SortByCached in descending order.
func (s Stream[T]) SortByDescCached[K cmp.Ordered](fn func(T) K) Stream[T] {
	return Stream[T]{items: sortByCached(s.items, fn, func(a, b K) int { return cmp.Compare(b, a) })}
}

// Partition splits into two Streams by fn — Java's
// Collectors.partitioningBy. Needs two output channels, not Gather's
// single emit-into-one-stream shape.
func (s Stream[T]) Partition(fn func(T) bool) (matched, unmatched Stream[T]) {
	var yes, no []T
	for _, v := range s.items {
		if fn(v) {
			yes = append(yes, v)
		} else {
			no = append(no, v)
		}
	}
	return Stream[T]{items: yes}, Stream[T]{items: no}
}

// Pair holds the elementwise combination of two Zipped Streams.
type Pair[A, B any] struct {
	First  A
	Second B
}

// Zip pairs elements from two Streams positionally, stopping at the
// shorter one — Rust/C++23's zip. Consumes two input Streams, not one.
func Zip[A, B any](sa Stream[A], sb Stream[B]) Stream[Pair[A, B]] {
	n := min(len(sa.items), len(sb.items))
	out := make([]Pair[A, B], n)
	for i := range n {
		out[i] = Pair[A, B]{First: sa.items[i], Second: sb.items[i]}
	}
	return Stream[Pair[A, B]]{items: out}
}
