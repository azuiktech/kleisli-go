package ds_test

import (
	"slices"
	"testing"

	"github.com/azuiktech/kleisli-go/ds"
)

func TestAdjList_Empty(t *testing.T) {
	g := ds.NewAdjList[string, int]()

	// Sizing
	if g.NodeCount() != 0 {
		t.Fatalf("expected 0 NodeCount, got %d", g.NodeCount())
	}
	if g.EdgeCount() != 0 {
		t.Fatalf("expected 0 EdgeCount, got %d", g.EdgeCount())
	}
	if g.OutDegree(ds.NodeID(0)) != 0 {
		t.Fatalf("expected 0 OutDegree on empty graph, got %d", g.OutDegree(ds.NodeID(0)))
	}
	if g.InDegree(ds.NodeID(0)) != 0 {
		t.Fatalf("expected 0 InDegree on empty graph, got %d", g.InDegree(ds.NodeID(0)))
	}

	// Nodes iteration on empty graph
	count := 0
	for range g.Nodes() {
		count++
	}
	if count != 0 {
		t.Fatalf("expected 0 nodes, got %d", count)
	}

	// Edges iteration on empty graph
	edgeCount := 0
	for range g.Edges() {
		edgeCount++
	}
	if edgeCount != 0 {
		t.Fatalf("expected 0 edges, got %d", edgeCount)
	}

	// OutEdges and InEdges for non-existent node
	outCount := 0
	for range g.OutEdges(ds.NodeID(0)) {
		outCount++
	}
	if outCount != 0 {
		t.Fatalf("expected 0 out edges, got %d", outCount)
	}

	inCount := 0
	for range g.InEdges(ds.NodeID(0)) {
		inCount++
	}
	if inCount != 0 {
		t.Fatalf("expected 0 in edges, got %d", inCount)
	}

	// Direct lookups
	if g.Node(ds.NodeID(0)).IsSome() {
		t.Fatal("expected Node(0) to be None")
	}
	if g.Edge(ds.EdgeID(0)).IsSome() {
		t.Fatal("expected Edge(0) to be None")
	}
}

func TestAdjList_AddNodeAndLookup(t *testing.T) {
	g := ds.NewAdjList[string, int]()

	n0 := g.AddNode("alpha")
	n1 := g.AddNode("beta")
	n2 := g.AddNode("gamma")

	if n0 != 0 || n1 != 1 || n2 != 2 {
		t.Fatalf("unexpected node IDs: %d, %d, %d", n0, n1, n2)
	}
	if g.NodeCount() != 3 {
		t.Fatalf("expected 3 NodeCount, got %d", g.NodeCount())
	}

	// Direct lookups
	optN0 := g.Node(n0)
	if optN0.IsNone() || optN0.MustGet() != "alpha" {
		t.Fatalf("unexpected node 0: %v", optN0)
	}
	optN1 := g.Node(n1)
	if optN1.IsNone() || optN1.MustGet() != "beta" {
		t.Fatalf("unexpected node 1: %v", optN1)
	}
	optN2 := g.Node(n2)
	if optN2.IsNone() || optN2.MustGet() != "gamma" {
		t.Fatalf("unexpected node 2: %v", optN2)
	}

	// Out of bounds lookups
	if g.Node(ds.NodeID(-1)).IsSome() {
		t.Fatal("expected None for negative node ID")
	}
	if g.Node(ds.NodeID(3)).IsSome() {
		t.Fatal("expected None for out of range node ID")
	}

	// Nodes() iterator
	var collectedIDs []ds.NodeID
	var collectedData []string
	for id, data := range g.Nodes() {
		collectedIDs = append(collectedIDs, id)
		collectedData = append(collectedData, data)
	}

	if !slices.Equal(collectedIDs, []ds.NodeID{0, 1, 2}) {
		t.Fatalf("unexpected node IDs: %v", collectedIDs)
	}
	if !slices.Equal(collectedData, []string{"alpha", "beta", "gamma"}) {
		t.Fatalf("unexpected node data: %v", collectedData)
	}
}

