package commits

import (
	"github.com/samber/lo"
	"math"
	"time"
)

type CommitStats struct {
	AdditionCount int
	CommitCount   int

	DeletionCount              int
	OldestCommitAgeInDays      int
	UniqueFileChangeCount      int
	UniqueDirectoryChangeCount int
	UniqueComponentChangeCount int
	UniqueAuthorCount          int
	UniqueAuthors              []string
	UniqueCommits              []string
	FileChanges                []string
}

// GetStats Gets the stats for a group of commits.
// Assumes that commits are sorted in reverse chronological order.
func GetStats(basedOn time.Time, commitParts []*PartOfCommit) *CommitStats {
	commits := make(map[string]bool, len(commitParts))
	components := make(map[string]bool, len(commitParts))
	files := make(map[string]bool, len(commitParts))
	directories := make(map[string]bool, len(commitParts))
	authors := make(map[string]bool, len(commitParts))

	totalAdditionCount := 0
	totalDeletionCount := 0
	oldestCommitAgeInDays := 0

	for _, part := range commitParts {
		totalAdditionCount += part.Additions
		totalDeletionCount += part.Deletions

		commits[part.Commit] = true
		components[part.Component] = true
		files[part.File] = true
		directories[part.Directory] = true
		authors[part.Author] = true

		commitAge := dayDiff(basedOn, part.Time)

		oldestCommitAgeInDays = int(math.Max(float64(commitAge), float64(oldestCommitAgeInDays)))
	}

	return &CommitStats{
		CommitCount:                len(commits),
		AdditionCount:              totalAdditionCount,
		DeletionCount:              totalDeletionCount,
		UniqueFileChangeCount:      len(files),
		UniqueDirectoryChangeCount: len(directories),
		UniqueComponentChangeCount: len(components),
		UniqueAuthorCount:          len(authors),
		UniqueAuthors:              lo.Keys(authors),
		UniqueCommits:              lo.Keys(commits),
		FileChanges:                lo.Keys(files),
	}
}

func dayDiff(from time.Time, to time.Time) int {
	return int(math.Abs(to.Sub(from).Hours() / 24))
}
