package git

import (
	"github.com/archstats/archstats/core"
	"github.com/archstats/archstats/extensions/git/commits"
	"github.com/samber/lo"
)

func (e *extension) componentCouplingViewFactory(results *core.Results) *core.View {
	components := lo.Keys(results.ComponentToFiles)

	totals := commits.GetCommitsInCommonForComponentPairs(components, e.splittedCommits.SplitByCommitHash())
	dayBucketSharedCommitCounts := map[int]map[string]commits.CommitHashes{}

	for days, split := range e.splittedCommits.DayBuckets() {
		dayBucketSharedCommitCounts[days] = commits.GetCommitsInCommonForComponentPairs(components, split.SplitByCommitHash())
	}

	rows := sharedCommitsToRows(components, totals, dayBucketSharedCommitCounts)

	return &core.View{
		Columns: sharedCommitColumns(e.DayBuckets),
		Rows:    rows,
	}
}
