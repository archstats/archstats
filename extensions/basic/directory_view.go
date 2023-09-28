package basic

import (
	"github.com/RyanSusana/archstats/core"
)

func directoryView(results *core.Results) *core.View {
	return genericView(getDistinctColumnsFromResults(results), results.StatsByDirectory)
}
