package views

import (
	"github.com/RyanSusana/archstats/snippets"
)

func SummaryView(results *snippets.Results) *View {
	return GenericView(getDistinctColumnsFromResults(results), results.SnippetsByType)
}
