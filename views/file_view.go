package views

import "github.com/RyanSusana/archstats/snippets"

func FileView(results *snippets.Results) *View {
	return GenericView(getDistinctColumnsFromResults(results), results.SnippetsByFile)
}
