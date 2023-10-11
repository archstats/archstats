package commits

import (
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestSharedCommits_Files(t *testing.T) {

	var commits []*PartOfCommit

	commits = append(commits, createSharedCommitBetweenFiles("1", "a", "b", "c")...)
	commits = append(commits, createSharedCommitBetweenFiles("2", "c", "b")...)
	commits = append(commits, createSharedCommitBetweenFiles("3", "a", "b")...)
	commits = append(commits, createSharedCommitBetweenFiles("4", "b", "c")...)
	commits = append(commits, createSharedCommitBetweenFiles("5", "x", "y", "z")...)
	commits = append(commits, createSharedCommitBetweenFiles("6", "x", "a")...)
	commits = append(commits, createSharedCommitBetweenFiles("7", "x", "b", "a")...)
	commits = append(commits, createSharedCommitBetweenFiles("8", "y", "c", "a")...)
	commits = append(commits, createSharedCommitBetweenFiles("9", "z", "b", "d")...)
	commits = append(commits, createSharedCommitBetweenFiles("10", "z", "y", "e")...)

	splittedCommits := lo.GroupBy(commits, func(commit *PartOfCommit) string {
		return commit.Commit
	})

	// Special cases:
	// f should not have any shared commits with anything else, because it is not in any of the commits
	// e should have no shared commits with anything else, because it is only in one commit that has no other files in the list
	// d should have one shared commit with b, because they are both in commit 9
	sharedCommits := GetCommitsInCommonForFilePairs([]string{"a", "b", "c", "d", "e", "f"}, splittedCommits)

	assert.Len(t, sharedCommits, 4)

	assert.Len(t, sharedCommits["a:b"], 3)
	assert.ElementsMatch(t, sharedCommits["a:b"], []string{"1", "3", "7"})

	assert.Len(t, sharedCommits["a:c"], 2)
	assert.ElementsMatch(t, sharedCommits["a:c"], []string{"1", "8"})

	assert.Len(t, sharedCommits["b:c"], 3)
	assert.ElementsMatch(t, sharedCommits["b:c"], []string{"1", "2", "4"})

	assert.Len(t, sharedCommits["b:d"], 1)
	assert.ElementsMatch(t, sharedCommits["b:d"], []string{"9"})

}

func TestSharedCommits_Components(t *testing.T) {

	var commits []*PartOfCommit

	commits = append(commits, createSharedCommitBetweenComponents("1", "a", "b", "c")...)
	commits = append(commits, createSharedCommitBetweenComponents("2", "c", "b")...)
	commits = append(commits, createSharedCommitBetweenComponents("3", "a", "b")...)
	commits = append(commits, createSharedCommitBetweenComponents("4", "b", "c")...)
	commits = append(commits, createSharedCommitBetweenComponents("5", "x", "y", "z")...)
	commits = append(commits, createSharedCommitBetweenComponents("6", "x", "a")...)
	commits = append(commits, createSharedCommitBetweenComponents("7", "x", "b", "a")...)
	commits = append(commits, createSharedCommitBetweenComponents("8", "y", "c", "a")...)
	commits = append(commits, createSharedCommitBetweenComponents("9", "z", "b", "d")...)
	commits = append(commits, createSharedCommitBetweenComponents("10", "z", "y", "e")...)

	splittedCommits := lo.GroupBy(commits, func(commit *PartOfCommit) string {
		return commit.Commit
	})

	// Special cases:
	// f should not have any shared commits with anything else, because it is not in any of the commits
	// e should have no shared commits with anything else, because it is only in one commit that has no other components in the list
	// d should have one shared commit with b, because they are both in commit 9
	sharedCommits := GetCommitsInCommonForComponentPairs([]string{"a", "b", "c", "d", "e", "f"}, splittedCommits)

	assert.Len(t, sharedCommits, 4)

	assert.Len(t, sharedCommits["a:b"], 3)
	assert.ElementsMatch(t, sharedCommits["a:b"], []string{"1", "3", "7"})

	assert.Len(t, sharedCommits["a:c"], 2)
	assert.ElementsMatch(t, sharedCommits["a:c"], []string{"1", "8"})

	assert.Len(t, sharedCommits["b:c"], 3)
	assert.ElementsMatch(t, sharedCommits["b:c"], []string{"1", "2", "4"})

	assert.Len(t, sharedCommits["b:d"], 1)
	assert.ElementsMatch(t, sharedCommits["b:d"], []string{"9"})

}

func createSharedCommitBetweenFiles(commitHash string, files ...string) []*PartOfCommit {
	return lo.Map(files, func(file string, _ int) *PartOfCommit {
		return &PartOfCommit{
			File:   file,
			Commit: commitHash,
		}
	})
}

func createSharedCommitBetweenComponents(commitHash string, files ...string) []*PartOfCommit {
	return lo.Map(files, func(file string, _ int) *PartOfCommit {
		return &PartOfCommit{
			Component: file,
			Commit:    commitHash,
		}
	})
}
