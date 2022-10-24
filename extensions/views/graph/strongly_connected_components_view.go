package graph

import (
	"github.com/RyanSusana/archstats/analysis"
	"gonum.org/v1/gonum/graph/topo"
)

func StronglyConnectedComponentGroupsView(results *analysis.Results) *analysis.View {
	theGraph := createGraph(results)

	groups := topo.TarjanSCC(theGraph)
	var rows []*analysis.Row
	for groupNr, theGroup := range groups {
		for _, component := range theGroup {
			rows = append(rows, &analysis.Row{
				Data: map[string]interface{}{
					"group_nr":   groupNr,
					"group_size": len(theGroup),
					"component":  theGraph.Node(component.ID()).(*componentNode).name,
				},
			})
		}
	}
	return &analysis.View{
		Columns: []*analysis.Column{
			analysis.IntColumn("group_nr"),
			analysis.IntColumn("group_size"),
			analysis.StringColumn("component"),
		},
		Rows: rows,
	}
}
