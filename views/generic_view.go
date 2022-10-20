package views

import (
	"github.com/RyanSusana/archstats/snippets"
	"golang.org/x/exp/slices"
	"math"
	"sort"
)

func GenericView(allColumns []string, group snippets.StatsGroup) *View {
	var toReturn []*Row
	for groupItem, stats := range group {
		if groupItem == "" {
			groupItem = "Unknown"
		}
		data := statsToRowData(groupItem, stats)
		//addFileCount(data, groupedSnippets)
		addAbstractness(data, stats)
		toReturn = append(toReturn, &Row{
			Data: data,
		})
	}

	columnsToReturn := []*Column{StringColumn(Name), IntColumn(FileCount)}
	if slices.Contains(allColumns, snippets.AbstractType) {
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

//TODO
//func addFileCount(data map[string]interface{}, groupedSnippets []*snippets.Snippet) {
//	data[FileCount] = getDistinctCount(groupedSnippets, fileCount)
//}
func addAbstractness(data map[string]interface{}, theStats *snippets.Stats) {
	stats := *theStats
	if _, hasAbstractTypes := data[snippets.AbstractType]; hasAbstractTypes {
		abstractTypes, types := stats[snippets.AbstractType], stats[snippets.Type]
		abstractness := math.Max(0, math.Min(1, float64(abstractTypes)/float64(types)))
		data[Abstractness] = nanToZero(abstractness)
	}
}

func statsToRowData(name string, statsRef *snippets.Stats) map[string]interface{} {
	stats := *statsRef
	toReturn := make(map[string]interface{}, len(stats)+1)
	toReturn["name"] = name
	for k, v := range stats {
		toReturn[k] = v
	}
	return toReturn
}

func getDistinctColumnsFromResults(results *snippets.Results) []string {
	var toReturn []string
	for theType, _ := range results.SnippetsByType {
		toReturn = append(toReturn, theType)
	}
	sort.Strings(toReturn)
	return toReturn
}

func fileCount(snippet *snippets.Snippet) interface{} {
	return snippet.File
}
func getDistinctCount(results []*snippets.Snippet, distinctFunc func(snippet *snippets.Snippet) interface{}) int {
	files := make(map[interface{}]bool)
	for _, snippet := range results {
		files[distinctFunc(snippet)] = true
	}
	return len(files)
}
