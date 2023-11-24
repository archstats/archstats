package git

import (
	"github.com/archstats/archstats/core"
	"github.com/archstats/archstats/extensions/git/commits"
	"github.com/samber/lo"
)

func (e *extension) directoryCouplingViewFactory(results *core.Results) *core.View {
	directory := lo.Keys(results.DirectoryToFiles)

	sharedCommits := commits.PairsToCommitsInCommon(directory, e.splittedCommits.DirectoryToCommitHashes())
	dayBucketSharedCommitCounts := map[int]map[string]commits.CommitHashes{}

	for days, split := range e.splittedCommits.DayBuckets() {
		dayBucketSharedCommitCounts[days] = commits.PairsToCommitsInCommon(directory, split.DirectoryToCommitHashes())
	}

	e.splittedCommits.DirectoryToCommitHashes()

	mappedDayBuckets := lo.MapValues(e.splittedCommits.DayBuckets(), func(splitted *commits.Splitted, _ int) map[string]commits.CommitHashes {
		return splitted.DirectoryToCommitHashes()
	})
	rows := sharedCommitsToRows(directory, sharedCommits, dayBucketSharedCommitCounts, e.splittedCommits.DirectoryToCommitHashes(), mappedDayBuckets)

	return &core.View{
		Columns: sharedCommitColumns(e.DayBuckets),
		Rows:    rows,
	}
}
