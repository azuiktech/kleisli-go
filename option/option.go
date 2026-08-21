// Package option provides a generic Option[T] type that represents either a
// present, non-nil value or its absence — the replacement for a nil-pointer
// check. nil and absent are the same concept: a Some can never hold nil.
//
// Map, FlatMap, and Then use Go 1.27 generic methods, enabling clean
// method-chaining across type boundaries without nested free functions.
//
//	var p *User
//	name := option.From(p).
//	    Map(func(u *User) string { return u.Name }).
//	    OrElse("")
package option

import (
	json "encoding/json/v2"
	"fmt"
	"reflect"

	"github.com/azuiktech/kleisli-go/result"
	"github.com/azuiktech/kleisli-go/unit"
)



// Option holds either a present, non-nil value of type T or nothing. The
// two are mutually exclusive by construction (there is no way to construct
// an Option with ok true and a nil val), so ok alone is the discriminant —
// there's no separate tag to keep in sync.
// Use Some, None, or From to construct; never use the zero value directly
// (though the zero value does happen to equal None[T]()).
type Option[T any] struct {
	val T
	ok  bool
}

// Some wraps a present value. Panics if val is nil (a nil pointer,
// interface, map, slice, chan, or func) — nil and absent are the same
// concept for Option, so a Some holding nil would be a contradiction. Use
// None for absence, or From if val's nilness isn't already known to be
// checked.
func Some[T any](val T) Option[T] {
	if isNil(val) {
		panic("option: Some called with a nil value — use None, or From if val might be nil")
	}
	return Option[T]{val: val, ok: true}
}

// None represents absence.
func None[T any]() Option[T] { return Option[T]{} }

// Void returns a present Option carrying no value.
func Void() Option[unit.Unit] { return Some(unit.Unit{}) }

// From converts a possibly-nil value into an Option: nil becomes None,
// anything else becomes Some.
//
//	var p *User
//	option.From(p) // None
func From[T any](val T) Option[T] {
	if isNil(val) {
		return None[T]()
	}
	return Option[T]{val: val, ok: true}
}

// FromMap returns Some(val) if key is present in m and its value is non-nil;
// otherwise it returns None.
func FromMap[K comparable, V any](m map[K]V, key K) Option[V] {
	val, ok := m[key]
	if !ok {
		return None[V]()
	}
	return From(val)
}

// FromOk converts a Go-idiomatic (val, ok) pair (e.g. map lookup, channel receive,
// type assertion) into an Option: returns Some(val) if ok is true and val is
// non-nil; otherwise returns None.
func FromOk[T any](val T, ok bool) Option[T] {
	if !ok {
		return None[T]()
	}
	return From(val)
}

// FromNonZero returns Some(val) if val is non-zero (val != zero); otherwise it returns None.
func FromNonZero[T comparable](val T) Option[T] {
	var zero T
	if val == zero {
		return None[T]()
	}
	return Some(val)
}


// FromSlice returns Some(slice[idx]) if idx is within bounds [0, len(slice));
// otherwise it returns None.
func FromSlice[T any](slice []T, idx int) Option[T] {
	if idx < 0 || idx >= len(slice) {
		return None[T]()
	}
	return From(slice[idx])
}

// FromResult converts a result.Result[T] into an Option[T]: an error becomes
// None, and a success value becomes Some(val).
func FromResult[T any](r result.Result[T]) Option[T] {
	val, err := r.Unwrap()
	if err != nil {
		return None[T]()
	}
	return From(val)
}




func isNil(v any) bool {
	if v == nil {
		return true
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Ptr, reflect.Interface, reflect.Map, reflect.Slice, reflect.Chan, reflect.Func:
		return rv.IsNil()
	default:
		return false
	}
}

// ToPtr returns a pointer to the value, or nil if absent.
func (o Option[T]) ToPtr() *T {
	if !o.ok {
		return nil
	}
	val := o.val
	return &val
}

// MarshalJSON writes the value itself when present, or null when absent —
// safe precisely because Some can never hold nil, so there's no "present
// but nil" case that null could be confused with.
func (o Option[T]) MarshalJSON() ([]byte, error) {
	if !o.ok {
		return []byte("null"), nil
	}
	return json.Marshal(o.val)
}

// UnmarshalJSON reads null as None, and anything else as Some of the
// decoded value. Returns an error (not a panic — this is untrusted input,
// unlike Some) if the JSON decodes to a nil value despite not being null.
func (o *Option[T]) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		*o = None[T]()
		return nil
	}
	var val T
	if err := json.Unmarshal(data, &val); err != nil {
		return err
	}
	if isNil(val) {
		return fmt.Errorf("option: decoded a nil value from non-null JSON")
	}
	*o = Option[T]{val: val, ok: true}
	return nil
}

// IsSome reports whether the Option holds a value.
func (o Option[T]) IsSome() bool { return o.ok }

// IsNone reports whether the Option is empty.
func (o Option[T]) IsNone() bool { return !o.ok }

// Unwrap returns the underlying (value, present) pair, matching Go's
// standard comma-ok convention for direct use in callers.
func (o Option[T]) Unwrap() (T, bool) { return o.val, o.ok }

// MustGet returns the value or panics if absent.
// Intended for tests and initialisation code only.
func (o Option[T]) MustGet() T {
	if !o.ok {
		panic("option: MustGet called on None")
	}
	return o.val
}

// Expect returns the value or panics with msg if absent — the friendly
// alternative to MustGet for init/construction code, where a bare "called
// on None" panic isn't enough context to diagnose from.
func (o Option[T]) Expect(msg string) T {
	if !o.ok {
		panic("option: " + msg)
	}
	return o.val
}

