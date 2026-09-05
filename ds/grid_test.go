package ds_test

import (
	"slices"
	"testing"

	"github.com/azuiktech/kleisli-go/ds"
)

func TestGrid_PutAndGet(t *testing.T) {
	g := ds.NewGrid[string, int, string]()

	// Put new entries (should return None)
	if prev := g.Put("server-1", 80, "http"); prev.IsSome() {
		t.Fatalf("expected None on new put, got %v", prev)
	}
	if prev := g.Put("server-1", 443, "https"); prev.IsSome() {
		t.Fatalf("expected None on new put, got %v", prev)
	}
	if prev := g.Put("server-2", 80, "http-alt"); prev.IsSome() {
		t.Fatalf("expected None on new put, got %v", prev)
	}

	if g.Len() != 3 {
		t.Fatalf("expected Len 3, got %d", g.Len())
	}

	// Get existing
	if got := g.Get("server-1", 80); !got.IsSome() || got.MustGet() != "http" {
		t.Fatalf("expected http, got %v", got)
	}
	if got := g.Get("server-1", 443); !got.IsSome() || got.MustGet() != "https" {
		t.Fatalf("expected https, got %v", got)
	}
	if got := g.Get("server-2", 80); !got.IsSome() || got.MustGet() != "http-alt" {
		t.Fatalf("expected http-alt, got %v", got)
	}

	// Get non-existing
	if got := g.Get("server-2", 443); got.IsSome() {
		t.Fatalf("expected None for non-existing cell, got %v", got)
	}
	if got := g.Get("ghost", 80); got.IsSome() {
		t.Fatalf("expected None for non-existing row, got %v", got)
	}

	// Overwrite existing (should return previous value)
	if prev := g.Put("server-1", 80, "http-v2"); !prev.IsSome() || prev.MustGet() != "http" {
		t.Fatalf("expected previous value http, got %v", prev)
	}
	if g.Len() != 3 {
		t.Fatalf("expected Len 3 after overwrite, got %d", g.Len())
	}
	if got := g.Get("server-1", 80); !got.IsSome() || got.MustGet() != "http-v2" {
		t.Fatalf("expected http-v2 after overwrite, got %v", got)
	}
}

func TestGrid_Contains(t *testing.T) {
	g := ds.NewGrid[string, string, int]()
	g.Put("r1", "c1", 100)
	g.Put("r1", "c2", 200)
	g.Put("r2", "c2", 300)

	if !g.Contains("r1", "c1") {
		t.Fatal("expected Contains(r1, c1) to be true")
	}
	if g.Contains("r2", "c1") {
		t.Fatal("expected Contains(r2, c1) to be false")
	}

	if !g.ContainsRow("r1") || !g.ContainsRow("r2") {
		t.Fatal("expected ContainsRow true for r1 and r2")
	}
	if g.ContainsRow("r3") {
		t.Fatal("expected ContainsRow false for r3")
	}

	if !g.ContainsCol("c1") || !g.ContainsCol("c2") {
		t.Fatal("expected ContainsCol true for c1 and c2")
	}
	if g.ContainsCol("c3") {
		t.Fatal("expected ContainsCol false for c3")
	}
}

func TestGrid_RowAndColIterators(t *testing.T) {
	g := ds.NewGrid[string, string, int]()
	g.Put("alice", "math", 95)
	g.Put("alice", "physics", 90)
	g.Put("bob", "math", 85)

	// Row iterator
	aliceScores := make(map[string]int)
	for col, val := range g.Row("alice") {
		aliceScores[col] = val
	}
	if len(aliceScores) != 2 || aliceScores["math"] != 95 || aliceScores["physics"] != 90 {
		t.Fatalf("unexpected alice scores: %v", aliceScores)
	}

	// Empty row iterator
	var count int
	for range g.Row("ghost") {
		count++
	}
	if count != 0 {
		t.Fatalf("expected 0 entries for ghost row, got %d", count)
	}

	// Col iterator
	mathScores := make(map[string]int)
	for row, val := range g.Col("math") {
		mathScores[row] = val
	}
	if len(mathScores) != 2 || mathScores["alice"] != 95 || mathScores["bob"] != 85 {
		t.Fatalf("unexpected math scores: %v", mathScores)
	}

	// Empty col iterator
	count = 0
	for range g.Col("ghost") {
		count++
	}
	if count != 0 {
		t.Fatalf("expected 0 entries for ghost col, got %d", count)
	}
}

