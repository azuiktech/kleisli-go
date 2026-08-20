package adt

import (
	"encoding/json"
	"fmt"
	"reflect"
)

// Option holds either a present, non-nil value of type T or nothing. The
// two are mutually exclusive by construction (there is no way to construct
// an Option with ok true and a nil val), so ok alone is the discriminant.
// Use Some, None, or Opt to construct; never use the zero value directly
// (though the zero value does happen to equal None[T]()).
type Option[T any] struct {
	val T
	ok  bool
}

// Some wraps a present value. Panics if val is nil.
func Some[T any](val T) Option[T] {
	if isNil(val) {
		panic("adt: Some called with a nil value — use None, or Opt if val might be nil")
	}
	return Option[T]{val: val, ok: true}
}

// None represents absence.
func None[T any]() Option[T] { return Option[T]{} }

// Opt converts a possibly-nil value into an Option: nil becomes None,
// anything else becomes Some. Named Opt (not From) to avoid collision with
// adt.From[T](val, err) which constructs a Result.
func Opt[T any](val T) Option[T] {
	if isNil(val) {
		return None[T]()
	}
	return Option[T]{val: val, ok: true}
}

// FromMap returns Some(val) if key is present in m and its value is non-nil;
// otherwise returns None.
func FromMap[K comparable, V any](m map[K]V, key K) Option[V] {
	val, ok := m[key]
	if !ok {
		return None[V]()
	}
	return Opt(val)
}

// FromOk converts a Go-idiomatic (val, ok) pair (e.g. map lookup, channel
// receive, type assertion) into an Option.
func FromOk[T any](val T, ok bool) Option[T] {
	if !ok {
		return None[T]()
	}
	return Opt(val)
}

// OptNonZero returns Some(val) if val is non-zero; otherwise returns None.
// Named OptNonZero (not FromNonZero) to avoid collision with adt.FromNonZero
// which constructs a Result from a (val, error) pair.
func OptNonZero[T comparable](val T) Option[T] {
	var zero T
	if val == zero {
		return None[T]()
	}
	return Some(val)
}

// FromSlice returns Some(slice[idx]) if idx is within bounds; otherwise None.
func FromSlice[T any](slice []T, idx int) Option[T] {
	if idx < 0 || idx >= len(slice) {
		return None[T]()
	}
	return Opt(slice[idx])
}

// FromResult converts a Result[T] into an Option[T]: error becomes None,
// success becomes Some(val). Also available as Result[T].ToOption().
func FromResult[T any](r Result[T]) Option[T] {
	return r.ToOption()
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

// MarshalJSON writes the value itself when present, or null when absent.
func (o Option[T]) MarshalJSON() ([]byte, error) {
	if !o.ok {
		return []byte("null"), nil
	}
	return json.Marshal(o.val)
}

// UnmarshalJSON reads null as None, and anything else as Some of the decoded
// value. Returns an error if the JSON decodes to a nil value despite not being null.
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
		return fmt.Errorf("adt: decoded a nil value from non-null JSON")
	}
	*o = Option[T]{val: val, ok: true}
	return nil
}

// IsSome reports whether the Option holds a value.
func (o Option[T]) IsSome() bool { return o.ok }

// IsNone reports whether the Option is empty.
func (o Option[T]) IsNone() bool { return !o.ok }

// Unwrap returns the underlying (value, present) pair.
func (o Option[T]) Unwrap() (T, bool) { return o.val, o.ok }

// MustGet returns the value or panics if absent. For tests and init only.
func (o Option[T]) MustGet() T {
	if !o.ok {
		panic("adt: MustGet called on None")
	}
	return o.val
}

// Expect returns the value or panics with msg if absent.
func (o Option[T]) Expect(msg string) T {
	if !o.ok {
		panic("adt: " + msg)
	}
	return o.val
}

// OrElse returns the value, or fallback if absent.
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

// Or returns o if it holds a value; otherwise returns fallback Option.
func (o Option[T]) Or(fallback Option[T]) Option[T] {
	if !o.ok {
		return fallback
	}
	return o
}

