package cycles

import (
	"github.com/RyanSusana/archstats/core"
	"gonum.org/v1/gonum/graph/topo"
)

func allComponentCyclesView(results *core.Results) *core.View {
	theGraph := results.ComponentGraph

	cycles := topo.DirectedCyclesIn(theGraph)

	// Remove last cycle, which is the full graph
	for i, cycle := range cycles {
		cycles[i] = cycle[:len(cycle)-1]
	}

	var rows []*core.Row
	for cycleNr, theCycle := range cycles {
		for componentIndex, component := range theCycle {
			successor := theCycle[wrapIndex(componentIndex+1, len(theCycle))]
			predecessor := theCycle[wrapIndex(componentIndex-1, len(theCycle))]
			rows = append(rows, &core.Row{
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
	return &core.View{
		Columns: []*core.Column{
			core.IntColumn("cycle_nr"),
			core.IntColumn("cycle_size"),
			core.StringColumn("component"),
			core.StringColumn("cycle_successor"),
			core.StringColumn("cycle_predecessor"),
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
