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

import (
	json "encoding/json/v2"
	"encoding/json/jsontext"
	"errors"
	"fmt"
	"strings"

	"github.com/azuiktech/kleisli-go/unit"
)




// Result holds either a success value of type T or an error. The two are
// mutually exclusive by construction (Err refuses a nil error, OK never sets
// one), so err's nilness alone is the discriminant — there's no separate
// tag to keep in sync.
// Use OK, Err, or From to construct; never use the zero value directly.
type Result[T any] struct {
	val T
	err error
}

// OK wraps a success value.
func OK[T any](val T) Result[T] { return Result[T]{val: val} }

// Err wraps a failure. Panics if err is nil — use OK for success.
func Err[T any](err error) Result[T] {
	if err == nil {
		panic("result.Err called with nil error — use result.OK for success values")
	}
	return Result[T]{err: err}
}

// Void returns a successful Result carrying no value.
func Void() Result[unit.Unit] { return OK(unit.Unit{}) }

// From converts a Go-idiomatic (val, error) pair.
func From[T any](val T, err error) Result[T] {
	if err != nil {
		return Err[T](err)
	}
	return OK(val)
}

// FromNonZero returns OK(val) if val is non-zero (val != zero); otherwise it returns Err[T](err).
func FromNonZero[T comparable](val T, err error) Result[T] {
	var zero T
	if val == zero {
		return Err[T](err)
	}
	return OK(val)
}


// wireResult is Result[T]'s JSON wire shape — Rust serde's externally
// tagged representation for Result<T, E> ({"Ok": T} / {"Err": E}), adapted
// to Go's lowercase field convention. Exactly one key is ever present: the
// key itself is the discriminant, mirroring Result's own invariant that
// exactly one of val/err is ever active.
//
// Ok is *T, not T, so that a success value which is itself a nil pointer
// (e.g. Result[*Invoice] holding "no match") still round-trips as
// {"ok":null} — distinct from {"err":"..."} — rather than being silently
// omitted by omitempty.
type wireOK[T any] struct {
	Ok *T `json:"ok"`
}

type wireErr struct {
	Err string `json:"err"`
}

// MarshalJSON writes the Result as {"ok":<val>} or {"err":"<message>"}.
func (r Result[T]) MarshalJSON() ([]byte, error) {
	if r.err != nil {
		return json.Marshal(wireErr{Err: r.err.Error()})
	}
	return json.Marshal(wireOK[T]{Ok: &r.val})
}


// UnmarshalJSON reads back the shape MarshalJSON writes. The reconstructed
// error is a plain errors.New(message) — the original error's identity and
// wrapping chain don't survive the round-trip, only its message text.
//
// This can't reuse wireResult the way MarshalJSON does: encoding/json
// collapses a JSON null into a nil pointer at whatever level it's
// unmarshaling into, so a *T field can't tell "ok present, value null"
// (a real success holding a nil T, e.g. Result[*Invoice]'s "no match")
// apart from "ok absent". Unmarshaling into a raw key map first, and
// checking key presence directly, avoids that collapse.
func (r *Result[T]) UnmarshalJSON(data []byte) error {
	var keys map[string]jsontext.Value
	if err := json.Unmarshal(data, &keys); err != nil {
		return err
	}
	if raw, present := keys["err"]; present {
		var msg string
		if err := json.Unmarshal(raw, &msg); err != nil {
			return err
		}
		*r = Err[T](errors.New(msg))
		return nil
	}
	if raw, present := keys["ok"]; present {
		var val T
		if err := json.Unmarshal(raw, &val); err != nil {
			return err
		}
		*r = OK(val)
		return nil
	}
	return fmt.Errorf(`result: JSON has neither "ok" nor "err" key`)
}

// IsOK reports whether the Result holds a success value.
func (r Result[T]) IsOK() bool { return r.err == nil }

// IsErr reports whether the Result holds an error.
func (r Result[T]) IsErr() bool { return r.err != nil }

// Unwrap returns the underlying (value, error) pair, matching Go's
// standard multi-return convention for direct use in callers.
func (r Result[T]) Unwrap() (T, error) { return r.val, r.err }


// MustGet returns the success value or panics with the error.
// Intended for tests and initialisation code only.
func (r Result[T]) MustGet() T {
	if r.err != nil {
		panic(r.err)
	}
	return r.val
}

// MustErr returns the error or panics if the Result is OK.
// Intended for tests and template rendering where error presence is guarded by IsErr().
func (r Result[T]) MustErr() error {
	if r.err == nil {
		panic("result.MustErr called on OK Result")
	}
	return r.err
}


