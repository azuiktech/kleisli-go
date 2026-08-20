package adt

import "sync"

// Lazy holds a value of type T that is computed at most once on first access.
type Lazy[T any] struct {
	get func() T
}

// Defer constructs a Lazy[T] that defers calling fn until Get is called.
// fn is executed at most once, thread-safely via sync.OnceValue.
// Named Defer to avoid the type/constructor name collision: Go does not allow
// a function and a type to share the same identifier at package scope.
func Defer[T any](fn func() T) Lazy[T] {
	return Lazy[T]{get: sync.OnceValue(fn)}
}

// DeferErr constructs a Lazy[Result[T]] from a Go-idiomatic (T, error) function,
// combining lazy evaluation and result composition.
func DeferErr[T any](fn func() (T, error)) Lazy[Result[T]] {
	return Defer(func() Result[T] {
		return From(fn())
	})
}

// Get returns the lazily evaluated value, evaluating fn on the first call.
// Safe to call on an uninitialized zero-value Lazy[T] (returns zero value of T).
func (l Lazy[T]) Get() T {
	if l.get == nil {
		var zero T
		return zero
	}
	return l.get()
}

// Map returns a new Lazy[U] that applies fn to the result of l.Get() when evaluated.
func (l Lazy[T]) Map[U any](fn func(T) U) Lazy[U] {
	return Defer(func() U {
		return fn(l.Get())
	})
}

// FlatMap returns a new Lazy[U] produced by chaining fn on l.Get() when evaluated.
func (l Lazy[T]) FlatMap[U any](fn func(T) Lazy[U]) Lazy[U] {
	return Defer(func() U {
		return fn(l.Get()).Get()
	})
}

// ToOption evaluates Get() and wraps the result in an Option[T].
func (l Lazy[T]) ToOption() Option[T] {
	return Opt(l.Get())
}

// ToResult evaluates Get() and converts via ToOption: a nil/absent value
// becomes Err(err), anything else becomes OK.
func (l Lazy[T]) ToResult(err error) Result[T] {
	return l.ToOption().ToResult(err)
}

// ToResultGet is ToResult with a lazy error producer — fn is only called
// when the value is absent.
func (l Lazy[T]) ToResultGet(fn func() error) Result[T] {
	return l.ToOption().ToResultGet(fn)
}

// Memoize returns a thread-safe memoized version of fn. fn is executed at
// most once per distinct key K, memoizing its output across concurrent calls.
func Memoize[K comparable, V any](fn func(K) V) func(K) V {
	var cache sync.Map
	return func(k K) V {
		getter := sync.OnceValue(func() V {
			return fn(k)
		})
		actual, _ := cache.LoadOrStore(k, getter)
		return actual.(func() V)()
	}
}

// MemoizeErr returns a thread-safe memoized version of a fallible function.
// fn is executed at most once per distinct key K, memoizing both value and error.
func MemoizeErr[K comparable, V any](fn func(K) (V, error)) func(K) (V, error) {
	var cache sync.Map
	return func(k K) (V, error) {
		getter := sync.OnceValues(func() (V, error) {
			return fn(k)
		})
		actual, _ := cache.LoadOrStore(k, getter)
		return actual.(func() (V, error))()
	}
}
