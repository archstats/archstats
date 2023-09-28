package cycles

import (
	"github.com/RyanSusana/archstats/analysis"
	"gonum.org/v1/gonum/graph/topo"
)

func allComponentCyclesView(results *analysis.Results) *analysis.View {
	theGraph := results.ComponentGraph

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
					"cycle_nr":          cycleNr + 1,
					"cycle_size":        len(theCycle),
					"component":         theGraph.IdToComponent(component.ID()),
					"cycle_successor":   theGraph.IdToComponent(successor.ID()),
					"cycle_predecessor": theGraph.IdToComponent(predecessor.ID()),
				},
			})
		}

	}
	return &analysis.View{
		Columns: []*analysis.Column{
			analysis.IntColumn("cycle_nr"),
			analysis.IntColumn("cycle_size"),
			analysis.StringColumn("component"),
			analysis.StringColumn("cycle_successor"),
			analysis.StringColumn("cycle_predecessor"),
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
