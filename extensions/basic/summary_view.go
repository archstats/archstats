package basic

import (
	"github.com/archstats/archstats/core"
	"github.com/archstats/archstats/version"
)

const (
	NameColumn  = "name"
	ValueColumn = "value"
)

func summaryView(results *core.Results) *core.View {
	var toReturn []*core.Row

	for stat, value := range *results.Stats {
		toReturn = append(toReturn, &core.Row{
			Data: map[string]interface{}{
				NameColumn:  stat,
				ValueColumn: value,
			},
		})
	}

	extraRows := []core.RowData{
		{
			NameColumn:  "version",
			ValueColumn: version.Version(),
		},
		{
			NameColumn:  "component_count",
			ValueColumn: len(results.StatsByComponent),
		},
		{
			NameColumn:  "connection_count",
			ValueColumn: len(results.Connections),
		},
		{
			NameColumn:  "directory_count",
			ValueColumn: len(results.StatsByDirectory),
		},
	}

	for _, row := range extraRows {
		toReturn = append(toReturn, &core.Row{
			Data: row,
		})
	}
	return &core.View{
		Columns: []*core.Column{core.StringColumn(NameColumn), core.IntColumn(ValueColumn)},
		Rows:    toReturn,
	}
}
