package views

import "github.com/RyanSusana/archstats/snippets"

func DirectoryRecursiveView(results *snippets.Results) *View {
	var toReturn []*Row
	snippetsByDirectory := results.SnippetsByDirectory
	allColumns := getDistinctColumnsFromResults(results)
	statsByDirectory := getStatsPerGroup(allColumns, snippetsByDirectory)
	allDirs := make([]string, 0, len(snippetsByDirectory))

	for dir, _ := range snippetsByDirectory {
		allDirs = append(allDirs, dir)
	}

	dirLookup := createDirectoryTree(results.RootDirectory, allDirs)

	for dir, node := range dirLookup {
		subtree := toPaths(node.subtree())
		var stats Stats
		for _, subDir := range subtree {
			stats = stats.Merge(statsByDirectory[subDir])
		}
		toReturn = append(toReturn, &Row{
			Data: statsToRowData(dir, stats),
		})
	}
	columnsToReturn := []*Column{StringColumn("name"), IntColumn(FileCount)}
	for _, column := range allColumns {
		columnsToReturn = append(columnsToReturn, IntColumn(column))
	}
	return &View{
		Columns: columnsToReturn,
		Rows:    toReturn,
	}
}
