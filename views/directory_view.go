package views

import (
	"github.com/RyanSusana/archstats/analysis"
)

func DirectoryView(results *analysis.Results) *View {
	return GenericView(getDistinctColumnsFromResults(results), results.StatsByDirectory)
}
