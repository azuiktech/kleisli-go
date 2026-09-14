package adt

import "encoding/json"

// Pair represents an ordered Cartesian product of two values of types A and B.
type Pair[A, B any] struct {
	first  A
	second B
}

// PairOf constructs a Pair of first and second.
func PairOf[A, B any](first A, second B) Pair[A, B] {
	return Pair[A, B]{first: first, second: second}
}

// First returns the first element of the pair.
func (p Pair[A, B]) First() A {
	return p.first
}

// Second returns the second element of the pair.
func (p Pair[A, B]) Second() B {
	return p.second
}

// Swap returns a new Pair with components reversed.
func (p Pair[A, B]) Swap() Pair[B, A] {
	return PairOf(p.second, p.first)
}

// Unpack returns the components as a Go multiple return value.
func (p Pair[A, B]) Unpack() (A, B) {
	return p.first, p.second
}

// Key returns the first element, read as a map entry's key.
func (p Pair[A, B]) Key() A { return p.first }

// Value returns the second element, read as a map entry's value.
func (p Pair[A, B]) Value() B { return p.second }

func unmarshalJSON[T any](data []byte) Result[T] {
	var target T
	return From(target, json.Unmarshal(data, &target))
}

type wirePair[A, B any] struct {
	First  A `json:"first"`
	Second B `json:"second"`
}

// MarshalJSON encodes Pair as {"first":...,"second":...}.
func (p Pair[A, B]) MarshalJSON() ([]byte, error) {
	return From(json.Marshal(wirePair[A, B]{First: p.first, Second: p.second})).Unwrap()
}

// UnmarshalJSON decodes Pair from {"first":...,"second":...}.
func (p *Pair[A, B]) UnmarshalJSON(data []byte) error {
	return unmarshalJSON[wirePair[A, B]](data).Fold(
		func(w wirePair[A, B]) error {
			p.first, p.second = w.First, w.Second
			return nil
		},
		func(err error) error { return err },
	)
}

// Triple represents an ordered Cartesian product of three values of types A, B, and C.
type Triple[A, B, C any] struct {
	first  A
	second B
	third  C
}

// TripleOf constructs a Triple of first, second, and third.
func TripleOf[A, B, C any](first A, second B, third C) Triple[A, B, C] {
	return Triple[A, B, C]{first: first, second: second, third: third}
}

// First returns the first element of the triple.
func (t Triple[A, B, C]) First() A {
	return t.first
}

// Second returns the second element of the triple.
func (t Triple[A, B, C]) Second() B {
	return t.second
}

// Third returns the third element of the triple.
func (t Triple[A, B, C]) Third() C {
	return t.third
}

// Unpack returns the components as a Go multiple return value.
func (t Triple[A, B, C]) Unpack() (A, B, C) {
	return t.first, t.second, t.third
}

type wireTriple[A, B, C any] struct {
	First  A `json:"first"`
	Second B `json:"second"`
	Third  C `json:"third"`
}

// MarshalJSON encodes Triple as {"first":...,"second":...,"third":...}.
func (t Triple[A, B, C]) MarshalJSON() ([]byte, error) {
	return From(json.Marshal(wireTriple[A, B, C]{First: t.first, Second: t.second, Third: t.third})).Unwrap()
}

// UnmarshalJSON decodes Triple from {"first":...,"second":...,"third":...}.
func (t *Triple[A, B, C]) UnmarshalJSON(data []byte) error {
	return unmarshalJSON[wireTriple[A, B, C]](data).Fold(
		func(w wireTriple[A, B, C]) error {
			t.first, t.second, t.third = w.First, w.Second, w.Third
			return nil
		},
		func(err error) error { return err },
	)
}

// Keyed is Pair read as a map entry — same type, via Key()/Value() instead
// of First()/Second().
type Keyed[K, V any] = Pair[K, V]

// Indexed pairs a value with its position in the sequence that produced it —
// stream.Enumerate's and async.Pipe's Enumerate's element type. Unlike Pair
// and Triple, its fields are exported: callers construct and destructure it
// via plain struct literals (Indexed[T]{Index: i, Value: v}), so both
// packages can share this one type without a manual rewrap at the boundary.
type Indexed[T any] struct {
	Index int
	Value T
}
