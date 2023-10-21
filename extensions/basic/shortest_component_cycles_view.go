package basic

import (
	"github.com/archstats/archstats/core"
)

func shortestComponentCyclesView(results *core.Results) *core.View {
	theGraph := results.ComponentGraph

	cycles := theGraph.ShortestCycles()

	var rows []*core.Row
	cycleNr := 0
	for cycleKey, cycle := range cycles {
		cycleNr++
		for _, cmpnt := range cycle {
			rows = append(rows, &core.Row{
				Data: map[string]interface{}{
					"component":  cmpnt,
					"cycle_nr":   cycleNr,
					"cycle_size": len(cycle),
					"cycle":      cycleKey,
				},
			})
		}
	}

	return &core.View{
		Columns: []*core.Column{
			core.IntColumn("cycle_nr"),
			core.StringColumn("component"),
			core.IntColumn("cycle_size"),
			core.StringColumn("cycle"),
		},
		Rows: rows,
	}
}
