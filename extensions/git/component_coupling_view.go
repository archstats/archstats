package git

import (
	"github.com/archstats/archstats/core"
	"github.com/archstats/archstats/extensions/git/commits"
	"github.com/samber/lo"
)

func (e *extension) componentCouplingViewFactory(results *core.Results) *core.View {
	components := lo.Keys(results.ComponentToFiles)

	sharedCommits := commits.PairsToCommitsInCommon(components, e.splittedCommits.ComponentToCommitHashes())
	dayBucketSharedCommitCounts := map[int]map[string]commits.CommitHashes{}

	for days, split := range e.splittedCommits.DayBuckets() {
		dayBucketSharedCommitCounts[days] = commits.PairsToCommitsInCommon(components, split.ComponentToCommitHashes())
	}

	e.splittedCommits.ComponentToCommitHashes()

	mappedDayBuckets := lo.MapValues(e.splittedCommits.DayBuckets(), func(splitted *commits.Splitted, _ int) map[string]commits.CommitHashes {
		return splitted.ComponentToCommitHashes()
	})
	rows := sharedCommitsToRows(components, sharedCommits, dayBucketSharedCommitCounts, e.splittedCommits.ComponentToCommitHashes(), mappedDayBuckets)

	return &core.View{
		Columns: sharedCommitColumns(e.DayBuckets),
		Rows:    rows,
	}
}
