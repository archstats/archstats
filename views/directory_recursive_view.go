package views

import (
	"github.com/RyanSusana/archstats/analysis"
)

func DirectoryRecursiveView(results *analysis.Results) *View {
	var toReturn []*Row
	snippetsByDirectory := results.SnippetsByDirectory
	allColumns := getDistinctColumnsFromResults(results)
	statsByDirectory := results.StatsByDirectory
	allDirs := make([]string, 0, len(snippetsByDirectory))

	for dir, _ := range snippetsByDirectory {
		allDirs = append(allDirs, dir)
	}

	dirLookup := createDirectoryTree(results.RootDirectory, allDirs)

	for dir, node := range dirLookup {
		subtree := toPaths(node.subtree())
		newStats := make(analysis.Stats)
		var stats = &newStats
		for _, subDir := range subtree {
			stats = analysis.MergeMultipleStats([]*analysis.Stats{stats, statsByDirectory[subDir]})
		}
		toReturn = append(toReturn, &Row{
			Data: statsToRowData(dir, stats),
		})
	}
	columnsToReturn := []*Column{StringColumn("name")}
	for _, column := range allColumns {
		columnsToReturn = append(columnsToReturn, IntColumn(column))
	}
	return &View{
		Columns: columnsToReturn,
		Rows:    toReturn,
	}
}
