// Package fn provides stateless function utilities: value transforms (Ptr,
// Deref, Cond, Clamp, Coalesce, …), pure function composition (Fn, Fn2,
// Then, Fork, Identity), and algorithms over rotated sequences (FindRotated,
// After, Before).
//
// These three concerns lived in separate packages (value, tacit, algo) but
// share the same shape — pure functions with no side effects and no state —
// so they belong together.
package fn

import (
	"cmp"
	"fmt"
	"slices"
	"strings"
	"sync"
)

// ── Value helpers ─────────────────────────────────────────────────────────────

// Must returns val if err is nil; otherwise it panics with err.
func Must[T any](val T, err error) T {
	if err != nil {
		panic(err)
	}
	return val
}

// WrapErr returns nil if err is nil; otherwise it wraps err with format and args.
// If format does not contain %w, ": %w" is automatically appended to preserve the error chain.
func WrapErr(err error, format string, args ...any) error {
	if err == nil {
		return nil
	}
	if !strings.Contains(format, "%w") {
		format += ": %w"
		args = append(args, err)
	} else if strings.Count(format, "%")-strings.Count(format, "%%") > len(args) {
		args = append(args, err)
	}
	return fmt.Errorf(format, args...)
}

// MapErr returns (val, nil) if err is nil; otherwise it wraps err and returns (val, wrappedErr).
func MapErr[T any](val T, err error, format string, args ...any) (T, error) {
	if err == nil {
		return val, nil
	}
	return val, WrapErr(err, format, args...)
}

// Fallback returns val if err is nil; otherwise returns fallback.
func Fallback[T any](val T, err error, fallback T) T {
	if err != nil {
		return fallback
	}
	return val
}

// FallbackGet returns val if err is nil; otherwise calls fn with err to produce a fallback.
func FallbackGet[T any](val T, err error, fn func(error) T) T {
	if err != nil {
		return fn(err)
	}
	return val
}

// Cond returns ifTrue if condition is true; otherwise returns ifFalse.
// Both branches are evaluated before Cond is called. Use CondGet if evaluation is expensive.
func Cond[T any](condition bool, ifTrue T, ifFalse T) T {
	if condition {
		return ifTrue
	}
	return ifFalse
}

// CondGet evaluates and returns ifTrue() if condition is true; otherwise evaluates and returns ifFalse().
func CondGet[T any](condition bool, ifTrue func() T, ifFalse func() T) T {
	if condition {
		return ifTrue()
	}
	return ifFalse()
}

// Ptr returns a pointer to v.
func Ptr[T any](v T) *T { return &v }

// Deref returns *ptr if ptr is non-nil; otherwise returns fallback.
func Deref[T any](ptr *T, fallback T) T {
	if ptr != nil {
		return *ptr
	}
	return fallback
}

// DerefGet returns *ptr if ptr is non-nil; otherwise calls fn to produce a fallback.
func DerefGet[T any](ptr *T, fn func() T) T {
	if ptr != nil {
		return *ptr
	}
	return fn()
}

// DerefZero returns *ptr if ptr is non-nil; otherwise returns the zero value of T.
func DerefZero[T any](ptr *T) T {
	if ptr != nil {
		return *ptr
	}
	var zero T
	return zero
}

// Tap calls sideEffect(val) for side effects and returns val unchanged.
func Tap[T any](val T, sideEffect func(T)) T {
	sideEffect(val)
	return val
}

// Pipe passes val through f, returning the result.
func Pipe[A, B any](val A, f func(A) B) B { return f(val) }

// Zero returns the zero value of type T.
func Zero[T any]() T {
	var zero T
	return zero
}

// IsZero reports whether v equals the zero value of type T.
func IsZero[T comparable](v T) bool {
	var zero T
	return v == zero
}

// Clamp returns v if lo ≤ v ≤ hi, lo if v < lo, or hi if v > hi.
func Clamp[T cmp.Ordered](v, lo, hi T) T { return min(max(v, lo), hi) }

