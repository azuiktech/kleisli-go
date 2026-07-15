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

// OrElse returns the success value, or fallback on failure.
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
