package cycles

import (
	"github.com/archstats/archstats/core"
	"gonum.org/v1/gonum/graph/topo"
	"sort"
)

func largestComponentCycleView(results *core.Results) *core.View {
	theGraph := results.ComponentGraph

	cycles := topo.DirectedCyclesIn(theGraph)
	sort.Slice(cycles, func(i, j int) bool {
		return len(cycles[i]) > len(cycles[j])
	})

	theCycle := cycles[0]
	theCycle = theCycle[:len(theCycle)-1]

	var rows []*core.Row
	for componentIndex, component := range theCycle {
		successor := theCycle[wrapIndex(componentIndex+1, len(theCycle))]
		predecessor := theCycle[wrapIndex(componentIndex-1, len(theCycle))]
		rows = append(rows, &core.Row{
			Data: map[string]interface{}{
				"component":         theGraph.IdToComponent(component.ID()),
				"cycle_successor":   theGraph.IdToComponent(successor.ID()),
				"cycle_predecessor": theGraph.IdToComponent(predecessor.ID()),
			},
		})
	}

	return &core.View{
		Columns: []*core.Column{
			core.StringColumn("component"),
			core.StringColumn("cycle_successor"),
			core.StringColumn("cycle_predecessor"),
		},
		Rows: rows,
	}
}