// ToResult converts the Option into a Result[T], returning OK(val) if
// present, or Err(err) if absent.
func (o Option[T]) ToResult(err error) Result[T] {
	if !o.ok {
		return Err[T](err)
	}
	return OK(o.val)
}

// ToResultGet converts the Option into a Result[T], returning OK(val) if
// present, or Err(fn()) if absent.
func (o Option[T]) ToResultGet(fn func() error) Result[T] {
	if !o.ok {
		return Err[T](fn())
	}
	return OK(o.val)
}

// ToSlice returns a single-element slice containing the value if present, or nil if absent.
func (o Option[T]) ToSlice() []T {
	if !o.ok {
		return nil
	}
	return []T{o.val}
}

// Filter keeps a present value only if fn reports true; otherwise returns None.
func (o Option[T]) Filter(fn func(T) bool) Option[T] {
	if o.ok && fn(o.val) {
		return o
	}
	return None[T]()
}

// Tap calls fn on the value for side effects and passes the Option through.
func (o Option[T]) Tap(fn func(T)) Option[T] {
	if o.ok {
		fn(o.val)
	}
	return o
}

// Fold collapses the Option into a single value of type U by handling both
// branches — onSome for presence, onNone for absence.
func (o Option[T]) Fold[U any](onSome func(T) U, onNone func() U) U {
	if !o.ok {
		return onNone()
	}
	return onSome(o.val)
}

// Map transforms the value into a different type. Absence propagates.
// Panics if fn returns nil — the same invariant Some enforces.
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

// FlatMap chains an Option-returning operation. Absence short-circuits.
func (o Option[T]) FlatMap[U any](fn func(T) Option[U]) Option[U] {
	if !o.ok {
		return None[U]()
	}
	return fn(o.val)
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
	return Opt(val)
}

// Somes returns every present value, dropping absences.
func Somes[T any](options []Option[T]) []T {
	vals := make([]T, 0, len(options))
	for _, o := range options {
		if o.ok {
			vals = append(vals, o.val)
		}
	}
	return vals
}

// optionNS is the receiver type for Option's conflicting free functions.
// Use via the Options variable: adt.Options.Zip2(...), adt.Options.Flatten(...).
type optionNS struct{}

// Options is the namespace for Option free functions that would conflict with
// their Result counterparts (Zip2, Zip3, Flatten, Contains, Sequence).
var Options = optionNS{}

// Zip2 combines two independent Options into one via fn. Both must be
// present; the first absence (oa, then ob) short-circuits.
func (optionNS) Zip2[A, B, U any](oa Option[A], ob Option[B], fn func(A, B) U) Option[U] {
	return oa.FlatMap(func(a A) Option[U] {
		return ob.Map(func(b B) U { return fn(a, b) })
	})
}

// Zip3 combines three independent Options into one via fn. All three must be
// present; the first absence (oa, then ob, then oc) short-circuits.
func (optionNS) Zip3[A, B, C, U any](oa Option[A], ob Option[B], oc Option[C], fn func(A, B, C) U) Option[U] {
	return oa.FlatMap(func(a A) Option[U] {
		return ob.FlatMap(func(b B) Option[U] {
			return oc.Map(func(c C) U { return fn(a, b, c) })
		})
	})
}

// Flatten collapses a nested Option[Option[T]] into a single Option[T].
func (optionNS) Flatten[T any](o Option[Option[T]]) Option[T] {
	if !o.ok {
		return None[T]()
	}
	return o.val
}

// Contains reports whether o is present and holds a value equal to target.
func (optionNS) Contains[T comparable](o Option[T], target T) bool {
	return o.ok && o.val == target
}

// Sequence turns a slice of Options into an Option of a slice, fail-fast on
// the first absence.
func (optionNS) Sequence[T any](options []Option[T]) Option[[]T] {
	vals := make([]T, len(options))
	for i, o := range options {
		if !o.ok {
			return None[[]T]()
		}
		vals[i] = o.val
	}
	return Some(vals)
}