// Coalesce returns the first non-zero value from vals. Returns the zero value of T if all are zero.
func Coalesce[T comparable](vals ...T) T {
	var zero T
	for _, v := range vals {
		if v != zero {
			return v
		}
	}
	return zero
}

// ── Predicate combinators ─────────────────────────────────────────────────────

// Not negates a predicate: Not(f)(x) == !f(x).
func Not[T any](f func(T) bool) func(T) bool {
	return func(x T) bool { return !f(x) }
}

// And returns a predicate that is true when all of fs are true (short-circuits).
func And[T any](fs ...func(T) bool) func(T) bool {
	return func(x T) bool {
		for _, f := range fs {
			if !f(x) {
				return false
			}
		}
		return true
	}
}

// Or returns a predicate that is true when any of fs is true (short-circuits).
func Or[T any](fs ...func(T) bool) func(T) bool {
	return func(x T) bool {
		for _, f := range fs {
			if f(x) {
				return true
			}
		}
		return false
	}
}

// AllOf is an alias for And — reads naturally in filter chains.
func AllOf[T any](fs ...func(T) bool) func(T) bool { return And(fs...) }

// AnyOf is an alias for Or — reads naturally in filter chains.
func AnyOf[T any](fs ...func(T) bool) func(T) bool { return Or(fs...) }

// NoneOf returns a predicate that is true when none of fs are true.
func NoneOf[T any](fs ...func(T) bool) func(T) bool { return Not(Or(fs...)) }

// ── Bound predicates ──────────────────────────────────────────────────────────

// EqualTo returns a predicate that reports whether its argument equals x.
func EqualTo[T comparable](x T) func(T) bool { return func(v T) bool { return v == x } }

// NotEqualTo returns a predicate that reports whether its argument differs from x.
func NotEqualTo[T comparable](x T) func(T) bool { return func(v T) bool { return v != x } }

// LessThan returns a predicate that reports whether its argument is < x.
func LessThan[T cmp.Ordered](x T) func(T) bool { return func(v T) bool { return v < x } }

// LessThanOrEqual returns a predicate that reports whether its argument is ≤ x.
func LessThanOrEqual[T cmp.Ordered](x T) func(T) bool { return func(v T) bool { return v <= x } }

// GreaterThan returns a predicate that reports whether its argument is > x.
func GreaterThan[T cmp.Ordered](x T) func(T) bool { return func(v T) bool { return v > x } }

// GreaterThanOrEqual returns a predicate that reports whether its argument is ≥ x.
func GreaterThanOrEqual[T cmp.Ordered](x T) func(T) bool { return func(v T) bool { return v >= x } }

// In returns a predicate that reports whether its argument is present in set.
func In[T comparable](set ...T) func(T) bool {
	if len(set) <= 3 {
		return func(v T) bool { return slices.Contains(set, v) }
	}
	m := make(map[T]struct{}, len(set))
	for _, v := range set {
		m[v] = struct{}{}
	}
	return func(v T) bool {
		_, ok := m[v]
		return ok
	}
}

// NotIn returns a predicate that reports whether its argument is not present in set.
func NotIn[T comparable](set ...T) func(T) bool { return Not(In(set...)) }

// ── Function composition (tacit / point-free) ─────────────────────────────────

// Fn is a named unary function type — wrapping func(T) U lets Then attach as a method.
type Fn[T, U any] func(T) U

// Fn2 is a named binary function type — wrapping func(A, B) U lets Fork attach as a method.
type Fn2[A, B, U any] func(A, B) U

// Then composes f then g, left to right: f.Then(g)(x) = g(f(x)).
func (f Fn[T, U]) Then[V any](g Fn[U, V]) Fn[T, V] {
	return func(x T) V { return g(f(x)) }
}

// Fork applies f and g to the same argument, then combines their results with
// the receiver: combine.Fork(f, g)(x) = combine(f(x), g(x)).
func (combine Fn2[A, B, U]) Fork[T any](f Fn[T, A], g Fn[T, B]) Fn[T, U] {
	return func(x T) U { return combine(f(x), g(x)) }
}

