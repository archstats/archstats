package views

import (
	"github.com/RyanSusana/archstats/snippets"
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
		files := snippets.GroupSnippetsBy(allSnippets, snippets.ByFile)
		directories := snippets.GroupSnippetsBy(allSnippets, snippets.ByDirectory)
		components := snippets.GroupSnippetsBy(allSnippets, snippets.ByComponent)

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
		OrderedColumns: []string{SnippetType, TotalCount, FileCount, DirectoryCount, ComponentCount},
		Rows:           toReturn,
	}
}
