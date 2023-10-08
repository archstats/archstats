package git

import (
	"github.com/archstats/archstats/core"
	"github.com/samber/lo"
)

func (e *extension) componentCouplingViewFactory(results *core.Results) *core.View {
	components := lo.Keys(results.ComponentToFiles)

	totals := getSharedCommitPairsFor(components, e.commitParts, false)

	partsByDayBucket := e.commitPartsByDayBucket
	dayBucketSharedCommitCounts := map[int]map[string]int{}
	for days, commits := range partsByDayBucket {
		dayBucketSharedCommitCounts[days] = getSharedCommitPairsFor(components, commits, false)
	}

	rows := sharedCommitsToRows(components, totals, dayBucketSharedCommitCounts)

	return &core.View{
		Columns: sharedCommitColumns(e.DayBuckets),
		Rows:    rows,
	}
}
