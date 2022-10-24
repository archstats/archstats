package graph

import (
	"github.com/RyanSusana/archstats/analysis"
	"gonum.org/v1/gonum/graph/topo"
)

func ComponentCyclesView(results *analysis.Results) *analysis.View {
	theGraph := createGraph(results)

	cycles := topo.DirectedCyclesIn(theGraph)

	// Remove last cycle, which is the full graph
	for i, cycle := range cycles {
		cycles[i] = cycle[:len(cycle)-1]
	}

	var rows []*analysis.Row
	for cycleNr, theCycle := range cycles {
		for componentIndex, component := range theCycle {
			successor := theCycle[wrapIndex(componentIndex+1, len(theCycle))]
			predecessor := theCycle[wrapIndex(componentIndex-1, len(theCycle))]
			rows = append(rows, &analysis.Row{
				Data: map[string]interface{}{
					"cycle_nr":    cycleNr,
					"cycle_size":  len(theCycle),
					"component":   theGraph.Node(component.ID()).(*componentNode).name,
					"successor":   theGraph.Node(successor.ID()).(*componentNode).name,
					"predecessor": theGraph.Node(predecessor.ID()).(*componentNode).name,
				},
			})
		}

	}
	return &analysis.View{
		Columns: []*analysis.Column{
			analysis.IntColumn("cycle_nr"),
			analysis.IntColumn("cycle_size"),
			analysis.StringColumn("component"),
			analysis.StringColumn("successor"),
			analysis.StringColumn("predecessor"),
		},
		Rows: rows,
	}
}

func wrapIndex(i, max int) int {
	i = i % max
	if i < 0 {
		i += max
	}
	return i
}
