package util

import (
	"github.com/archstats/archstats/core"
	"github.com/archstats/archstats/core/file"
	"github.com/archstats/archstats/core/stats"
	"github.com/samber/lo"
	"sort"
	"strings"
	"time"
)

const (
	Name = "name"
)

func GenericView(allColumns []string, group stats.StatsGroup) *core.View {
	// First, detect types for allColumns
	columnTypes := make(map[string]core.ColumnType)
	for _, column := range allColumns {
		cType := core.Integer
		for _, statsRef := range group {
			if val, exists := (*statsRef)[column]; exists && val != nil {
				switch val.(type) {
				case string:
					cType = core.String
				case float64:
					cType = core.Float
				case time.Time:
					cType = core.Date
				case file.Position:
					cType = core.PositionInFile
				}
				break
			}
		}
		columnTypes[column] = cType
	}

	var toReturn []*core.Row
	for groupItem, stats := range group {
		if groupItem == "" {
			groupItem = "Unknown"
		}
		data := statsToRowData(groupItem, stats)
		
		// Ensure row has all columns, setting appropriate default values
		for _, column := range allColumns {
			if _, hasColumn := data[column]; !hasColumn {
				switch columnTypes[column] {
				case core.String:
					data[column] = ""
				default:
					data[column] = 0
				}
			}
		}
		
		toReturn = append(toReturn, &core.Row{
			Data: data,
		})
	}

	columnsToReturn := []*core.Column{core.StringColumn(Name)}
	for _, column := range allColumns {
		switch columnTypes[column] {
		case core.String:
			columnsToReturn = append(columnsToReturn, core.StringColumn(column))
		case core.Float:
			columnsToReturn = append(columnsToReturn, core.FloatColumn(column))
		case core.Date:
			columnsToReturn = append(columnsToReturn, core.DateColumn(column))
		case core.PositionInFile:
			columnsToReturn = append(columnsToReturn, core.PositionInFileColumn(column))
		default:
			columnsToReturn = append(columnsToReturn, core.IntColumn(column))
		}
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

func statsToRowData(name string, statsRef *stats.Stats) map[string]interface{} {
	stats := *statsRef
	toReturn := make(map[string]interface{}, len(stats)+1)
	toReturn["name"] = name
	for k, v := range stats {
		toReturn[k] = v
	}
	return toReturn
}

func GetDistinctColumnsFrom(results stats.StatsGroup) []string {

	allStats := lo.MapToSlice(results, func(_ string, stats *stats.Stats) *stats.Stats {
		return stats
	})

	init := make(stats.Stats)
	singleStats := lo.Reduce(allStats, func(acc *stats.Stats, stats *stats.Stats, _ int) *stats.Stats {
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
