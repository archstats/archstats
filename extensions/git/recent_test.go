package git

import (
	"testing"
	"time"

	"github.com/archstats/archstats/extensions/git/commits"
	"github.com/stretchr/testify/assert"
)

func part(commit string, daysAgo, deletions int) *commits.PartOfCommit {
	return &commits.PartOfCommit{Commit: commit, File: "a.go", Deletions: deletions,
		Time: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC).AddDate(0, 0, -daysAgo)}
}

// The commit that added a file is not a change to it: a file written last
// month read as one of the busiest.
func TestRecentChangesLeaveOutTheAddingCommit(t *testing.T) {
	added, edit := part("add", 40, 0), part("edit", 10, 3)
	got := recentChanges([]*commits.PartOfCommit{edit, added}, []*commits.PartOfCommit{edit, added})
	assert.Equal(t, []*commits.PartOfCommit{edit}, got)
}

// History cut short starts with a change, which deletes lines: it stays.
func TestRecentChangesKeepAnOldestCommitThatDeleted(t *testing.T) {
	first, edit := part("first", 40, 5), part("edit", 10, 3)
	got := recentChanges([]*commits.PartOfCommit{edit, first}, []*commits.PartOfCommit{edit, first})
	assert.Len(t, got, 2)
}

// A sweep is left out of countable already; the adding commit is still
// found among all of the file's commits.
func TestRecentChangesFindTheAddingCommitAmongSweeps(t *testing.T) {
	added, sweep, edit := part("add", 400, 0), part("sweep", 30, 2), part("edit", 10, 3)
	got := recentChanges([]*commits.PartOfCommit{edit, sweep, added}, []*commits.PartOfCommit{edit, added})
	assert.Equal(t, []*commits.PartOfCommit{edit}, got)
}

// A group counts a commit once, however many of its files it touched.
func TestRecentCommitsCountACommitOncePerGroup(t *testing.T) {
	x := &commits.CommitStats{RecencyWeights: map[string]float64{"a": 1, "b": 0.5}}
	y := &commits.CommitStats{RecencyWeights: map[string]float64{"a": 1}}
	assert.InDelta(t, 1.5, RecentCommits([]interface{}{x, y}), 1e-9)
}
