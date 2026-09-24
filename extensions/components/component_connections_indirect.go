package components

import (
	"github.com/archstats/archstats/core"
	"github.com/samber/lo"
	"gonum.org/v1/gonum/graph"
	"gonum.org/v1/gonum/graph/path"
	"sort"
	"strings"
)

func ConnectionsIndirectView(results *core.Results) *core.View {
	theGraph := results.ComponentGraph

	allShortest := path.DijkstraAllPaths(theGraph)

	components := lo.Keys(results.SnippetsByComponent)
	sort.Strings(components)

	var rows []*core.Row
	for _, from := range components {
		for _, to := range components {
			if from == to {
				continue
			}
			shortestPaths, _ := allShortest.AllBetween(theGraph.ComponentToId(from), theGraph.ComponentToId(to))

			// One row per pair. Every tied shortest path used to get its own
			// row, so counting rows counted some pairs several times; the
			// path kept is the alphabetically first, so it is stable.
			best := ""
			bestLen := 0
			for _, shortest := range shortestPaths {
				if len(shortest) < 2 {
					continue
				}
				joined := strings.Join(lo.Map(shortest, func(node graph.Node, _ int) string {
					return theGraph.IdToComponent(node.ID())
				}), " -> ")
				if best == "" || joined < best {
					best, bestLen = joined, len(shortest)
				}
			}
			if best == "" {
				continue
			}
			rows = append(rows, &core.Row{
				Data: map[string]interface{}{
					"from": from,
					"to":   to,
					// Hops, not nodes: a direct dependency is 1 away.
					"shortest_path_length": bestLen - 1,
					"shortest_path":        best,
				},
			})
		}
	}

	return &core.View{
		Columns: []*core.Column{core.StringColumn("from"), core.StringColumn("to"), core.IntColumn("shortest_path_length"), core.StringColumn("shortest_path")},
		Rows:    rows,
	}
}