// OrElse returns the value, or fallback if absent. fallback is evaluated
// by the caller before this is called, regardless of branch — Go has no
// way to defer argument evaluation — the same gotcha as Rust's unwrap_or
// and Java's Optional.orElse. Use OrElseGet if fallback is expensive to
// compute.
func (o Option[T]) OrElse(fallback T) T {
	if !o.ok {
		return fallback
	}
	return o.val
}

// OrElseGet calls fn to produce a fallback value if absent.
func (o Option[T]) OrElseGet(fn func() T) T {
	if !o.ok {
		return fn()
	}
	return o.val
}

// Or returns o if it holds a value; otherwise it returns fallback Option.
func (o Option[T]) Or(fallback Option[T]) Option[T] {
	if !o.ok {
		return fallback
	}
	return o
}

// ToResult converts the Option into a result.Result[T], returning result.OK(val)
// if present, or result.Err(err) if absent.
func (o Option[T]) ToResult(err error) result.Result[T] {
	if !o.ok {
		return result.Err[T](err)
	}
	return result.OK(o.val)
}

// ToResultGet converts the Option into a result.Result[T], returning result.OK(val)
// if present, or result.Err(fn()) if absent.
func (o Option[T]) ToResultGet(fn func() error) result.Result[T] {
	if !o.ok {
		return result.Err[T](fn())
	}
	return result.OK(o.val)
}

// ToSlice returns a single-element slice containing the value if present, or nil if absent.
func (o Option[T]) ToSlice() []T {
	if !o.ok {
		return nil
	}
	return []T{o.val}
}


// Filter keeps a present value only if fn reports true for it; otherwise
// (or if already absent) the result is None.
func (o Option[T]) Filter(fn func(T) bool) Option[T] {
	if o.ok && fn(o.val) {
		return o
	}
	return None[T]()
}

// Tap calls fn on the value for side effects (e.g. metrics, tracing) and
// passes the Option through unchanged.
func (o Option[T]) Tap(fn func(T)) Option[T] {
	if o.ok {
		fn(o.val)
	}
	return o
}

// Fold collapses the Option into a single value of type U by handling
// both branches — onSome for presence, onNone for absence. Equivalent to
// Haskell's maybe, and shorter than the equivalent Map(onSome).OrElseGet(onNone)
// two-step.
func (o Option[T]) Fold[U any](onSome func(T) U, onNone func() U) U {
	if !o.ok {
		return onNone()
	}
	return onSome(o.val)
}

// Map transforms the value into a different type. Absence propagates
// unchanged. Panics if fn returns nil — the same invariant Some enforces.
func (o Option[T]) Map[U any](fn func(T) U) Option[U] {
	if !o.ok {
		return None[U]()
	}
	return Some(fn(o.val))
}

// Map0 maps the Option to a new type by calling fn with no arguments,
// ignoring the current value. Absence propagates unchanged.
func (o Option[T]) Map0[U any](fn func() U) Option[U] {
	if !o.ok {
		return None[U]()
	}
	return Some(fn())
}

// FlatMap chains an Option-returning operation. Absence short-circuits: a
// None Option never calls fn.
func (o Option[T]) FlatMap[U any](fn func(T) Option[U]) Option[U] {
	if !o.ok {
		return None[U]()
	}
	return fn(o.val)
}

// Flatten collapses a nested Option[Option[T]] into a single Option[T].
func Flatten[T any](o Option[Option[T]]) Option[T] {
	if !o.ok {
		return None[T]()
	}
	return o.val
}

// Contains reports whether o is present and holds a value equal to target.
func Contains[T comparable](o Option[T], target T) bool {
	return o.ok && o.val == target
}


// Then chains a Go-idiomatic (U, bool)-returning function.
func (o Option[T]) Then[U any](fn func(T) (U, bool)) Option[U] {
	if !o.ok {
		return None[U]()
	}
	val, ok := fn(o.val)
	if !ok {
		return None[U]()
	}
	return From(val)
}

// Zip2 combines two independent Options into one via fn. Both must be
// present; the first absence (oa, then ob) short-circuits.
func Zip2[A, B, U any](oa Option[A], ob Option[B], fn func(A, B) U) Option[U] {
	return oa.FlatMap(func(a A) Option[U] {
		return ob.Map(func(b B) U { return fn(a, b) })
	})
}

// Zip3 combines three independent Options into one via fn. All three must
// be present; the first absence (oa, then ob, then oc) short-circuits.
func Zip3[A, B, C, U any](oa Option[A], ob Option[B], oc Option[C], fn func(A, B, C) U) Option[U] {
	return oa.FlatMap(func(a A) Option[U] {
		return ob.FlatMap(func(b B) Option[U] {
			return oc.Map(func(c C) U { return fn(a, b, c) })
		})
	})
}

// Sequence turns a slice of Options into an Option of a slice, fail-fast
// on the first absence encountered — Haskell's sequence/traverse for
// []Option.
func Sequence[T any](options []Option[T]) Option[[]T] {
	vals := make([]T, len(options))
	for i, o := range options {
		if !o.ok {
			return None[[]T]()
		}
		vals[i] = o.val
	}
	return Some(vals)
}

// Somes returns every present value, dropping absences. Unlike Sequence,
// this never fails: an Option slice with no present values returns an
// empty slice.
func Somes[T any](options []Option[T]) []T {
	vals := make([]T, 0, len(options))
	for _, o := range options {
		if o.ok {
			vals = append(vals, o.val)
		}
	}
	return vals
}
