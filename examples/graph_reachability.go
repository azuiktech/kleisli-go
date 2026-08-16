package examples

import (
	"fmt"

	"github.com/azuiktech/kleisli-go/result"
	"github.com/azuiktech/kleisli-go/stream"
)

// ============================================================================
// PROBLEM: Directed Graph Reachability & Transition Auditor (Algorithms / Graph)
// ============================================================================
// Mini Problem Definition:
// Given a directed workflow graph composed of node names, fork flags, and transition targets:
// 1. Audit node configurations: ensure no node is declared as both a Fork and ordinary transition source.
// 2. Traversal audit: verify all nodes in the registry are reachable from `startNode`.
// 3. Return `result.Result[GraphAudit]` with clean error reporting.

type GraphNodeSpec struct {
	Name        string
	IsFork      bool
	Transitions []string
}

type GraphAudit struct {
	ReachableNodes []string
	TotalNodes     int
}

// AuditGraphTopology validates a directed graph specification monadically.
func AuditGraphTopology(startNode string, nodes map[string]GraphNodeSpec) result.Result[GraphAudit] {
	if _, exists := nodes[startNode]; !exists {
		return result.Err[GraphAudit](fmt.Errorf("start node %q not found in graph registry", startNode))
	}

	// Step 1: Audit for invalid dual Fork + Ordinary transition configurations using stream.OfMap
	if conflict := stream.OfMap(nodes).First(func(p stream.Pair[string, GraphNodeSpec]) bool {
		return p.Second.IsFork && len(p.Second.Transitions) > 0
	}); conflict.IsSome() {
		return result.Err[GraphAudit](fmt.Errorf("invalid graph config: node %q cannot be both a Fork and an ordinary transition source", conflict.MustGet().First))
	}

	// Step 2: Compute reachability via Breadth-First Traversal (BFS)
	visited := map[string]bool{startNode: true}
	queue := []string{startNode}

	for len(queue) > 0 {
		curr := queue[0]
		queue = queue[1:]

		spec := nodes[curr]
		for _, target := range spec.Transitions {
			if !visited[target] {
				visited[target] = true
				if _, exists := nodes[target]; exists {
					queue = append(queue, target)
				}
			}
		}
	}

	// Step 3: Check unreachable nodes using stream.OfMap
	if unreachable := stream.OfMap(nodes).First(func(p stream.Pair[string, GraphNodeSpec]) bool {
		return !visited[p.First]
	}); unreachable.IsSome() {
		return result.Err[GraphAudit](fmt.Errorf("unreachable node detected: node %q cannot be reached from start %q", unreachable.MustGet().First, startNode))
	}

	reachableList := stream.OfMap(visited).
		Map(func(p stream.Pair[string, bool]) string { return p.First }).
		Collect()

	return result.OK(GraphAudit{
		ReachableNodes: reachableList,
		TotalNodes:     len(nodes),
	})
}
