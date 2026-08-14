// Package value provides lightweight value-to-value transformation,
// pointer manipulation, ternary, and fallback helpers.
package value

import (
	"fmt"
	"strings"
)


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

// MapErr returns (val, nil) if err is nil; otherwise it wraps err with format and args
// and returns (val, wrappedErr).
func MapErr[T any](val T, err error, format string, args ...any) (T, error) {
	if err == nil {
		return val, nil
	}
	return val, WrapErr(err, format, args...)
}


// Fallback returns val if err is nil; otherwise it returns fallback.
func Fallback[T any](val T, err error, fallback T) T {
	if err != nil {
		return fallback
	}
	return val
}

// FallbackGet returns val if err is nil; otherwise it calls fn with err
// to produce a fallback value.
func FallbackGet[T any](val T, err error, fn func(error) T) T {
	if err != nil {
		return fn(err)
	}
	return val
}

// Cond returns ifTrue if condition is true; otherwise it returns ifFalse.
// Both ifTrue and ifFalse are evaluated by the caller before Cond is called.
// Use CondGet if branch evaluation is expensive.
func Cond[T any](condition bool, ifTrue T, ifFalse T) T {
	if condition {
		return ifTrue
	}
	return ifFalse
}

// CondGet evaluates and returns ifTrue() if condition is true; otherwise
// it evaluates and returns ifFalse().
func CondGet[T any](condition bool, ifTrue func() T, ifFalse func() T) T {
	if condition {
		return ifTrue()
	}
	return ifFalse()
}

// Ptr returns a pointer to v.
func Ptr[T any](v T) *T {
	return &v
}

// Deref returns *ptr if ptr is non-nil; otherwise it returns fallback.
func Deref[T any](ptr *T, fallback T) T {
	if ptr != nil {
		return *ptr
	}
	return fallback
}

// DerefGet returns *ptr if ptr is non-nil; otherwise it calls fn to produce
// a fallback value.
func DerefGet[T any](ptr *T, fn func() T) T {
	if ptr != nil {
		return *ptr
	}
	return fn()
}

// DerefZero returns *ptr if ptr is non-nil; otherwise it returns the zero value of T.
func DerefZero[T any](ptr *T) T {
	if ptr != nil {
		return *ptr
	}
	var zero T
	return zero
}

// Tap calls sideEffect(val) for side effects (e.g. metrics, logging) and
// returns val unchanged.
func Tap[T any](val T, sideEffect func(T)) T {
	sideEffect(val)
	return val
}

// Pipe passes val through fn, returning the result.
func Pipe[A, B any](val A, fn func(A) B) B {
	return fn(val)
}

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

// Coalesce returns the first non-zero value from vals. If all elements
// are zero values (or vals is empty), it returns the zero value of T.
func Coalesce[T comparable](vals ...T) T {
	var zero T
	for _, v := range vals {
		if v != zero {
			return v
		}
	}
	return zero
}
