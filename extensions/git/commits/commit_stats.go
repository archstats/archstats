package commits

import (
	"github.com/samber/lo"
	"math"
	"time"
)

type CommitStats struct {
	CommitCount                int
	AdditionCount              int
	DeletionCount              int
	UniqueFileChangeCount      int
	UniqueComponentChangeCount int
	UniqueAuthorCount          int
	OldestCommitAgeInDays      int
}

// GetStats Gets the stats for a group of commits.
// Assumes that commits are sorted in reverse chronological order.
// TODO - most of this can be greatly optimized by doing a single pass over the commitParts
// DO THIS AFTER WRITING TESTS
func GetStats(basedOn time.Time, commitParts []*PartOfCommit) *CommitStats {
	return &CommitStats{
		CommitCount:                commitCount(commitParts),
		AdditionCount:              additionCount(commitParts),
		DeletionCount:              deletionCount(commitParts),
		UniqueFileChangeCount:      uniqueFileChangeCount(commitParts),
		UniqueComponentChangeCount: uniqueComponentChangeCount(commitParts),
		UniqueAuthorCount:          uniqueAuthorCount(commitParts),
		OldestCommitAgeInDays:      ageInDays(basedOn, commitParts),
	}
}

func uniqueAuthorCount(commitParts []*PartOfCommit) int {
	return len(lo.UniqBy(commitParts, func(part *PartOfCommit) string {
		return part.Author
	}))
}

func uniqueComponentChangeCount(commitParts []*PartOfCommit) int {
	return len(lo.UniqBy(commitParts, func(part *PartOfCommit) string {
		return part.Component
	}))
}

func uniqueFileChangeCount(commitParts []*PartOfCommit) int {
	return len(lo.UniqBy(commitParts, func(part *PartOfCommit) string {
		return part.File
	}))
}

func commitCount(commitParts []*PartOfCommit) int {
	return len(lo.UniqBy(commitParts, func(part *PartOfCommit) string {
		return part.Commit
	}))
}

func deletionCount(commitParts []*PartOfCommit) int {
	return lo.SumBy(commitParts, func(part *PartOfCommit) int {
		return part.Deletions
	})
}

func additionCount(commitParts []*PartOfCommit) int {
	return lo.SumBy(commitParts, func(part *PartOfCommit) int {
		return part.Additions
	})
}

func ageInDays(from time.Time, commits []*PartOfCommit) int {
	if len(commits) > 0 {
		return dayDiff(from, commits[len(commits)-1].Time)
	}
	return 0
}

func dayDiff(from time.Time, to time.Time) int {
	return int(math.Abs(to.Sub(from).Hours() / 24))
}
