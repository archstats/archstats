package views

import (
	"github.com/RyanSusana/archstats/analysis"
)

const (
	NameColumn  = "name"
	CountColumn = "count"
)

func SummaryView(results *analysis.Results) *View {
	var toReturn []*Row

	for stat, value := range *results.Stats {
		toReturn = append(toReturn, &Row{
			Data: map[string]interface{}{
				NameColumn:  stat,
				CountColumn: value,
			},
		})
	}

	extraRows := []RowData{
		{
			NameColumn:  "component_count",
			CountColumn: len(results.StatsByComponent),
		},
		{
			NameColumn:  "connection_count",
			CountColumn: len(results.Connections),
		},
		{
			NameColumn:  "directory_count",
			CountColumn: len(results.StatsByDirectory),
		},
	}

	for _, row := range extraRows {
		toReturn = append(toReturn, &Row{
			Data: row,
		})
	}
	return &View{
		Columns: []*Column{StringColumn(NameColumn), IntColumn(CountColumn)},
		Rows:    toReturn,
	}
}
