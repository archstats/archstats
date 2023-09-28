package basic

import (
	"github.com/RyanSusana/archstats/core"
	"gonum.org/v1/gonum/graph/topo"
)

func stronglyConnectedComponentGroupsView(results *core.Results) *core.View {
	theGraph := results.ComponentGraph

	groups := topo.TarjanSCC(theGraph)
	var rows []*core.Row
	for groupNr, theGroup := range groups {
		for _, component := range theGroup {
			rows = append(rows, &core.Row{
				Data: map[string]interface{}{
					"group_nr":   groupNr,
					"group_size": len(theGroup),
					"component":  theGraph.IdToComponent(component.ID()),
				},
			})
		}
	}
	return &core.View{
		Columns: []*core.Column{
			core.IntColumn("group_nr"),
			core.IntColumn("group_size"),
			core.StringColumn("component"),
		},
		Rows: rows,
	}
}
