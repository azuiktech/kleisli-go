// Package result provides a generic Result[T] type that represents either a
// success value or an error.
//
// Map, FlatMap, and Then use Go 1.27 generic methods, enabling clean
// method-chaining across type boundaries without nested free functions.
//
//	user, err := result.From(verifier.Verify(ctx, token)).
//	    MapErr(wrapUnauthorized).
//	    Then(upsertUser).
//	    FlatMap(ensurePlan).
//	    Then(buildDTO).
//	    Unwrap()
package result

import "fmt"

// Result holds either a success value of type T or an error.
// Use OK, Err, or From to construct; never use the zero value directly.
type Result[T any] struct {
	val T
	err error
	ok  bool
}

// OK wraps a success value.
func OK[T any](val T) Result[T] { return Result[T]{val: val, ok: true} }

// Err wraps a failure. Panics if err is nil — use OK for success.
func Err[T any](err error) Result[T] {
	if err == nil {
		panic("result.Err called with nil error — use result.OK for success values")
	}
	return Result[T]{err: err}
}

// From converts a Go-idiomatic (val, error) pair.
func From[T any](val T, err error) Result[T] {
	if err != nil {
		return Err[T](err)
	}
	return OK(val)
}

// IsOK reports whether the Result holds a success value.
func (r Result[T]) IsOK() bool { return r.ok }

// IsErr reports whether the Result holds an error.
func (r Result[T]) IsErr() bool { return !r.ok }

// Val returns the success value, or the zero value of T on failure.
func (r Result[T]) Val() T { return r.val }

// Error returns the error, or nil on success.
func (r Result[T]) Error() error { return r.err }

// Unwrap returns the underlying (value, error) pair, matching Go's
// standard multi-return convention for direct use in callers.
func (r Result[T]) Unwrap() (T, error) { return r.val, r.err }

// MustGet returns the success value or panics with the error.
// Intended for tests and initialisation code only.
func (r Result[T]) MustGet() T {
	if !r.ok {
		panic(r.err)
	}
	return r.val
}

// Expect returns the success value or panics with msg wrapping the
// underlying error — the friendly alternative to MustGet for
// init/construction code, where a bare stack trace on the raw error isn't
// enough context to diagnose from. Prefer this over chaining
// MapErr(wrapErr(msg)).MustGet() for the same effect.
func (r Result[T]) Expect(msg string) T {
	if !r.ok {
		panic(fmt.Errorf("%s: %w", msg, r.err))
	}
	return r.val
}

// OrElse returns the success value, or fallback on failure. fallback is
// evaluated by the caller before this is called, regardless of branch —
// Go has no way to defer argument evaluation — the same gotcha as Rust's
// unwrap_or and Java's Optional.orElse. Use OrElseGet if fallback is
// expensive to compute.
func (r Result[T]) OrElse(fallback T) T {
	if !r.ok {
		return fallback
	}
	return r.val
}

// OrElseGet calls fn with the error to produce a fallback value on failure.
func (r Result[T]) OrElseGet(fn func(error) T) T {
	if !r.ok {
		return fn(r.err)
	}
	return r.val
}

// MapErr transforms the error, leaving a success Result unchanged.
// Use to annotate errors with context before they surface to callers.
func (r Result[T]) MapErr(fn func(error) error) Result[T] {
	if !r.ok {
		return Err[T](fn(r.err))
	}
	return r
}

// Recover turns a failure into a fallback Result via fn — the fallback may
// itself fail. A success Result passes through unchanged. Use for retry
// paths or default-value fallbacks that are themselves fallible; for a
// fallback that can't fail, use OrElse/OrElseGet instead.
func (r Result[T]) Recover(fn func(error) Result[T]) Result[T] {
	if !r.ok {
		return fn(r.err)
	}
	return r
}

// Tap calls fn on the success value for side effects (e.g. metrics, tracing)
// and passes the Result through unchanged.
func (r Result[T]) Tap(fn func(T)) Result[T] {
	if r.ok {
		fn(r.val)
	}
	return r
}

// TapErr calls fn on the error for side effects (e.g. logging)
// and passes the Result through unchanged.
func (r Result[T]) TapErr(fn func(error)) Result[T] {
	if !r.ok {
		fn(r.err)
	}
	return r
}

// Map transforms the success value into a different type.
// Errors propagate unchanged.
func (r Result[T]) Map[U any](fn func(T) U) Result[U] {
	if !r.ok {
		return Err[U](r.err)
	}
	return OK(fn(r.val))
}

// FlatMap chains a Result-returning operation.
// Errors short-circuit: a failed Result never calls fn.
func (r Result[T]) FlatMap[U any](fn func(T) Result[U]) Result[U] {
	if !r.ok {
		return Err[U](r.err)
	}
	return fn(r.val)
}

// Then chains a Go-idiomatic (U, error)-returning function.
func (r Result[T]) Then[U any](fn func(T) (U, error)) Result[U] {
	if !r.ok {
		return Err[U](r.err)
	}
	return From(fn(r.val))
}

// Fold collapses the Result into a single value of type U by handling
// both branches — onOK for success, onErr for failure. Equivalent to
// Haskell's either or Rust's map_or_else, and shorter than the equivalent
// Map(onOK).OrElseGet(onErr) two-step.
func (r Result[T]) Fold[U any](onOK func(T) U, onErr func(error) U) U {
	if !r.ok {
		return onErr(r.err)
	}
	return onOK(r.val)
}

// Zip2 combines two independent Results into one via fn. Both must
// succeed; the first failure (ra, then rb) short-circuits.
func Zip2[A, B, U any](ra Result[A], rb Result[B], fn func(A, B) U) Result[U] {
	return ra.FlatMap(func(a A) Result[U] {
		return rb.Map(func(b B) U { return fn(a, b) })
	})
}

// Zip3 combines three independent Results into one via fn. All three must
// succeed; the first failure (ra, then rb, then rc) short-circuits.
func Zip3[A, B, C, U any](ra Result[A], rb Result[B], rc Result[C], fn func(A, B, C) U) Result[U] {
	return ra.FlatMap(func(a A) Result[U] {
		return rb.FlatMap(func(b B) Result[U] {
			return rc.Map(func(c C) U { return fn(a, b, c) })
		})
	})
}

// Sequence turns a slice of Results into a Result of a slice, fail-fast on
// the first error encountered — Haskell's sequence/traverse for []Result.
func Sequence[T any](results []Result[T]) Result[[]T] {
	vals := make([]T, len(results))
	for i, r := range results {
		if !r.ok {
			return Err[[]T](r.err)
		}
		vals[i] = r.val
	}
	return OK(vals)
}

// Successes returns every success value, dropping errors — Haskell's
// rights. Unlike Sequence, this never fails: a Result slice with no
// successes returns an empty slice.
func Successes[T any](results []Result[T]) []T {
	vals := make([]T, 0, len(results))
	for _, r := range results {
		if r.ok {
			vals = append(vals, r.val)
		}
	}
	return vals
}

// Failures returns every error, dropping success values — Haskell's lefts.
func Failures[T any](results []Result[T]) []error {
	errs := make([]error, 0, len(results))
	for _, r := range results {
		if !r.ok {
			errs = append(errs, r.err)
		}
	}
	return errs
}
