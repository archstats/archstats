package git

import (
	"github.com/samber/lo"
	"math"
	"time"
)

type commitStats struct {
	commitCount                int
	additionCount              int
	deletionCount              int
	uniqueFileChangeCount      int
	uniqueComponentChangeCount int
	uniqueAuthorCount          int
	oldestCommitAgeInDays      int
}

// Gets the stats for a group of commits.
// Assumes that commits are sorted in reverse chronological order.
// TODO - most of this can be greatly optimized by doing a single pass over the commitParts
func getCommitStats(basedOn time.Time, commitParts []*partOfCommit) *commitStats {
	return &commitStats{
		commitCount:                commitCount(commitParts),
		additionCount:              additionCount(commitParts),
		deletionCount:              deletionCount(commitParts),
		uniqueFileChangeCount:      uniqueFileChangeCount(commitParts),
		uniqueComponentChangeCount: uniqueComponentChangeCount(commitParts),
		uniqueAuthorCount:          uniqueAuthorCount(commitParts),

		oldestCommitAgeInDays: ageInDays(basedOn, commitParts),
	}
}

func uniqueAuthorCount(commitParts []*partOfCommit) int {
	return len(lo.UniqBy(commitParts, func(part *partOfCommit) string {
		return part.author
	}))
}

func uniqueComponentChangeCount(commitParts []*partOfCommit) int {
	return len(lo.UniqBy(commitParts, func(part *partOfCommit) string {
		return part.component
	}))
}

func uniqueFileChangeCount(commitParts []*partOfCommit) int {
	return len(lo.UniqBy(commitParts, func(part *partOfCommit) string {
		return part.file
	}))
}

func commitCount(commitParts []*partOfCommit) int {
	return len(lo.UniqBy(commitParts, func(part *partOfCommit) string {
		return part.commit
	}))
}

func deletionCount(commitParts []*partOfCommit) int {
	return lo.SumBy(commitParts, func(part *partOfCommit) int {
		return part.deletions
	})
}

func additionCount(commitParts []*partOfCommit) int {
	return lo.SumBy(commitParts, func(part *partOfCommit) int {
		return part.additions
	})
}

func ageInDays(from time.Time, commits []*partOfCommit) int {
	if len(commits) > 0 {
		return dayDiff(from, commits[len(commits)-1].time)
	}
	return 0
}

func dayDiff(from time.Time, to time.Time) int {
	return int(math.Abs(to.Sub(from).Hours() / 24))
}
