package views

import "github.com/RyanSusana/archstats/snippets"

func DirectoryView(results *snippets.Results) *View {
	return GenericView(getDistinctColumnsFromResults(results), results.StatsByDirectory)
}
