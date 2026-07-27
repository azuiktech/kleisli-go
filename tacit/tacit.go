// Package tacit provides Fn[T, U] and Fn2[A, B, U] — named function types
// for point-free (tacit) function composition: building a new function
// entirely by combining existing ones, without naming an intermediate
// argument anywhere in the definition. The array-language ancestry (APL/
// J/BQN "trains") is direct — Fork is exactly a dyadic fork train,
// applying two functions to the same argument and combining their
// results with a third:
//
//	var StdDev = Subtract.Fork(Identity, Mean).Then(Square).Then(Mean).Then(Sqrt)
//
// This has nothing to do with stream's or async's own pipelines — no
// data flows through anything here, no goroutines, nothing executes
// until the composed Fn is finally called with an argument. It exists
// purely to build the func(T) U values stream.Map/async.Pipe.Parallel/
// etc. take as arguments; a composed Fn is directly assignable wherever
// a plain func(T) U is expected, no conversion needed.
package tacit

// Fn is a named unary function type — wrapping func(T) U lets Then
// attach as a method.
type Fn[T, U any] func(T) U

// Fn2 is a named binary function type — wrapping func(A, B) U lets Fork
// attach as a method.
type Fn2[A, B, U any] func(A, B) U

// Then composes f then g, left to right: f.Then(g)(x) = g(f(x)) —
// matches this library's own Result.Then/FlatMap naming rather than the
// array-language ∘'s right-to-left reading.
func (f Fn[T, U]) Then[V any](g Fn[U, V]) Fn[T, V] {
	return func(x T) V { return g(f(x)) }
}

// Fork applies f and g to the same argument, then combines their results
// with the receiver: combine.Fork(f, g)(x) = combine(f(x), g(x)). The
// array-language fork train (f g h) x = g(f x)(h x), with the combining
// function as the receiver instead of a third positional argument.
func (combine Fn2[A, B, U]) Fork[T any](f Fn[T, A], g Fn[T, B]) Fn[T, U] {
	return func(x T) U { return combine(f(x), g(x)) }
}

// Identity returns its argument unchanged — Fork's most common second
// argument, standing in for "the original input" alongside a derived
// one, e.g. Subtract.Fork(Identity, Mean) for x - mean(x).
func Identity[T any](x T) T { return x }
