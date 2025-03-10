package components

import (
	"github.com/archstats/archstats/core"
	"github.com/samber/lo"
	"gonum.org/v1/gonum/graph"
	"gonum.org/v1/gonum/graph/path"
	"strings"
)

func ConnectionsIndirectView(results *core.Results) *core.View {
	theGraph := results.ComponentGraph

	allShortest := path.DijkstraAllPaths(theGraph)

	var rows []*core.Row
	for from := range results.SnippetsByComponent {
		for to := range results.SnippetsByComponent {
			if from == to {
				continue
			}

			shortestPaths, _ := allShortest.AllBetween(theGraph.ComponentToId(from), theGraph.ComponentToId(to))

			for _, shortest := range shortestPaths {
				if len(shortest) >= 2 {
					rows = append(rows, &core.Row{
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
	}

	return &core.View{
		Columns: []*core.Column{core.StringColumn("from"), core.StringColumn("to"), core.IntColumn("shortest_path_length"), core.StringColumn("shortest_path")},
		Rows:    rows,
	}
}
