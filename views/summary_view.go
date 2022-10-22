package views

import (
	"github.com/RyanSusana/archstats/analysis"
)

const (
	NameColumn  = "name"
	ValueColumn = "value"
)

func SummaryView(results *analysis.Results) *View {
	var toReturn []*Row

	for stat, value := range *results.Stats {
		toReturn = append(toReturn, &Row{
			Data: map[string]interface{}{
				NameColumn:  stat,
				ValueColumn: value,
			},
		})
	}

	extraRows := []RowData{
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
		toReturn = append(toReturn, &Row{
			Data: row,
		})
	}
	return &View{
		Columns: []*Column{StringColumn(NameColumn), IntColumn(ValueColumn)},
		Rows:    toReturn,
	}
}
