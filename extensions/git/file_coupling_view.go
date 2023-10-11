package git

import (
	"github.com/archstats/archstats/core"
	"github.com/archstats/archstats/extensions/git/commits"
	"github.com/samber/lo"
)

func (e *extension) fileCouplingViewFactory(results *core.Results) *core.View {
	files := lo.Keys(results.FileToComponent)

	totals := commits.GetCommitsInCommonForFilePairs(files, e.splittedCommits.SplitByCommitHash())
	dayBucketSharedCommitCounts := map[int]map[string]commits.CommitHashes{}

	for days, splittedCommits := range e.splittedCommits.DayBuckets() {
		dayBucketSharedCommitCounts[days] = commits.GetCommitsInCommonForFilePairs(files, splittedCommits.SplitByCommitHash())
	}

	rows := sharedCommitsToRows(files, totals, dayBucketSharedCommitCounts)

	return &core.View{
		Columns: sharedCommitColumns(e.DayBuckets),
		Rows:    rows,
	}
}
