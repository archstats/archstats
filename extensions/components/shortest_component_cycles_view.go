package components

import (
	"sort"

	"github.com/archstats/archstats/core"
)

func ShortestCyclesView(results *core.Results) *core.View {
	theGraph := results.ComponentGraph

	cycles := theGraph.ShortestCycles()

	// Numbered in the order of their text, not of the map holding them, so a
	// cycle keeps its number between two scans of the same commit.
	keys := make([]string, 0, len(cycles))
	for key := range cycles {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	var rows []*core.Row
	cycleNr := 0
	for _, cycleKey := range keys {
		cycle := cycles[cycleKey]
		cycleNr++
		for _, cmpnt := range cycle.Components() {
			rows = append(rows, &core.Row{
				Data: map[string]interface{}{
					"component":  cmpnt,
					"cycle_nr":   cycleNr,
					"cycle_size": len(cycle) - 1,
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
