package git

import (
	"github.com/archstats/archstats/core"
	"github.com/archstats/archstats/extensions/git/commits"
	"github.com/samber/lo"
)

// DirectoryCouplingMinShared is the fewest shared commits a directory pair needs to be kept.
const DirectoryCouplingMinShared = 2

func (e *extension) directoryCouplingViewFactory(results *core.Results) *core.View {
	directory := lo.Keys(results.DirectoryToFiles)

	sharedCommits := commits.PairsToCommitsInCommon(directory, e.couplingCommits.DirectoryToCommitHashes())
	dayBucketSharedCommitCounts := map[int]map[commits.Pair]commits.CommitHashes{}

	for days, split := range e.couplingCommits.DayBuckets() {
		dayBucketSharedCommitCounts[days] = commits.PairsToCommitsInCommon(directory, split.DirectoryToCommitHashes())
	}

	mappedDayBuckets := lo.MapValues(e.couplingCommits.DayBuckets(), func(splitted *commits.Splitted, _ int) map[string]commits.CommitHashes {
		return splitted.DirectoryToCommitHashes()
	})
	rows := sharedCommitsToRows(sharedCommits, dayBucketSharedCommitCounts, e.couplingCommits.DirectoryToCommitHashes(), mappedDayBuckets)
	// Directory pairs that shared one commit are most of the table and say
	// little: kept from two shared commits up.
	kept := rows[:0]
	for _, r := range rows {
		if n, ok := r.Data[SharedCommitCount].(int); ok && n < DirectoryCouplingMinShared {
			continue
		}
		kept = append(kept, r)
	}
	rows = kept

	return &core.View{
		Columns: sharedCommitColumns(e.DayBuckets),
		Rows:    rows,
	}
}
