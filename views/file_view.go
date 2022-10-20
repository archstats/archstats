package views

import (
	"github.com/RyanSusana/archstats/analysis"
)

func FileView(results *analysis.Results) *View {
	return GenericView(getDistinctColumnsFromResults(results), results.StatsByFile)
}
