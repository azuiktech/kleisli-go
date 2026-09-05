package ds

import (
	"iter"
	"slices"

	"github.com/azuiktech/kleisli-go/adt"
	"github.com/azuiktech/kleisli-go/stream"
)

// Coord represents a 2D coordinate key (Row, Col).
type Coord[R comparable, C comparable] struct {
	Row R
	Col C
}

// Grid represents a 2D row-column coordinate collection.
// The main storage is a flat map of Coord[R, C] tuple keys to values.
// Secondary indexes use compact slices to map Row -> []Col and Col -> []Row.
type Grid[R comparable, C comparable, V any] struct {
	cells   map[Coord[R, C]]V // Main storage
	rowCols map[R][]C         // Secondary: Row -> []Col
	colRows map[C][]R         // Secondary: Col -> []Row
}

// NewGrid constructs an empty Grid.
func NewGrid[R comparable, C comparable, V any]() *Grid[R, C, V] {
	return &Grid[R, C, V]{
		cells:   make(map[Coord[R, C]]V),
		rowCols: make(map[R][]C),
		colRows: make(map[C][]R),
	}
}

// Put associates coordinate (r, c) with value v.
// Returns Some(previousValue) if replaced, or None if new.
func (g *Grid[R, C, V]) Put(r R, c C, v V) adt.Option[V] {
	k := Coord[R, C]{Row: r, Col: c}
	prev := adt.FromMap(g.cells, k)

	if prev.IsNone() {
		if cols, ok := g.rowCols[r]; !ok || !slices.Contains(cols, c) {
			g.rowCols[r] = append(g.rowCols[r], c)
		}
		if rows, ok := g.colRows[c]; !ok || !slices.Contains(rows, r) {
			g.colRows[c] = append(g.colRows[c], r)
		}
	}

	g.cells[k] = v
	return prev
}

// Get returns the value at coordinate (r, c), or None if absent.
func (g *Grid[R, C, V]) Get(r R, c C) adt.Option[V] {
	return adt.FromMap(g.cells, Coord[R, C]{Row: r, Col: c})
}

// Remove removes the value at coordinate (r, c).
// Returns Some(removedValue) if present, or None if absent.
func (g *Grid[R, C, V]) Remove(r R, c C) adt.Option[V] {
	k := Coord[R, C]{Row: r, Col: c}
	return adt.FromMap(g.cells, k).Tap(func(V) {
		delete(g.cells, k)

		adt.FromMap(g.rowCols, r).Tap(func(cols []C) {
			filtered := stream.Of(cols).Filter(func(col C) bool { return col != c }).Collect()
			if len(filtered) == 0 {
				delete(g.rowCols, r)
			} else {
				g.rowCols[r] = filtered
			}
		})

		adt.FromMap(g.colRows, c).Tap(func(rows []R) {
			filtered := stream.Of(rows).Filter(func(row R) bool { return row != r }).Collect()
			if len(filtered) == 0 {
				delete(g.colRows, c)
			} else {
				g.colRows[c] = filtered
			}
		})
	})
}

// Contains reports whether coordinate (r, c) exists in the grid.
func (g *Grid[R, C, V]) Contains(r R, c C) bool {
	return g.Get(r, c).IsSome()
}

// ContainsRow reports whether row r has any cells.
func (g *Grid[R, C, V]) ContainsRow(r R) bool {
	return adt.FromMap(g.rowCols, r).IsSome()
}

// ContainsCol reports whether column c has any cells.
func (g *Grid[R, C, V]) ContainsCol(c C) bool {
	return adt.FromMap(g.colRows, c).IsSome()
}

// All returns an iterator yielding all (Coord[R, C], V) pairs lazily with zero allocations.
func (g *Grid[R, C, V]) All() iter.Seq2[Coord[R, C], V] {
	return func(yield func(Coord[R, C], V) bool) {
		for k, v := range g.cells {
			if !yield(k, v) {
				return
			}
		}
	}
}

// Values returns an iterator yielding all values lazily with zero allocations.
func (g *Grid[R, C, V]) Values() iter.Seq[V] {
	return func(yield func(V) bool) {
		for _, v := range g.cells {
			if !yield(v) {
				return
			}
		}
	}
}

// Row returns an iterator yielding (Col, Val) pairs for row r lazily with zero allocations.
func (g *Grid[R, C, V]) Row(r R) iter.Seq2[C, V] {
	return func(yield func(C, V) bool) {
		if cols, ok := g.rowCols[r]; ok {
			for _, c := range cols {
				if v, ok := g.cells[Coord[R, C]{Row: r, Col: c}]; ok {
					if !yield(c, v) {
						return
					}
				}
			}
		}
	}
}

// Col returns an iterator yielding (Row, Val) pairs for column c lazily with zero allocations.
func (g *Grid[R, C, V]) Col(c C) iter.Seq2[R, V] {
	return func(yield func(R, V) bool) {
		if rows, ok := g.colRows[c]; ok {
			for _, r := range rows {
				if v, ok := g.cells[Coord[R, C]{Row: r, Col: c}]; ok {
					if !yield(r, v) {
						return
					}
				}
			}
		}
	}
}

// RowKeys returns an iterator yielding all distinct row keys lazily with zero allocations.
func (g *Grid[R, C, V]) RowKeys() iter.Seq[R] {
	return func(yield func(R) bool) {
		for r := range g.rowCols {
			if !yield(r) {
				return
			}
		}
	}
}

// ColKeys returns an iterator yielding all distinct column keys lazily with zero allocations.
func (g *Grid[R, C, V]) ColKeys() iter.Seq[C] {
	return func(yield func(C) bool) {
		for c := range g.colRows {
			if !yield(c) {
				return
			}
		}
	}
}

// Len returns the total number of cells in the grid.
func (g *Grid[R, C, V]) Len() int {
	return len(g.cells)
}

// Clear removes all cells and resets all secondary indexes.
func (g *Grid[R, C, V]) Clear() {
	clear(g.cells)
	clear(g.rowCols)
	clear(g.colRows)
}
