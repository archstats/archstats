package git

import (
	"testing"
	"github.com/stretchr/testify/assert"
)

func TestGitCommitFiltering(t *testing.T) {
	commits := []*rawCommit{
		{
			Hash: "commit1",
			Files: []*rawPartOfCommit{
				{Path: "file1"},
				{Path: "file2"},
			},
		},
		{
			Hash: "commit2",
			Files: []*rawPartOfCommit{
				{Path: "file1"},
				{Path: "file2"},
				{Path: "file3"},
			},
		},
		{
			Hash: "commit3",
			Files: []*rawPartOfCommit{
				{Path: "file1"},
			},
		},
	}

	t.Run("Filter with limit 2", func(t *testing.T) {
		e := &extension{
			MaxChangesPerCommit: 2,
		}

		var filtered []*rawCommit
		for _, commit := range commits {
			if e.MaxChangesPerCommit > 0 && len(commit.Files) > e.MaxChangesPerCommit {
				continue
			}
			filtered = append(filtered, commit)
		}

		assert.Len(t, filtered, 2)
		assert.Equal(t, "commit1", filtered[0].Hash)
		assert.Equal(t, "commit3", filtered[1].Hash)
	})

	t.Run("Filter disabled (0)", func(t *testing.T) {
		e := &extension{
			MaxChangesPerCommit: 0,
		}

		var filtered []*rawCommit
		for _, commit := range commits {
			if e.MaxChangesPerCommit > 0 && len(commit.Files) > e.MaxChangesPerCommit {
				continue
			}
			filtered = append(filtered, commit)
		}

		assert.Len(t, filtered, 3)
	})

	t.Run("Filter disabled (negative)", func(t *testing.T) {
		e := &extension{
			MaxChangesPerCommit: -1,
		}

		var filtered []*rawCommit
		for _, commit := range commits {
			if e.MaxChangesPerCommit > 0 && len(commit.Files) > e.MaxChangesPerCommit {
				continue
			}
			filtered = append(filtered, commit)
		}

		assert.Len(t, filtered, 3)
	})
}
