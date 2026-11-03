package git

import (
	"github.com/archstats/archstats/extensions/git/commits"
	"github.com/samber/lo"
)

func UniqueAuthors(thingsToMerge []interface{}) interface{} {
	commitSlice := lo.Map(thingsToMerge, func(thing interface{}, _ int) *commits.CommitStats {
		return thing.(*commits.CommitStats)
	})
	authors := make(map[string]bool)
	for _, commit := range commitSlice {
		for _, author := range commit.UniqueAuthors {
			authors[author] = true
		}
	}
	return len(authors)
}

func UniqueCommits(thingsToMerge []interface{}) interface{} {
	commitSlice := lo.Map(thingsToMerge, func(thing interface{}, _ int) *commits.CommitStats {
		return thing.(*commits.CommitStats)
	})
	commits := make(map[string]bool)
	for _, commit := range commitSlice {
		for _, commitHash := range commit.UniqueCommits {
			commits[commitHash] = true
		}
	}
	return len(commits)
}

// RecentCommits sums the recency weights of the group's unique commits: a
// commit that touched three of its files counts once.
func RecentCommits(thingsToMerge []interface{}) interface{} {
	weights := make(map[string]float64)
	for _, thing := range thingsToMerge {
		for commitHash, weight := range thing.(*commits.CommitStats).RecencyWeights {
			weights[commitHash] = weight
		}
	}
	total := 0.0
	for _, weight := range weights {
		total += weight
	}
	return total
}

func UniqueFiles(thingsToMerge []interface{}) interface{} {
	commitSlice := lo.Map(thingsToMerge, func(thing interface{}, _ int) *commits.CommitStats {
		return thing.(*commits.CommitStats)
	})
	files := make(map[string]bool)
	for _, commit := range commitSlice {
		for _, file := range commit.FileChanges {
			files[file] = true
		}
	}
	return len(files)
}
func TotalAdditions(thingsToMerge []interface{}) interface{} {
	commitSlice := lo.Map(thingsToMerge, func(thing interface{}, _ int) *commits.CommitStats {
		return thing.(*commits.CommitStats)
	})
	totalAdditions := 0
	for _, commit := range commitSlice {
		totalAdditions += commit.AdditionCount
	}
	return totalAdditions
}
func TotalDeletions(thingsToMerge []interface{}) interface{} {
	commitSlice := lo.Map(thingsToMerge, func(thing interface{}, _ int) *commits.CommitStats {
		return thing.(*commits.CommitStats)
	})
	totalDeletions := 0
	for _, commit := range commitSlice {
		totalDeletions += commit.DeletionCount
	}
	return totalDeletions
}
