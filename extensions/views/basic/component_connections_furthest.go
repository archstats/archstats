package basic

import (
	"github.com/RyanSusana/archstats/analysis"
	"github.com/samber/lo"
	"gonum.org/v1/gonum/graph"
	"gonum.org/v1/gonum/graph/path"
	"strings"
)

func componentConnectionsFurthestView(results *analysis.Results) *analysis.View {
	theGraph := results.ComponentGraph

	allPaths := path.DijkstraAllPaths(theGraph)

	var rows []*analysis.Row
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

		rows = append(rows, &analysis.Row{
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

	return &analysis.View{
		Columns: []*analysis.Column{analysis.StringColumn("component"), analysis.StringColumn("furthest_component"), analysis.IntColumn("furthest_component_distance"), analysis.StringColumn("furthest_component_shortest_path")},
		Rows:    rows,
	}
}
