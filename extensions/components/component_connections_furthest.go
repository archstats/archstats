package components

import (
	"github.com/archstats/archstats/core"
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
			shortest, _, _ := allPaths.Between(theGraph.ComponentToId(from), theGraph.ComponentToId(to))
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
