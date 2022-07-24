package views

import "archstats/snippets"

func DirectoryView(results *snippets.Results) *View {
	return GenericView(getDistinctColumnsFromResults(results), results.SnippetsByDirectory)
}
