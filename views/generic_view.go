package views

import (
	"github.com/RyanSusana/archstats/analysis"
	"golang.org/x/exp/slices"
	"math"
	"sort"
	"strings"
)

func GenericView(allColumns []string, group analysis.StatsGroup) *View {
	var toReturn []*Row
	for groupItem, stats := range group {
		if groupItem == "" {
			groupItem = "Unknown"
		}
		data := statsToRowData(groupItem, stats)
		ensureRowHasAllColumns(data, allColumns)
		addAbstractness(data, stats)
		toReturn = append(toReturn, &Row{
			Data: data,
		})
	}

	columnsToReturn := []*Column{StringColumn(Name)}
	if slices.Contains(allColumns, analysis.AbstractType) {
		columnsToReturn = append(columnsToReturn, FloatColumn(Abstractness))
	}
	for _, column := range allColumns {
		columnsToReturn = append(columnsToReturn, IntColumn(column))
	}
	return &View{
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

func addAbstractness(data map[string]interface{}, theStats *analysis.Stats) {
	stats := *theStats
	if _, hasAbstractTypes := data[analysis.AbstractType]; hasAbstractTypes {
		abstractTypes := toInt(stats[analysis.AbstractType])
		types := toInt(stats[analysis.Type])
		abstractness := math.Max(0, math.Min(1, float64(abstractTypes)/float64(types)))
		data[Abstractness] = nanToZero(abstractness)
	}
}

func statsToRowData(name string, statsRef *analysis.Stats) map[string]interface{} {
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
