package views

import "archstats/snippets"

func DirectoryRecursiveView(results *snippets.Results) *View {
	var toReturn []*Row
	snippetsByDirectory := results.SnippetsByDirectory
	allColumns := getDistinctColumnsFromResults(results)
	statsByDirectory := statsByGroup(allColumns, snippetsByDirectory)
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
	columnsToReturn := []string{"name"}
	for _, column := range allColumns {
		columnsToReturn = append(columnsToReturn, column)
	}
	return &View{
		OrderedColumns: columnsToReturn,
		Rows:           toReturn,
	}
}
