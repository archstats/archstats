package views

import (
	"github.com/RyanSusana/archstats/snippets"
	"gonum.org/v1/gonum/graph/topo"
)

func ComponentCyclesView(results *snippets.Results) *View {
	theGraph := createGraph(results)

	cycles := topo.DirectedCyclesIn(theGraph)

	// Remove last cycle, which is the full graph
	for i, cycle := range cycles {
		cycles[i] = cycle[:len(cycle)-1]
	}

	var rows []*Row
	for cycleNr, theCycle := range cycles {
		for componentIndex, component := range theCycle {
			successor := theCycle[wrapIndex(componentIndex+1, len(theCycle))]
			predecessor := theCycle[wrapIndex(componentIndex-1, len(theCycle))]
			rows = append(rows, &Row{
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
	return &View{
		Columns: []*Column{
			IntColumn("cycle_nr"),
			IntColumn("cycle_size"),
			StringColumn("component"),
			StringColumn("successor"),
			StringColumn("predecessor"),
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
