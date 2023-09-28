package basic

import (
	"github.com/RyanSusana/archstats/core"
	"github.com/samber/lo"
	"gonum.org/v1/gonum/graph"
	"gonum.org/v1/gonum/graph/path"
	"strings"
)

func componentConnectionsFurthestView(results *core.Results) *core.View {
	theGraph := results.ComponentGraph

	allPaths := path.DijkstraAllPaths(theGraph)

	var rows []*core.Row
	for from := range results.SnippetsByComponent {
		key := from
		var furthest []graph.Node
		for to := range results.SnippetsByComponent {
			if from == to {
				continue
			}
			shortest, _, _ := allPaths.Between(theGraph.ComponentToId(from), theGraph.ComponentToId(to))
			if len(shortest) > len(furthest) {
				furthest = shortest
			}
		}

		if len(furthest) <= 0 {
			continue
		}
		rows = append(rows, &core.Row{
			Data: map[string]interface{}{
				"component":                   key,
				"furthest_component":          theGraph.IdToComponent(furthest[len(furthest)-1].ID()),
				"furthest_component_distance": len(furthest),
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
