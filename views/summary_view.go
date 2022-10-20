package views

import (
	"github.com/RyanSusana/archstats/snippets"
	"github.com/samber/lo"
)

const (
	SnippetType    = "snippet_type"
	TotalCount     = "total_count"
	FileCount      = "file_count"
	DirectoryCount = "directory_count"
	ComponentCount = "component_count"
)

func SummaryView(results *snippets.Results) *View {
	var toReturn []*Row

	for snippetType, allSnippets := range results.SnippetsByType {
		files := lo.GroupBy(allSnippets, snippets.ByFile)
		directories := lo.GroupBy(allSnippets, snippets.ByDirectory)
		components := lo.GroupBy(allSnippets, snippets.ByComponent)

		toReturn = append(toReturn, &Row{
			Data: map[string]interface{}{
				SnippetType:    snippetType,
				TotalCount:     len(allSnippets),
				FileCount:      len(files),
				DirectoryCount: len(directories),
				ComponentCount: len(components),
			},
		})

	}
	return &View{
		Columns: []*Column{StringColumn(SnippetType), IntColumn(TotalCount), IntColumn(FileCount), IntColumn(DirectoryCount), IntColumn(ComponentCount)},
		Rows:    toReturn,
	}
}
