package views

import "archstats/snippets"

func FileView(results *snippets.Results) *View {
	return GenericView(getDistinctColumnsFromResults(results), results.SnippetsByFile)
}
