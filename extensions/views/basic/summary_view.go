package basic

import (
	"github.com/RyanSusana/archstats/analysis"
)

const (
	NameColumn  = "name"
	ValueColumn = "value"
)

func summaryView(results *analysis.Results) *analysis.View {
	var toReturn []*analysis.Row

	for stat, value := range *results.Stats {
		toReturn = append(toReturn, &analysis.Row{
			Data: map[string]interface{}{
				NameColumn:  stat,
				ValueColumn: value,
			},
		})
	}

	extraRows := []analysis.RowData{
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
		toReturn = append(toReturn, &analysis.Row{
			Data: row,
		})
	}
	return &analysis.View{
		Columns: []*analysis.Column{analysis.StringColumn(NameColumn), analysis.IntColumn(ValueColumn)},
		Rows:    toReturn,
	}
}
