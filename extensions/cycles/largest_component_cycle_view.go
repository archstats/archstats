package cycles

import (
	"github.com/RyanSusana/archstats/analysis"
	"gonum.org/v1/gonum/graph/topo"
	"sort"
)

func largestComponentCycleView(results *analysis.Results) *analysis.View {
	theGraph := results.ComponentGraph

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
				"component":         theGraph.IdToComponent(component.ID()),
				"cycle_successor":   theGraph.IdToComponent(successor.ID()),
				"cycle_predecessor": theGraph.IdToComponent(predecessor.ID()),
			},
		})
	}

	return &analysis.View{
		Columns: []*analysis.Column{
			analysis.StringColumn("component"),
			analysis.StringColumn("cycle_successor"),
			analysis.StringColumn("cycle_predecessor"),
		},
		Rows: rows,
	}
}