// Identity returns its argument unchanged.
func Identity[T any](x T) T { return x }

// Constant returns a function that ignores its argument and always returns val.
func Constant[T, U any](val U) func(T) U {
	return func(T) U { return val }
}

// ── Memoization ───────────────────────────────────────────────────────────────

// Memoize returns a thread-safe memoized version of fn, called at most once per key.
func Memoize[K comparable, V any](fn func(K) V) func(K) V {
	var cache sync.Map
	return func(k K) V {
		getter := sync.OnceValue(func() V { return fn(k) })
		actual, _ := cache.LoadOrStore(k, getter)
		return actual.(func() V)()
	}
}

// MemoizeErr returns a thread-safe memoized version of a fallible fn, called at most once per key.
func MemoizeErr[K comparable, V any](fn func(K) (V, error)) func(K) (V, error) {
	var cache sync.Map
	return func(k K) (V, error) {
		getter := sync.OnceValues(func() (V, error) { return fn(k) })
		actual, _ := cache.LoadOrStore(k, getter)
		return actual.(func() (V, error))()
	}
}

// ── Rotated-sequence algorithms ───────────────────────────────────────────────

// FindRotated binary-searches two sorted partitions of a rotated sequence.
// first is the older/lower partition (s[pivot:]); second is the newer/higher
// partition (s[:pivot]). Logical order is first → second.
//
// Returns a logical index n:
//
//	n ∈ [0, len(first))                         — found in first at offset n
//	n ∈ [len(first), len(first)+len(second))    — found in second at offset n-len(first)
//	len(first)+len(second)                       — not found
//
// Aligns directly with ds.RingBuffer.Segments():
//
//	first, second := ring.Segments()
//	n := fn.FindRotated(first, second, val, comp)
func FindRotated[T, V any](first, second []T, val V, comp func(T, V) int) int {
	if i, ok := slices.BinarySearchFunc(first, val, comp); ok {
		return i
	}
	if i, ok := slices.BinarySearchFunc(second, val, comp); ok {
		return len(first) + i
	}
	return len(first) + len(second)
}

// After returns elements strictly after logical index n in circular order.
// n must be a valid index returned by FindRotated (not the not-found sentinel).
func After[T any](first, second []T, n int) ([]T, []T) {
	nf := len(first)
	if n < nf {
		return first[n+1:], second
	}
	return second[n-nf+1:], nil
}

// Before returns elements strictly before logical index n in circular order.
// n must be a valid index returned by FindRotated (not the not-found sentinel).
func Before[T any](first, second []T, n int) ([]T, []T) {
	nf := len(first)
	if n < nf {
		return first[:n], nil
	}
	m := n - nf
	if m == 0 {
		return first, nil
	}
	return first, second[:m]
}

// FindRotatedWithPivot searches s[pivot:] then s[:pivot] for val and returns
// a physical index into s. Returns len(s) when not found.
func FindRotatedWithPivot[T, V any](s []T, pivot int, val V, comp func(T, V) int) int {
	n := FindRotated(s[pivot:], s[:pivot], val, comp)
	total := len(s)
	if n == total {
		return total
	}
	nf := total - pivot
	if n < nf {
		return pivot + n
	}
	return n - nf
}

// AfterWithPivot returns elements strictly after physical index pos in
// circular order, treating s[pivot:] as the older partition.
func AfterWithPivot[T any](s []T, pos, pivot int) ([]T, []T) {
	n := pos - pivot
	if n < 0 {
		n += len(s)
	}
	return After(s[pivot:], s[:pivot], n)
}

// BeforeWithPivot returns elements strictly before physical index pos in
// circular order, treating s[pivot:] as the older partition.
func BeforeWithPivot[T any](s []T, pos, pivot int) ([]T, []T) {
	n := pos - pivot
	if n < 0 {
		n += len(s)
	}
	return Before(s[pivot:], s[:pivot], n)
}
