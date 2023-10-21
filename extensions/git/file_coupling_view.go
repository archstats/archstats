package git

import (
	"github.com/archstats/archstats/core"
	"github.com/archstats/archstats/extensions/git/commits"
	"github.com/samber/lo"
)

func (e *extension) fileCouplingViewFactory(results *core.Results) *core.View {
	files := lo.Keys(results.FileToComponent)

	sharedCommits := commits.PairsToCommitsInCommon(files, e.splittedCommits.FileToCommitHashes())
	dayBucketSharedCommitCounts := map[int]map[string]commits.CommitHashes{}

	for days, splittedCommits := range e.splittedCommits.DayBuckets() {
		dayBucketSharedCommitCounts[days] = commits.PairsToCommitsInCommon(files, splittedCommits.FileToCommitHashes())
	}

	mappedDayBuckets := lo.MapValues(e.splittedCommits.DayBuckets(), func(splitted *commits.Splitted, _ int) map[string]commits.CommitHashes {
		return splitted.FileToCommitHashes()
	})
	rows := sharedCommitsToRows(files, sharedCommits, dayBucketSharedCommitCounts, e.splittedCommits.FileToCommitHashes(), mappedDayBuckets)

	return &core.View{
		Columns: sharedCommitColumns(e.DayBuckets),
		Rows:    rows,
	}
}