func TestGrid_Remove(t *testing.T) {
	g := ds.NewGrid[string, int, string]()
	g.Put("node-1", 22, "ssh")
	g.Put("node-1", 80, "http")

	// Remove existing
	removed := g.Remove("node-1", 22)
	if !removed.IsSome() || removed.MustGet() != "ssh" {
		t.Fatalf("expected ssh removed, got %v", removed)
	}
	if g.Len() != 1 {
		t.Fatalf("expected Len 1, got %d", g.Len())
	}
	if g.Contains("node-1", 22) {
		t.Fatal("coordinate should not exist after remove")
	}
	if !g.ContainsCol(80) || g.ContainsCol(22) {
		t.Fatal("col 22 should be pruned, col 80 should remain")
	}

	// Remove non-existing
	absent := g.Remove("node-1", 22)
	if absent.IsSome() {
		t.Fatal("expected None when removing non-existing")
	}

	// Remove last cell of node-1 (should prune row key)
	g.Remove("node-1", 80)
	if g.Len() != 0 {
		t.Fatalf("expected Len 0, got %d", g.Len())
	}
	if g.ContainsRow("node-1") {
		t.Fatal("row node-1 should be pruned after removing all its cells")
	}
	if g.ContainsCol(80) {
		t.Fatal("col 80 should be pruned after removing all its cells")
	}
}

func TestGrid_AllAndValuesIterators(t *testing.T) {
	g := ds.NewGrid[string, string, int]()
	g.Put("r1", "c1", 10)
	g.Put("r1", "c2", 20)
	g.Put("r2", "c2", 30)

	// All() iter.Seq2
	collected := make(map[ds.Coord[string, string]]int)
	for coord, val := range g.All() {
		collected[coord] = val
	}
	if len(collected) != 3 {
		t.Fatalf("expected 3 entries in All(), got %d", len(collected))
	}
	if collected[ds.Coord[string, string]{Row: "r1", Col: "c1"}] != 10 {
		t.Fatalf("unexpected value for r1:c1")
	}

	// Values() iter.Seq
	var vals []int
	for val := range g.Values() {
		vals = append(vals, val)
	}
	slices.Sort(vals)
	if !slices.Equal(vals, []int{10, 20, 30}) {
		t.Fatalf("unexpected values: %v", vals)
	}

	// RowKeys()
	var rowKeys []string
	for r := range g.RowKeys() {
		rowKeys = append(rowKeys, r)
	}
	slices.Sort(rowKeys)
	if !slices.Equal(rowKeys, []string{"r1", "r2"}) {
		t.Fatalf("unexpected row keys: %v", rowKeys)
	}

	// ColKeys()
	var colKeys []string
	for c := range g.ColKeys() {
		colKeys = append(colKeys, c)
	}
	slices.Sort(colKeys)
	if !slices.Equal(colKeys, []string{"c1", "c2"}) {
		t.Fatalf("unexpected col keys: %v", colKeys)
	}
}

func TestGrid_Clear(t *testing.T) {
	g := ds.NewGrid[string, string, int]()
	g.Put("a", "b", 1)
	g.Put("c", "d", 2)

	g.Clear()

	if g.Len() != 0 {
		t.Fatalf("expected Len 0, got %d", g.Len())
	}
	if g.Contains("a", "b") || g.ContainsRow("a") || g.ContainsCol("b") {
		t.Fatal("grid should be empty after clear")
	}
}
