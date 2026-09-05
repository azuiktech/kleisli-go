package ds

import (
	"slices"
	"sync/atomic"

	"github.com/azuiktech/kleisli-go/adt"
	"github.com/azuiktech/kleisli-go/stream"
)

var nextIndexID uint64

func allocIndexID() uint64 {
	return atomic.AddUint64(&nextIndexID, 1)
}

// Index represents an index specification on V.
type Index[V any] interface {
	indexID() uint64
	isUnique() bool
	createStorage() internalIndex[V]
}

type internalIndex[V any] interface {
	canInsert(v *V) bool
	insert(v *V)
	delete(v *V)
	contains(v *V) bool
	len() int
	clear()
}

// Unique defines a unique index on V by key K.
type Unique[V any, K comparable] struct {
	id      uint64
	extract func(*V) K
}

func (u Unique[V, K]) indexID() uint64 {
	return u.id
}

func (u Unique[V, K]) isUnique() bool {
	return true
}

func (u Unique[V, K]) createStorage() internalIndex[V] {
	return &uniqueStorage[V, K]{
		extract: u.extract,
		data:    make(map[K]*V),
	}
}

// NonUnique defines a non-unique secondary index on V by key K.
type NonUnique[V any, K comparable] struct {
	id      uint64
	extract func(*V) K
}

func (nu NonUnique[V, K]) indexID() uint64 {
	return nu.id
}

func (nu NonUnique[V, K]) isUnique() bool {
	return false
}

func (nu NonUnique[V, K]) createStorage() internalIndex[V] {
	return &nonUniqueStorage[V, K]{
		extract: nu.extract,
		data:    make(map[K][]*V),
	}
}

// UniqueIndex declares a unique index action for key extraction on *V.
func UniqueIndex[V any, K comparable](extract func(*V) K) Unique[V, K] {
	return Unique[V, K]{
		id:      allocIndexID(),
		extract: extract,
	}
}

// NonUniqueIndex declares a non-unique index action for key extraction on *V.
func NonUniqueIndex[V any, K comparable](extract func(*V) K) NonUnique[V, K] {
	return NonUnique[V, K]{
		id:      allocIndexID(),
		extract: extract,
	}
}

// uniqueStorage stores items indexed 1-to-1 by key K.
type uniqueStorage[V any, K comparable] struct {
	extract func(*V) K
	data    map[K]*V
}

func (s *uniqueStorage[V, K]) canInsert(v *V) bool {
	return adt.FromMap(s.data, s.extract(v)).IsNone()
}

func (s *uniqueStorage[V, K]) insert(v *V) {
	s.data[s.extract(v)] = v
}

func (s *uniqueStorage[V, K]) delete(v *V) {
	k := s.extract(v)
	adt.FromMap(s.data, k).
		Filter(func(existing *V) bool { return existing == v }).
		Tap(func(*V) { delete(s.data, k) })
}

func (s *uniqueStorage[V, K]) contains(v *V) bool {
	return adt.FromMap(s.data, s.extract(v)).
		Filter(func(existing *V) bool { return existing == v }).
		IsSome()
}

func (s *uniqueStorage[V, K]) len() int {
	return len(s.data)
}

func (s *uniqueStorage[V, K]) clear() {
	clear(s.data)
}

// nonUniqueStorage stores items indexed 1-to-many by key K.
type nonUniqueStorage[V any, K comparable] struct {
	extract func(*V) K
	data    map[K][]*V
}

func (s *nonUniqueStorage[V, K]) canInsert(v *V) bool {
	return true
}

func (s *nonUniqueStorage[V, K]) insert(v *V) {
	k := s.extract(v)
	s.data[k] = append(s.data[k], v)
}

func (s *nonUniqueStorage[V, K]) delete(v *V) {
	k := s.extract(v)
	adt.FromMap(s.data, k).Tap(func(items []*V) {
		filtered := stream.Of(items).Filter(func(item *V) bool { return item != v }).Collect()
		if len(filtered) == 0 {
			delete(s.data, k)
		} else {
			s.data[k] = filtered
		}
	})
}

func (s *nonUniqueStorage[V, K]) contains(v *V) bool {
	return adt.FromMap(s.data, s.extract(v)).
		Filter(func(items []*V) bool {
			return stream.Of(items).Any(func(item *V) bool { return item == v })
		}).
		IsSome()
}

func (s *nonUniqueStorage[V, K]) len() int {
	var total int
	for _, items := range s.data {
		total += len(items)
	}
	return total
}

func (s *nonUniqueStorage[V, K]) clear() {
	clear(s.data)
}

// UniqueView provides type-safe query operations for a unique index.
type UniqueView[V any, K comparable] interface {
	Find(key K) adt.Option[*V]
	Exists(key K) bool
}

type uniqueViewImpl[V any, K comparable] struct {
	storage *uniqueStorage[V, K]
}

func (uv *uniqueViewImpl[V, K]) Find(key K) adt.Option[*V] {
	return adt.Opt(uv.storage).FlatMap(func(s *uniqueStorage[V, K]) adt.Option[*V] {
		return adt.FromMap(s.data, key)
	})
}

func (uv *uniqueViewImpl[V, K]) Exists(key K) bool {
	return uv.Find(key).IsSome()
}

// NonUniqueView provides type-safe query operations for a non-unique index.
type NonUniqueView[V any, K comparable] interface {
	Find(key K) []*V
	Count(key K) int
}

type nonUniqueViewImpl[V any, K comparable] struct {
	storage *nonUniqueStorage[V, K]
}