// Expect returns the success value or panics with msg wrapping the
// underlying error — the friendly alternative to MustGet for
// init/construction code, where a bare stack trace on the raw error isn't
// enough context to diagnose from. Prefer this over chaining
// MapErr(wrapErr(msg)).MustGet() for the same effect.
func (r Result[T]) Expect(msg string) T {
	if r.err != nil {
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
	if r.err != nil {
		return fallback
	}
	return r.val
}

// OrElseGet calls fn with the error to produce a fallback value on failure.
func (r Result[T]) OrElseGet(fn func(error) T) T {
	if r.err != nil {
		return fn(r.err)
	}
	return r.val
}

// Or returns r if it holds a success value; otherwise it returns fallback Result.
func (r Result[T]) Or(fallback Result[T]) Result[T] {
	if r.err != nil {
		return fallback
	}
	return r
}


// MapErr transforms the error, leaving a success Result unchanged.
// Use to annotate errors with context before they surface to callers.
func (r Result[T]) MapErr(fn func(error) error) Result[T] {
	if r.err != nil {
		return Err[T](fn(r.err))
	}
	return r
}

func wrapErr(err error, format string, args ...any) error {
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

// MapErrf annotates an error with a formatted context string if Result holds an error.
// If format does not contain %w, ": %w" is automatically appended along with the underlying error,
// preserving the error wrapping chain for errors.Is/errors.As.
func (r Result[T]) MapErrf(format string, args ...any) Result[T] {
	if r.err == nil {
		return r
	}
	return Err[T](wrapErr(r.err, format, args...))
}

// WrapErr is an alias for MapErrf.
func (r Result[T]) WrapErr(format string, args ...any) Result[T] {
	return r.MapErrf(format, args...)
}


// Recover turns a failure into a fallback Result via fn — the fallback may
// itself fail. A success Result passes through unchanged. Use for retry
// paths or default-value fallbacks that are themselves fallible; for a
// fallback that can't fail, use OrElse/OrElseGet instead.
func (r Result[T]) Recover(fn func(error) Result[T]) Result[T] {
	if r.err != nil {
		return fn(r.err)
	}
	return r
}

// Tap calls fn on the success value for side effects (e.g. metrics, tracing)
// and passes the Result through unchanged.
func (r Result[T]) Tap(fn func(T)) Result[T] {
	if r.err == nil {
		fn(r.val)
	}
	return r
}

// TapErr calls fn on the error for side effects (e.g. logging)
// and passes the Result through unchanged.
func (r Result[T]) TapErr(fn func(error)) Result[T] {
	if r.err != nil {
		fn(r.err)
	}
	return r
}

// Map transforms the success value into a different type.
// Errors propagate unchanged.
func (r Result[T]) Map[U any](fn func(T) U) Result[U] {
	if r.err != nil {
		return Err[U](r.err)
	}
	return OK(fn(r.val))
}

// Map0 maps the Result to a new type by calling fn with no arguments,
// ignoring the current value. Errors propagate unchanged.
func (r Result[T]) Map0[U any](fn func() U) Result[U] {
	if r.err != nil {
		return Err[U](r.err)
	}
	return OK(fn())
}

// FlatMap chains a Result-returning operation.
// Errors short-circuit: a failed Result never calls fn.
func (r Result[T]) FlatMap[U any](fn func(T) Result[U]) Result[U] {
	if r.err != nil {
		return Err[U](r.err)
	}
	return fn(r.val)
}

// Flatten collapses a nested Result[Result[T]] into a single Result[T].
func Flatten[T any](r Result[Result[T]]) Result[T] {
	if r.err != nil {
		return Err[T](r.err)
	}
	return r.val
}

// Contains reports whether r holds a success value equal to target.
func Contains[T comparable](r Result[T], target T) bool {
	return r.err == nil && r.val == target
}


// Then chains a Go-idiomatic (U, error)-returning function.
func (r Result[T]) Then[U any](fn func(T) (U, error)) Result[U] {
	if r.err != nil {
		return Err[U](r.err)
	}
	return From(fn(r.val))
}

// Fold collapses the Result into a single value of type U by handling
// both branches — onOK for success, onErr for failure. Equivalent to
// Haskell's either or Rust's map_or_else, and shorter than the equivalent
// Map(onOK).OrElseGet(onErr) two-step.
func (r Result[T]) Fold[U any](onOK func(T) U, onErr func(error) U) U {
	if r.err != nil {
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
		if r.err != nil {
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
		if r.err == nil {
			vals = append(vals, r.val)
		}
	}
	return vals
}

// Failures returns every error, dropping success values — Haskell's lefts.
func Failures[T any](results []Result[T]) []error {
	errs := make([]error, 0, len(results))
	for _, r := range results {
		if r.err != nil {
			errs = append(errs, r.err)
		}
	}
	return errs
}
