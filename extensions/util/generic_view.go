package util

import (
	"github.com/archstats/archstats/core"
	"github.com/archstats/archstats/core/file"
	"github.com/samber/lo"
	"sort"
	"strings"
)

const (
	Name = "name"
)

func GenericView(allColumns []string, group file.StatsGroup) *core.View {
	var toReturn []*core.Row
	for groupItem, stats := range group {
		if groupItem == "" {
			groupItem = "Unknown"
		}
		data := statsToRowData(groupItem, stats)
		ensureRowHasAllColumns(data, allColumns)
		toReturn = append(toReturn, &core.Row{
			Data: data,
		})
	}

	columnsToReturn := []*core.Column{core.StringColumn(Name)}
	for _, column := range allColumns {
		columnsToReturn = append(columnsToReturn, core.IntColumn(column))
	}
	return &core.View{
		Columns: columnsToReturn,
		Rows:    toReturn,
	}
}

func ensureRowHasAllColumns(data map[string]interface{}, columns []string) {
	for _, column := range columns {
		if _, hasColumn := data[column]; !hasColumn {
			data[column] = 0
		}
	}
}

func statsToRowData(name string, statsRef *file.Stats) map[string]interface{} {
	stats := *statsRef
	toReturn := make(map[string]interface{}, len(stats)+1)
	toReturn["name"] = name
	for k, v := range stats {
		toReturn[k] = v
	}
	return toReturn
}

func GetDistinctColumnsFrom(results file.StatsGroup) []string {

	allStats := lo.MapToSlice(results, func(_ string, stats *file.Stats) *file.Stats {
		return stats
	})

	init := make(file.Stats)
	singleStats := lo.Reduce(allStats, func(acc *file.Stats, stats *file.Stats, _ int) *file.Stats {
		for k, v := range *stats {
			(*acc)[k] = v
		}
		return acc
	}, &init)
	var toReturn []string
	for theType, _ := range *singleStats {
		if !strings.HasPrefix(theType, "_") {
			toReturn = append(toReturn, theType)
		}
	}
	sort.Strings(toReturn)
	return toReturn
}

func ToInt(value interface{}) int {
	if value == nil {
		return 0
	}
	return value.(int)
}
