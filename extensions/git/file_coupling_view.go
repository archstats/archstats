package git

import (
	"github.com/archstats/archstats/core"
	"github.com/archstats/archstats/extensions/git/commits"
	"github.com/samber/lo"
)

func (e *extension) fileCouplingViewFactory(results *core.Results) *core.View {
	files := lo.Keys(results.FileToComponent)

	sharedCommits := commits.PairsToCommitsInCommon(files, e.couplingCommits.FileToCommitHashes())
	dayBucketSharedCommitCounts := map[int]map[string]commits.CommitHashes{}

	for days, splittedCommits := range e.couplingCommits.DayBuckets() {
		dayBucketSharedCommitCounts[days] = commits.PairsToCommitsInCommon(files, splittedCommits.FileToCommitHashes())
	}

	mappedDayBuckets := lo.MapValues(e.couplingCommits.DayBuckets(), func(splitted *commits.Splitted, _ int) map[string]commits.CommitHashes {
		return splitted.FileToCommitHashes()
	})
	rows := sharedCommitsToRows(files, sharedCommits, dayBucketSharedCommitCounts, e.couplingCommits.FileToCommitHashes(), mappedDayBuckets)

	return &core.View{
		Columns: sharedCommitColumns(e.DayBuckets),
		Rows:    rows,
	}
}
