package graph

import (
	"github.com/RyanSusana/archstats/analysis"
	"gonum.org/v1/gonum/graph/topo"
	"sort"
)

func LargestComponentCycleView(results *analysis.Results) *analysis.View {
	theGraph := createGraph(results)

	cycles := topo.DirectedCyclesIn(theGraph)
	sort.Slice(cycles, func(i, j int) bool {
		return len(cycles[i]) > len(cycles[j])
	})

	theCycle := cycles[0]
	theCycle = theCycle[:len(theCycle)-1]

	var rows []*analysis.Row
	for componentIndex, component := range theCycle {
		successor := theCycle[wrapIndex(componentIndex+1, len(theCycle))]
		predecessor := theCycle[wrapIndex(componentIndex-1, len(theCycle))]
		rows = append(rows, &analysis.Row{
			Data: map[string]interface{}{
				"component":   theGraph.Node(component.ID()).(*componentNode).name,
				"successor":   theGraph.Node(successor.ID()).(*componentNode).name,
				"predecessor": theGraph.Node(predecessor.ID()).(*componentNode).name,
			},
		})
	}

	return &analysis.View{
		Columns: []*analysis.Column{
			analysis.StringColumn("component"),
			analysis.StringColumn("successor"),
			analysis.StringColumn("predecessor"),
		},
		Rows: rows,
	}
}
