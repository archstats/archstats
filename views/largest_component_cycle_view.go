package views

import (
	"github.com/RyanSusana/archstats/snippets"
	"gonum.org/v1/gonum/graph/topo"
	"sort"
)

func LargestComponentCycleView(results *snippets.Results) *View {
	theGraph := createGraph(results)

	cycles := topo.DirectedCyclesIn(theGraph)
	sort.Slice(cycles, func(i, j int) bool {
		return len(cycles[i]) > len(cycles[j])
	})

	theCycle := cycles[0]
	theCycle = theCycle[:len(theCycle)-1]

	var rows []*Row
	for componentIndex, component := range theCycle {
		successor := theCycle[wrapIndex(componentIndex+1, len(theCycle))]
		predecessor := theCycle[wrapIndex(componentIndex-1, len(theCycle))]
		rows = append(rows, &Row{
			Data: map[string]interface{}{
				"component":   theGraph.Node(component.ID()).(*componentNode).name,
				"successor":   theGraph.Node(successor.ID()).(*componentNode).name,
				"predecessor": theGraph.Node(predecessor.ID()).(*componentNode).name,
			},
		})
	}

	return &View{
		Columns: []*Column{
			StringColumn("component"),
			StringColumn("successor"),
			StringColumn("predecessor"),
		},
		Rows: rows,
	}
}
