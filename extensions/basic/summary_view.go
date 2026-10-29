package basic

import (
	"github.com/archstats/archstats/core"
)

const (
	NameColumn  = "name"
	ValueColumn = "value"
)

func summaryView(results *core.Results) *core.View {
	var toReturn []*core.Row

	totalStats := results.Calculate(results.StatRecords)
	for stat, value := range *totalStats {
		// A stat that is text, like java_class, names one file's class; its
		// "total" was whichever file's name the merge kept.
		if t, ok := core.ColumnTypeOf(value); !ok || (t != core.Integer && t != core.Float) {
			continue
		}
		toReturn = append(toReturn, &core.Row{
			Data: map[string]interface{}{
				NameColumn:  stat,
				ValueColumn: value,
			},
		})
	}

	// The engine version is text in a column of numbers; it is in _snapshot
	// as engine_version.
	extraRows := []core.RowData{
		{
			NameColumn:  "component_count",
			ValueColumn: len(results.ComponentToFiles),
		},
		{
			NameColumn:  "connection_count",
			ValueColumn: len(results.Connections),
		},
		{
			NameColumn:  "module_count",
			ValueColumn: results.Modules.Len(),
		},
		{
			NameColumn:  "directory_count",
			ValueColumn: len(results.DirectoryToFiles),
		},
	}

	toReturn = append(toReturn, unitCounts(results)...)

	for _, row := range extraRows {
		toReturn = append(toReturn, &core.Row{
			Data: row,
		})
	}
	return &core.View{
		Columns: []*core.Column{core.StringColumn(NameColumn), core.NumberColumn(ValueColumn)},
		Rows:    toReturn,
	}
}
