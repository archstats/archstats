package components

import (
	"math"

	"github.com/archstats/archstats/core"
	"github.com/archstats/archstats/core/component"
	"github.com/samber/lo"
	"gonum.org/v1/gonum/graph"
	"gonum.org/v1/gonum/graph/path"
	"sort"
	"strings"
)

func ConnectionsFurthestView(results *core.Results) *core.View {
	theGraph := results.ComponentGraph

	allPaths := path.DijkstraAllPaths(theGraph)

	// Sorted, so a tie between two equally far components resolves the same
	// way on every scan instead of by map iteration order.
	components := lo.Keys(results.SnippetsByComponent)
	sort.Strings(components)

	var rows []*core.Row
	for _, from := range components {
		var furthest []graph.Node
		for _, to := range components {
			if from == to {
				continue
			}
			shortest := canonicalPath(theGraph, allPaths, theGraph.ComponentToId(from), theGraph.ComponentToId(to))
			if len(shortest) > len(furthest) {
				furthest = shortest
			}
		}
		// A path of one node is no reach at all.
		if len(furthest) < 2 {
			continue
		}
		rows = append(rows, &core.Row{
			Data: map[string]interface{}{
				"component":          from,
				"furthest_component": theGraph.IdToComponent(furthest[len(furthest)-1].ID()),
				// Hops, not the nodes on the path: a direct dependency is 1 away.
				// Counting nodes put every distance in the product one too far.
				"furthest_component_distance": len(furthest) - 1,
				"furthest_component_shortest_path": strings.Join(lo.Map(
					furthest,
					func(node graph.Node, _ int) string {
						return theGraph.IdToComponent(node.ID())
					},
				), " -> "),
			},
		})
	}

	return &core.View{
		Columns: []*core.Column{core.StringColumn("component"), core.StringColumn("furthest_component"), core.IntColumn("furthest_component_distance"), core.StringColumn("furthest_component_shortest_path")},
		Rows:    rows,
	}
}

// canonicalPath is a shortest path from one component to another that takes
// the alphabetically first next step wherever several are equally short.
// gonum's Between picks among them at random, so the path shown changed
// between two scans of the same commit.
func canonicalPath(g *component.Graph, paths path.AllShortest, from, to int64) []graph.Node {
	left := paths.Weight(from, to)
	if math.IsInf(left, 0) {
		return nil
	}
	route := []graph.Node{g.Node(from)}
	for current := from; current != to; {
		var next graph.Node
		successors := g.From(current)
		for successors.Next() {
			n := successors.Node()
			if paths.Weight(n.ID(), to) != left-1 {
				continue
			}
			if next == nil || g.IdToComponent(n.ID()) < g.IdToComponent(next.ID()) {
				next = n
			}
		}
		if next == nil {
			return nil
		}
		route = append(route, next)
		current, left = next.ID(), left-1
	}
	return route
}
