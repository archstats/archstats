package basic

import (
	"github.com/RyanSusana/archstats/analysis"
	"github.com/RyanSusana/archstats/analysis/file"
	"golang.org/x/exp/slices"
	"math"
	"sort"
	"strings"
)

func genericView(allColumns []string, group file.StatsGroup) *analysis.View {
	var toReturn []*analysis.Row
	for groupItem, stats := range group {
		if groupItem == "" {
			groupItem = "Unknown"
		}
		data := statsToRowData(groupItem, stats)
		ensureRowHasAllColumns(data, allColumns)
		addAbstractness(data, stats)
		toReturn = append(toReturn, &analysis.Row{
			Data: data,
		})
	}

	columnsToReturn := []*analysis.Column{analysis.StringColumn(Name)}
	if slices.Contains(allColumns, file.AbstractType) {
		columnsToReturn = append(columnsToReturn, analysis.FloatColumn(Abstractness))
	}
	for _, column := range allColumns {
		columnsToReturn = append(columnsToReturn, analysis.IntColumn(column))
	}
	return &analysis.View{
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

func addAbstractness(data map[string]interface{}, theStats *file.Stats) {
	stats := *theStats
	if _, hasAbstractTypes := data[file.AbstractType]; hasAbstractTypes {
		abstractTypes := toInt(stats[file.AbstractType])
		types := toInt(stats[file.Type])
		abstractness := math.Max(0, math.Min(1, float64(abstractTypes)/float64(types)))
		data[Abstractness] = nanToZero(abstractness)
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

func getDistinctColumnsFromResults(results *analysis.Results) []string {
	var toReturn []string
	for theType, _ := range *results.Stats {
		if !strings.HasPrefix(theType, "_") {
			toReturn = append(toReturn, theType)
		}
	}
	sort.Strings(toReturn)
	return toReturn
}

func toInt(value interface{}) int {
	if value == nil {
		return 0
	}
	return value.(int)
}