func (nv *nonUniqueViewImpl[V, K]) Find(key K) []*V {
	return adt.Opt(nv.storage).
		FlatMap(func(s *nonUniqueStorage[V, K]) adt.Option[[]*V] {
			return adt.FromMap(s.data, key)
		}).
		Map(slices.Clone).
		OrElse(nil)
}

func (nv *nonUniqueViewImpl[V, K]) Count(key K) int {
	return len(nv.Find(key))
}

// Table represents an in-memory collection of *V where the first unique index acts as the main table.
type Table[V any] struct {
	main      internalIndex[V]
	mainID    uint64
	indexes   map[uint64]internalIndex[V]
	secondary []internalIndex[V]
}

// NewTable constructs a new multi-index Table over *V.
// The first argument must be a Unique index, which serves as the Primary Key for the main table.
func NewTable[V any, PK comparable](pk Unique[V, PK], secondary ...Index[V]) *Table[V] {
	mainStorage := pk.createStorage()
	t := &Table[V]{
		main:      mainStorage,
		mainID:    pk.id,
		indexes:   make(map[uint64]internalIndex[V], len(secondary)+1),
		secondary: make([]internalIndex[V], 0, len(secondary)),
	}
	t.indexes[pk.id] = mainStorage

	stream.Of(secondary).Each(func(idx Index[V]) {
		storage := idx.createStorage()
		t.indexes[idx.indexID()] = storage
		t.secondary = append(t.secondary, storage)
	})
	return t
}

// Insert adds a record to the table, atomically updating the main table and all secondary indexes.
// Returns false if the primary key or any secondary unique index constraint is violated.
func (t *Table[V]) Insert(v *V) bool {
	return adt.Opt(v).Filter(func(v *V) bool {
		// Validate main table primary key uniqueness
		if !t.main.canInsert(v) {
			return false
		}
		// Validate all secondary unique indexes
		return stream.Of(t.secondary).All(func(idx internalIndex[V]) bool {
			return idx.canInsert(v)
		})
	}).Map(func(v *V) bool {
		// Insert into main table (primary key)
		t.main.insert(v)
		// Insert into all secondary indexes
		stream.Of(t.secondary).Each(func(idx internalIndex[V]) {
			idx.insert(v)
		})
		return true
	}).OrElse(false)
}

// Delete removes a record from the table and all its indexes.
// Returns false if the record was not found in the main table.
func (t *Table[V]) Delete(v *V) bool {
	return adt.Opt(v).Filter(func(v *V) bool {
		return t.main.contains(v)
	}).Map(func(v *V) bool {
		t.main.delete(v)
		stream.Of(t.secondary).Each(func(idx internalIndex[V]) {
			idx.delete(v)
		})
		return true
	}).OrElse(false)
}

// Len returns the number of records currently stored in the main table.
func (t *Table[V]) Len() int {
	return t.main.len()
}

// Clear removes all records from the main table and all secondary indexes.
func (t *Table[V]) Clear() {
	t.main.clear()
	stream.Of(t.secondary).Each(func(idx internalIndex[V]) { idx.clear() })
}

// From binds the unique index to the given table, returning a type-safe UniqueView.
func (u Unique[V, K]) From(t *Table[V]) UniqueView[V, K] {
	return adt.Opt(t).
		FlatMap(func(tbl *Table[V]) adt.Option[internalIndex[V]] {
			return adt.FromMap(tbl.indexes, u.id)
		}).
		FlatMap(func(idx internalIndex[V]) adt.Option[*uniqueStorage[V, K]] {
			s, ok := idx.(*uniqueStorage[V, K])
			return adt.FromOk(s, ok)
		}).
		Map(func(s *uniqueStorage[V, K]) UniqueView[V, K] {
			return &uniqueViewImpl[V, K]{storage: s}
		}).
		OrElse(&uniqueViewImpl[V, K]{storage: nil})
}

// Find is a direct convenience action: ByID.Find(table, key).
func (u Unique[V, K]) Find(t *Table[V], key K) adt.Option[*V] {
	return u.From(t).Find(key)
}

// From binds the non-unique index to the given table, returning a type-safe NonUniqueView.
func (nu NonUnique[V, K]) From(t *Table[V]) NonUniqueView[V, K] {
	return adt.Opt(t).
		FlatMap(func(tbl *Table[V]) adt.Option[internalIndex[V]] {
			return adt.FromMap(tbl.indexes, nu.id)
		}).
		FlatMap(func(idx internalIndex[V]) adt.Option[*nonUniqueStorage[V, K]] {
			s, ok := idx.(*nonUniqueStorage[V, K])
			return adt.FromOk(s, ok)
		}).
		Map(func(s *nonUniqueStorage[V, K]) NonUniqueView[V, K] {
			return &nonUniqueViewImpl[V, K]{storage: s}
		}).
		OrElse(&nonUniqueViewImpl[V, K]{storage: nil})
}

// Find is a direct convenience action: ByZipCode.Find(table, key).
func (nu NonUnique[V, K]) Find(t *Table[V], key K) []*V {
	return nu.From(t).Find(key)
}

// By provides functional syntax: ds.By(table, ByID).Find(key)
func By[V any, K comparable](t *Table[V], idx Unique[V, K]) UniqueView[V, K] {
	return idx.From(t)
}

// ByGroup provides functional syntax: ds.ByGroup(table, ByZipCode).Find(key)
func ByGroup[V any, K comparable](t *Table[V], idx NonUnique[V, K]) NonUniqueView[V, K] {
	return idx.From(t)
}
