package views

import (
	"github.com/RyanSusana/archstats/analysis"
	"gonum.org/v1/gonum/graph/topo"
)

func StronglyConnectedComponentGroupsView(results *analysis.Results) *View {
	theGraph := createGraph(results)

	groups := topo.TarjanSCC(theGraph)
	var rows []*Row
	for groupNr, theGroup := range groups {
		for _, component := range theGroup {
			rows = append(rows, &Row{
				Data: map[string]interface{}{
					"group_nr":   groupNr,
					"group_size": len(theGroup),
					"component":  theGraph.Node(component.ID()).(*componentNode).name,
				},
			})
		}
	}
	return &View{
		Columns: []*Column{
			IntColumn("group_nr"),
			IntColumn("group_size"),
			StringColumn("component"),
		},
		Rows: rows,
	}
}
