package commits

import (
	"github.com/samber/lo"
	"math"
	"time"
)

type CommitStats struct {
	AdditionCount int
	CommitCount   int

	DeletionCount         int
	OldestCommitAgeInDays int
	// NewestCommitAgeInDays is how long before the anchor the group last
	// changed; -1 when it has no commits.
	NewestCommitAgeInDays      int
	UniqueFileChangeCount      int
	UniqueDirectoryChangeCount int
	UniqueComponentChangeCount int
	UniqueAuthorCount          int
	UniqueAuthors              []string
	UniqueCommits              []string
	FileChanges                []string
	// RecencyWeights is each unique commit's weight by its age: 1 at the
	// anchor, halving every RecencyHalfLifeDays. Summed over a group's
	// unique commits it is the group's recent commit count.
	RecencyWeights map[string]float64
}

// RecencyHalfLifeDays is how long a commit takes to count half as much
// towards recent churn: about six months.
const RecencyHalfLifeDays = 182

// RecencyWeight is what a commit age days before the anchor counts for.
func RecencyWeight(ageInDays int) float64 {
	return math.Pow(0.5, float64(ageInDays)/RecencyHalfLifeDays)
}

// GetStats Gets the stats for a group of commits.
// Assumes that commits are sorted in reverse chronological order.
func GetStats(basedOn time.Time, commitParts []*PartOfCommit) *CommitStats {
	commits := make(map[string]bool, len(commitParts))
	components := make(map[string]bool, len(commitParts))
	files := make(map[string]bool, len(commitParts))
	directories := make(map[string]bool, len(commitParts))
	authors := make(map[string]bool, len(commitParts))
	weights := make(map[string]float64, len(commitParts))

	totalAdditionCount := 0
	totalDeletionCount := 0
	oldestCommitAgeInDays := 0
	newestCommitAgeInDays := -1

	for _, part := range commitParts {
		totalAdditionCount += part.Additions
		totalDeletionCount += part.Deletions

		commits[part.Commit] = true
		// A file that no longer exists belongs to no component. Counting ""
		// gave nearly every author one component they never touched.
		if part.Component != "" {
			components[part.Component] = true
		}
		files[part.File] = true
		directories[part.Directory] = true
		authors[part.Author] = true

		commitAge := dayDiff(basedOn, part.Time)
		weights[part.Commit] = RecencyWeight(commitAge)

		oldestCommitAgeInDays = int(math.Max(float64(commitAge), float64(oldestCommitAgeInDays)))
		if newestCommitAgeInDays < 0 || commitAge < newestCommitAgeInDays {
			newestCommitAgeInDays = commitAge
		}
	}

	return &CommitStats{
		CommitCount:                len(commits),
		AdditionCount:              totalAdditionCount,
		DeletionCount:              totalDeletionCount,
		OldestCommitAgeInDays:      oldestCommitAgeInDays,
		NewestCommitAgeInDays:      newestCommitAgeInDays,
		UniqueFileChangeCount:      len(files),
		UniqueDirectoryChangeCount: len(directories),
		UniqueComponentChangeCount: len(components),
		UniqueAuthorCount:          len(authors),
		UniqueAuthors:              lo.Keys(authors),
		UniqueCommits:              lo.Keys(commits),
		FileChanges:                lo.Keys(files),
		RecencyWeights:             weights,
	}
}

func dayDiff(from time.Time, to time.Time) int {
	return int(math.Abs(to.Sub(from).Hours() / 24))
}
