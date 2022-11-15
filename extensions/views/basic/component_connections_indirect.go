package basic

import (
	"github.com/RyanSusana/archstats/analysis"
	"github.com/samber/lo"
	"gonum.org/v1/gonum/graph"
	"gonum.org/v1/gonum/graph/path"
	"strings"
)

func componentConnectionsIndirectView(results *analysis.Results) *analysis.View {
	theGraph := results.ComponentGraph

	allShortest := path.DijkstraAllPaths(theGraph)

	var rows []*analysis.Row
	for from := range results.SnippetsByComponent {
		for to := range results.SnippetsByComponent {
			if from == to {
				continue
			}

			shortest, _, _ := allShortest.Between(theGraph.ComponentToId(from), theGraph.ComponentToId(to))

			if len(shortest) >= 2 {
				rows = append(rows, &analysis.Row{
					Data: map[string]interface{}{
						"from":                 from,
						"to":                   to,
						"shortest_path_length": len(shortest),
						"shortest_path": strings.Join(lo.Map(
							shortest,
							func(node graph.Node, _ int) string {
								return theGraph.IdToComponent(node.ID())
							},
						), " -> "),
					},
				})
			}
		}
	}

	return &analysis.View{
		Columns: []*analysis.Column{analysis.StringColumn("from"), analysis.StringColumn("to"), analysis.IntColumn("shortest_path_length"), analysis.StringColumn("shortest_path")},
		Rows:    rows,
	}
}
