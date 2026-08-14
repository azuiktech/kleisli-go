package examples

import (
	"errors"

	"github.com/azuiktech/kleisli-go/lazy"
)


// ============================================================================
// PROBLEM: Thread-Safe Memoized Function Cache (Algorithms / Optimization)
// ============================================================================
// Mini Problem Definition:
// 1. Compute expensive calculations (e.g. Fibonacci numbers or string hashing)
//    at most once per distinct input key.
// 2. Memoize fallible function evaluations (value and error) thread-safely using `lazy.MemoizeErr`.

// MemoizedFibonacci creates a thread-safe memoized Fibonacci calculator.
func MemoizedFibonacci() func(int) uint64 {
	var fib func(n int) uint64

	fib = lazy.Memoize(func(n int) uint64 {
		if n <= 1 {
			return uint64(n)
		}
		return fib(n-1) + fib(n-2)
	})

	return fib
}

// MemoizedFactorizer creates a thread-safe memoized fallible prime factorizer.
func MemoizedFactorizer() func(int) ([]int, error) {
	return lazy.MemoizeErr(func(n int) ([]int, error) {
		if n <= 1 {
			return nil, errors.New("cannot factorize numbers <= 1")
		}

		var factors []int
		d := 2
		temp := n
		for temp >= d*d {
			if temp%d == 0 {
				factors = append(factors, d)
				temp /= d
			} else {
				d++
			}
		}
		if temp > 1 {
			factors = append(factors, temp)
		}

		return factors, nil
	})
}