func TestAdjList_AddEdgeAndTraversal(t *testing.T) {
	// Build graph:
	// A(0) -> B(1) [edge 0: 100]
	// B(1) -> C(2) [edge 1: 200]
	// A(0) -> C(2) [edge 2: 300]
	g := ds.NewAdjList[string, int]()

	nA := g.AddNode("A")
	nB := g.AddNode("B")
	nC := g.AddNode("C")

	e0 := g.AddEdge(100, nA, nB)
	e1 := g.AddEdge(200, nB, nC)
	e2 := g.AddEdge(300, nA, nC)

	if e0 != 0 || e1 != 1 || e2 != 2 {
		t.Fatalf("unexpected edge IDs: %d, %d, %d", e0, e1, e2)
	}
	if g.EdgeCount() != 3 {
		t.Fatalf("expected 3 EdgeCount, got %d", g.EdgeCount())
	}

	// Degrees
	if g.OutDegree(nA) != 2 || g.InDegree(nA) != 0 {
		t.Fatalf("unexpected degree for nA: out=%d, in=%d", g.OutDegree(nA), g.InDegree(nA))
	}
	if g.OutDegree(nB) != 1 || g.InDegree(nB) != 1 {
		t.Fatalf("unexpected degree for nB: out=%d, in=%d", g.OutDegree(nB), g.InDegree(nB))
	}
	if g.OutDegree(nC) != 0 || g.InDegree(nC) != 2 {
		t.Fatalf("unexpected degree for nC: out=%d, in=%d", g.OutDegree(nC), g.InDegree(nC))
	}
	if g.OutDegree(ds.NodeID(-1)) != 0 || g.InDegree(ds.NodeID(99)) != 0 {
		t.Fatal("expected 0 degree for out-of-bounds node IDs")
	}

	// Direct edge lookups
	if g.Edge(e0).MustGet() != 100 {
		t.Fatalf("expected edge 0 to have 100, got %v", g.Edge(e0))
	}
	if g.Edge(e1).MustGet() != 200 {
		t.Fatalf("expected edge 1 to have 200, got %v", g.Edge(e1))
	}
	if g.Edge(e2).MustGet() != 300 {
		t.Fatalf("expected edge 2 to have 300, got %v", g.Edge(e2))
	}
	if g.Edge(ds.EdgeID(-1)).IsSome() || g.Edge(ds.EdgeID(99)).IsSome() {
		t.Fatal("expected None for out of bounds edge IDs")
	}

	// Src and Dst
	if g.Src(e0) != nA || g.Dst(e0) != nB {
		t.Fatalf("unexpected endpoints for e0: src=%d, dst=%d", g.Src(e0), g.Dst(e0))
	}
	if g.Src(e1) != nB || g.Dst(e1) != nC {
		t.Fatalf("unexpected endpoints for e1: src=%d, dst=%d", g.Src(e1), g.Dst(e1))
	}
	if g.Src(e2) != nA || g.Dst(e2) != nC {
		t.Fatalf("unexpected endpoints for e2: src=%d, dst=%d", g.Src(e2), g.Dst(e2))
	}

	// OutEdges of A
	var outAIDs []ds.EdgeID
	var outAData []int
	for eid, edata := range g.OutEdges(nA) {
		outAIDs = append(outAIDs, eid)
		outAData = append(outAData, edata)
	}
	if !slices.Equal(outAIDs, []ds.EdgeID{e0, e2}) || !slices.Equal(outAData, []int{100, 300}) {
		t.Fatalf("unexpected OutEdges(A): ids=%v, data=%v", outAIDs, outAData)
	}

	// InEdges of C
	var inCIDs []ds.EdgeID
	var inCData []int
	for eid, edata := range g.InEdges(nC) {
		inCIDs = append(inCIDs, eid)
		inCData = append(inCData, edata)
	}
	if !slices.Equal(inCIDs, []ds.EdgeID{e1, e2}) || !slices.Equal(inCData, []int{200, 300}) {
		t.Fatalf("unexpected InEdges(C): ids=%v, data=%v", inCIDs, inCData)
	}

	// InEdges of A should be empty
	inACount := 0
	for range g.InEdges(nA) {
		inACount++
	}
	if inACount != 0 {
		t.Fatalf("expected 0 InEdges for A, got %d", inACount)
	}

	// All Edges iterator
	var allEdges []ds.EdgeID
	for eid := range g.Edges() {
		allEdges = append(allEdges, eid)
	}
	if !slices.Equal(allEdges, []ds.EdgeID{e0, e1, e2}) {
		t.Fatalf("unexpected Edges(): %v", allEdges)
	}
}

