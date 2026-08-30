package stream

import "github.com/azuiktech/kleisli-go/adt"

// Number is the set of Go's built-in numeric types.
type Number interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64 |
		~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 | ~uintptr |
		~float32 | ~float64
}

// NumberStream is an eager numeric pipeline produced by Stream.MapToNumber.
// Its methods (Sum, Product, Mean, Min, Max) are available because the
// element type is constrained to Number.
type NumberStream[N Number] struct {
	inner Stream[N]
}

// MapToNumber projects each element to a numeric type N, returning a
// NumberStream that exposes Sum, Product, Mean, Min, Max.
func (s Stream[T]) MapToNumber[N Number](fn func(T) N) NumberStream[N] {
	return NumberStream[N]{inner: s.Map(fn)}
}

func (s NumberStream[N]) Sum() N {
	return s.inner.Reduce(N(0), func(acc, v N) N { return acc + v })
}

func (s NumberStream[N]) Product() N {
	return s.inner.Reduce(N(1), func(acc, v N) N { return acc * v })
}

func (s NumberStream[N]) Mean() adt.Option[float64] {
	items := s.inner.Collect()
	if len(items) == 0 {
		return adt.None[float64]()
	}
	total := Of(items).Reduce(float64(0), func(acc float64, v N) float64 { return acc + float64(v) })
	return adt.Some(total / float64(len(items)))
}

func (s NumberStream[N]) Min() adt.Option[N] {
	return s.inner.MinBy(func(v N) N { return v })
}

func (s NumberStream[N]) Max() adt.Option[N] {
	return s.inner.MaxBy(func(v N) N { return v })
}

// NumberSeq is a lazy numeric pipeline produced by Seq.MapToNumber.
type NumberSeq[N Number] struct {
	inner Seq[N]
}

// MapToNumber projects each element to a numeric type N, returning a
// NumberSeq that exposes Sum, Product, Mean, Min, Max.
func (s Seq[T]) MapToNumber[N Number](fn func(T) N) NumberSeq[N] {
	return NumberSeq[N]{inner: s.Map(fn)}
}

func (s NumberSeq[N]) Sum() N {
	return s.inner.Reduce(N(0), func(acc, v N) N { return acc + v })
}

func (s NumberSeq[N]) Product() N {
	return s.inner.Reduce(N(1), func(acc, v N) N { return acc * v })
}

func (s NumberSeq[N]) Mean() adt.Option[float64] {
	var count int
	total := s.inner.Reduce(float64(0), func(acc float64, v N) float64 {
		count++
		return acc + float64(v)
	})
	if count == 0 {
		return adt.None[float64]()
	}
	return adt.Some(total / float64(count))
}

func (s NumberSeq[N]) Min() adt.Option[N] {
	return s.inner.MinBy(func(v N) N { return v })
}

func (s NumberSeq[N]) Max() adt.Option[N] {
	return s.inner.MaxBy(func(v N) N { return v })
}
