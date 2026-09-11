package ds

import (
	"iter"

	"github.com/azuiktech/kleisli-go/adt"
)

type NodeID int
type EdgeID int

// EdgeListGraph is a graph interface providing iteration over all edges and total edge count.
type EdgeListGraph[E any] interface {
	Edges() iter.Seq2[EdgeID, E]
	EdgeCount() int
}

// NodeListGraph is a graph interface providing iteration over all nodes and total node count.
type NodeListGraph[N any] interface {
	Nodes() iter.Seq2[NodeID, N]
	NodeCount() int
}

// IncidenceGraph is a directed graph interface providing outgoing edges, degree, and endpoint queries.
type IncidenceGraph[E any] interface {
	OutEdges(id NodeID) iter.Seq2[EdgeID, E]
	OutDegree(id NodeID) int
	Src(id EdgeID) NodeID
	Dst(id EdgeID) NodeID
}

// BidirectionalGraph extends IncidenceGraph with incoming edges and in-degree queries.
type BidirectionalGraph[E any] interface {
	IncidenceGraph[E]
	InEdges(id NodeID) iter.Seq2[EdgeID, E]
	InDegree(id NodeID) int
}

type adjNode[N any] struct {
	outEdges []EdgeID
	inEdges  []EdgeID
	data     N
}

type adjEdge[E any] struct {
	src  NodeID
	dst  NodeID
	data E
}

type AdjList[N, E any] struct {
	nodes []adjNode[N]
	edges []adjEdge[E]
}

func NewAdjList[N, E any]() *AdjList[N, E] {
	return &AdjList[N, E]{}
}

func (a *AdjList[N, E]) AddNode(data N) NodeID {
	id := NodeID(len(a.nodes))
	a.nodes = append(a.nodes, adjNode[N]{
		data: data,
	})
	return id
}

func (a *AdjList[N, E]) AddEdge(data E, src, dst NodeID) EdgeID {
	id := EdgeID(len(a.edges))
	a.edges = append(a.edges, adjEdge[E]{
		src:  src,
		dst:  dst,
		data: data,
	})
	a.nodes[src].outEdges = append(a.nodes[src].outEdges, id)
	a.nodes[dst].inEdges = append(a.nodes[dst].inEdges, id)
	return id
}

func (a *AdjList[N, E]) NodeCount() int {
	return len(a.nodes)
}

func (a *AdjList[N, E]) EdgeCount() int {
	return len(a.edges)
}

func (a *AdjList[N, E]) Node(id NodeID) adt.Option[N] {
	if int(id) < 0 || int(id) >= len(a.nodes) {
		return adt.None[N]()
	}
	return adt.Some(a.nodes[id].data)
}

func (a *AdjList[N, E]) Edge(id EdgeID) adt.Option[E] {
	if int(id) < 0 || int(id) >= len(a.edges) {
		return adt.None[E]()
	}
	return adt.Some(a.edges[id].data)
}

func (a *AdjList[N, E]) Nodes() iter.Seq2[NodeID, N] {
	return func(yield func(NodeID, N) bool) {
		for i, n := range a.nodes {
			if !yield(NodeID(i), n.data) {
				return
			}
		}
	}
}

func (a *AdjList[N, E]) Edges() iter.Seq2[EdgeID, E] {
	return func(yield func(EdgeID, E) bool) {
		for i, e := range a.edges {
			if !yield(EdgeID(i), e.data) {
				return
			}
		}
	}
}

func (a *AdjList[N, E]) OutEdges(id NodeID) iter.Seq2[EdgeID, E] {
	return func(yield func(EdgeID, E) bool) {
		if int(id) < 0 || int(id) >= len(a.nodes) {
			return
		}
		for _, eID := range a.nodes[id].outEdges {
			if !yield(eID, a.edges[eID].data) {
				return
			}
		}
	}
}

func (a *AdjList[N, E]) InEdges(id NodeID) iter.Seq2[EdgeID, E] {
	return func(yield func(EdgeID, E) bool) {
		if int(id) < 0 || int(id) >= len(a.nodes) {
			return
		}
		for _, eID := range a.nodes[id].inEdges {
			if !yield(eID, a.edges[eID].data) {
				return
			}
		}
	}
}

