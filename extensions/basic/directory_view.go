package basic

import (
	"github.com/RyanSusana/archstats/analysis"
)

func directoryView(results *analysis.Results) *analysis.View {
	return genericView(getDistinctColumnsFromResults(results), results.StatsByDirectory)
}