func TestIncidenceList_Basics(t *testing.T) {
	g := ds.NewIncidenceList[string, float64]()

	if g.NodeCount() != 0 || g.EdgeCount() != 0 {
		t.Fatalf("expected 0 counts initially: nodes=%d, edges=%d", g.NodeCount(), g.EdgeCount())
	}

	n0 := g.AddNode("first")
	n1 := g.AddNode("second")

	e0 := g.AddEdge(3.14, n0, n1)

	if g.NodeCount() != 2 || g.EdgeCount() != 1 {
		t.Fatalf("expected 2 nodes, 1 edge: nodes=%d, edges=%d", g.NodeCount(), g.EdgeCount())
	}
	if g.OutDegree(n0) != 1 || g.OutDegree(n1) != 0 {
		t.Fatalf("unexpected out degree: n0=%d, n1=%d", g.OutDegree(n0), g.OutDegree(n1))
	}
	if g.OutDegree(ds.NodeID(99)) != 0 {
		t.Fatal("expected 0 out degree for out-of-bounds node")
	}

	// Direct lookups
	if g.Node(n0).MustGet() != "first" || g.Node(n1).MustGet() != "second" {
		t.Fatal("unexpected node data in IncidenceList")
	}
	if g.Edge(e0).MustGet() != 3.14 {
		t.Fatal("unexpected edge data in IncidenceList")
	}

	// Endpoints
	if g.Src(e0) != n0 || g.Dst(e0) != n1 {
		t.Fatalf("mismatched endpoints in IncidenceList: src=%d, dst=%d", g.Src(e0), g.Dst(e0))
	}

	// OutEdges of n0
	var out0 []ds.EdgeID
	for eid := range g.OutEdges(n0) {
		out0 = append(out0, eid)
	}
	if !slices.Equal(out0, []ds.EdgeID{e0}) {
		t.Fatalf("unexpected OutEdges(n0): %v", out0)
	}

	// OutEdges of n1 should be empty
	out1Count := 0
	for range g.OutEdges(n1) {
		out1Count++
	}
	if out1Count != 0 {
		t.Fatalf("expected 0 OutEdges for n1, got %d", out1Count)
	}
}

func TestGraph_InterfacePolymorphism(t *testing.T) {
	adj := ds.NewAdjList[string, string]()
	nA := adj.AddNode("NodeA")
	nB := adj.AddNode("NodeB")
	adj.AddEdge("A->B", nA, nB)

	inc := ds.NewIncidenceList[string, string]()
	n0 := inc.AddNode("Node0")
	n1 := inc.AddNode("Node1")
	inc.AddEdge("0->1", n0, n1)

	// Helper functions testing interface contracts
	getNodeCount := func(g ds.NodeListGraph[string]) int {
		return g.NodeCount()
	}

	getEdgeCount := func(g ds.EdgeListGraph[string]) int {
		return g.EdgeCount()
	}

	getOutDegree := func(g ds.IncidenceGraph[string], id ds.NodeID) int {
		return g.OutDegree(id)
	}

	getInDegree := func(g ds.BidirectionalGraph[string], id ds.NodeID) int {
		return g.InDegree(id)
	}

	// NodeListGraph
	if getNodeCount(adj) != 2 || getNodeCount(inc) != 2 {
		t.Fatal("NodeListGraph NodeCount polymorphism failed")
	}

	// EdgeListGraph
	if getEdgeCount(adj) != 1 || getEdgeCount(inc) != 1 {
		t.Fatal("EdgeListGraph EdgeCount polymorphism failed")
	}

	// IncidenceGraph
	if getOutDegree(adj, nA) != 1 || getOutDegree(inc, n0) != 1 {
		t.Fatal("IncidenceGraph OutDegree polymorphism failed")
	}
	if getOutDegree(adj, nB) != 0 || getOutDegree(inc, n1) != 0 {
		t.Fatal("IncidenceGraph OutDegree 0 check failed")
	}

	// BidirectionalGraph (only AdjList)
	if getInDegree(adj, nB) != 1 || getInDegree(adj, nA) != 0 {
		t.Fatal("BidirectionalGraph InDegree polymorphism failed")
	}
}

func TestGraph_EarlyBreakIterator(t *testing.T) {
	g := ds.NewAdjList[int, int]()
	for i := 0; i < 10; i++ {
		g.AddNode(i)
	}
	for i := 1; i < 10; i++ {
		g.AddEdge(i*10, ds.NodeID(0), ds.NodeID(i))
	}

	if g.NodeCount() != 10 || g.EdgeCount() != 9 {
		t.Fatalf("unexpected counts: nodes=%d, edges=%d", g.NodeCount(), g.EdgeCount())
	}
	if g.OutDegree(ds.NodeID(0)) != 9 {
		t.Fatalf("expected out-degree 9 for node 0, got %d", g.OutDegree(ds.NodeID(0)))
	}

	// Break early in Nodes()
	nodesCount := 0
	for id := range g.Nodes() {
		nodesCount++
		if id == 3 {
			break
		}
	}
	if nodesCount != 4 {
		t.Fatalf("expected early break after 4 nodes, got %d", nodesCount)
	}

	// Break early in Edges()
	edgesCount := 0
	for eid := range g.Edges() {
		edgesCount++
		if eid == 2 {
			break
		}
	}
	if edgesCount != 3 {
		t.Fatalf("expected early break after 3 edges, got %d", edgesCount)
	}

	// Break early in OutEdges()
	outCount := 0
	for eid := range g.OutEdges(ds.NodeID(0)) {
		outCount++
		if eid == 1 {
			break
		}
	}
	if outCount != 2 {
		t.Fatalf("expected early break after 2 out edges, got %d", outCount)
	}
}