func (a *AdjList[N, E]) OutDegree(id NodeID) int {
	if int(id) < 0 || int(id) >= len(a.nodes) {
		return 0
	}
	return len(a.nodes[id].outEdges)
}

func (a *AdjList[N, E]) InDegree(id NodeID) int {
	if int(id) < 0 || int(id) >= len(a.nodes) {
		return 0
	}
	return len(a.nodes[id].inEdges)
}

func (a *AdjList[N, E]) Src(id EdgeID) NodeID {
	return a.edges[id].src
}

func (a *AdjList[N, E]) Dst(id EdgeID) NodeID {
	return a.edges[id].dst
}

type incNode[N any] struct {
	outEdges []EdgeID
	data     N
}

type IncidenceList[N, E any] struct {
	nodes []incNode[N]
	edges []adjEdge[E]
}

func NewIncidenceList[N, E any]() *IncidenceList[N, E] {
	return &IncidenceList[N, E]{}
}

func (a *IncidenceList[N, E]) AddNode(data N) NodeID {
	id := NodeID(len(a.nodes))
	a.nodes = append(a.nodes, incNode[N]{
		data: data,
	})
	return id
}

func (a *IncidenceList[N, E]) AddEdge(data E, src, dst NodeID) EdgeID {
	id := EdgeID(len(a.edges))
	a.edges = append(a.edges, adjEdge[E]{
		src:  src,
		dst:  dst,
		data: data,
	})
	a.nodes[src].outEdges = append(a.nodes[src].outEdges, id)
	return id
}

func (a *IncidenceList[N, E]) NodeCount() int {
	return len(a.nodes)
}

func (a *IncidenceList[N, E]) EdgeCount() int {
	return len(a.edges)
}

func (a *IncidenceList[N, E]) Node(id NodeID) adt.Option[N] {
	if int(id) < 0 || int(id) >= len(a.nodes) {
		return adt.None[N]()
	}
	return adt.Some(a.nodes[id].data)
}

func (a *IncidenceList[N, E]) Edge(id EdgeID) adt.Option[E] {
	if int(id) < 0 || int(id) >= len(a.edges) {
		return adt.None[E]()
	}
	return adt.Some(a.edges[id].data)
}

func (a *IncidenceList[N, E]) Nodes() iter.Seq2[NodeID, N] {
	return func(yield func(NodeID, N) bool) {
		for i, n := range a.nodes {
			if !yield(NodeID(i), n.data) {
				return
			}
		}
	}
}

func (a *IncidenceList[N, E]) Edges() iter.Seq2[EdgeID, E] {
	return func(yield func(EdgeID, E) bool) {
		for i, e := range a.edges {
			if !yield(EdgeID(i), e.data) {
				return
			}
		}
	}
}

func (a *IncidenceList[N, E]) OutEdges(id NodeID) iter.Seq2[EdgeID, E] {
	return func(yield func(EdgeID, E) bool) {
		if int(id) < 0 || int(id) >= len(a.nodes) {
			return
		}
		for _, eID := range a.nodes[id].outEdges {
			if !yield(eID, a.edges[eID].data) {
				return
			}
		}
	}
}

func (a *IncidenceList[N, E]) OutDegree(id NodeID) int {
	if int(id) < 0 || int(id) >= len(a.nodes) {
		return 0
	}
	return len(a.nodes[id].outEdges)
}

func (a *IncidenceList[N, E]) Src(id EdgeID) NodeID {
	return a.edges[id].src
}

func (a *IncidenceList[N, E]) Dst(id EdgeID) NodeID {
	return a.edges[id].dst
}

// Compile-time interface satisfaction checks.
var (
	_ EdgeListGraph[int]      = (*AdjList[string, int])(nil)
	_ NodeListGraph[string]   = (*AdjList[string, int])(nil)
	_ IncidenceGraph[int]     = (*AdjList[string, int])(nil)
	_ BidirectionalGraph[int] = (*AdjList[string, int])(nil)

	_ EdgeListGraph[int]    = (*IncidenceList[string, int])(nil)
	_ NodeListGraph[string] = (*IncidenceList[string, int])(nil)
	_ IncidenceGraph[int]   = (*IncidenceList[string, int])(nil)
)
