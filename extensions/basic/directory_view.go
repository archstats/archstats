package basic

import (
	"github.com/archstats/archstats/core"
)

func directoryView(results *core.Results) *core.View {
	return genericView(getDistinctColumnsFromResults(results), results.StatsByDirectory)
}
