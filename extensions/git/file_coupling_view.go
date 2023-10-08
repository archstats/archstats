package git

import (
	"github.com/archstats/archstats/core"
	"github.com/samber/lo"
)

func (e *extension) fileCouplingViewFactory(results *core.Results) *core.View {
	files := lo.Keys(results.FileToComponent)

	totals := getSharedCommitPairsFor(files, e.commitParts, true)

	partsByDayBucket := e.commitPartsByDayBucket
	dayBucketSharedCommitCounts := map[int]map[string]int{}
	for days, commits := range partsByDayBucket {
		dayBucketSharedCommitCounts[days] = getSharedCommitPairsFor(files, commits, true)
	}

	rows := sharedCommitsToRows(files, totals, dayBucketSharedCommitCounts)

	return &core.View{
		Columns: sharedCommitColumns(e.DayBuckets),
		Rows:    rows,
	}
}
