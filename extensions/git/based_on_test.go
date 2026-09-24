package git

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/archstats/archstats/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A repository last touched in 2020 still has a last 30 days: its own. The
// windows count back from the newest commit scanned, so the same commit
// reads the same on any day, and the anchor is recorded with the snapshot.
func TestWindowsCountBackFromTheNewestCommit(t *testing.T) {
	dir := t.TempDir()
	commit := func(name, date string) {
		t.Helper()
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(date+"\n"), 0o644))
		for _, args := range [][]string{{"add", "."}, {"commit", "-q", "-m", name}} {
			cmd := exec.Command("git", args...)
			cmd.Dir = dir
			cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=Ann", "GIT_AUTHOR_EMAIL=ann@x.org", "GIT_COMMITTER_NAME=Ann", "GIT_COMMITTER_EMAIL=ann@x.org",
				"GIT_AUTHOR_DATE="+date, "GIT_COMMITTER_DATE="+date, "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1")
			out, err := cmd.CombinedOutput()
			require.NoErrorf(t, err, "git %v: %s", args, out)
		}
	}
	init := exec.Command("git", "init", "-q", "-b", "main")
	init.Dir = dir
	require.NoError(t, init.Run())
	commit("old.txt", "2019-06-01T10:00:00Z")
	commit("recent.txt", "2020-01-10T10:00:00Z")
	commit("recent.txt", "2020-01-20T10:00:00Z")

	ext := Extension().(*extension)
	results, err := core.New(&core.Config{RootPath: dir, Extensions: []core.Extension{ext}}).Analyze()
	require.NoError(t, err)
	assert.Equal(t, "2020-01-20T10:00:00Z", results.SnapshotInfo["git_based_on"])

	var total, recent string
	for _, r := range results.StatRecordsByFile["./recent.txt"] {
		switch r.StatType {
		case "git__commits__total":
			total = fmt.Sprint(r.Value)
		case "git__commits__last_30_days":
			recent = fmt.Sprint(r.Value)
		}
	}
	assert.NotEmpty(t, recent)
	assert.Equal(t, total, recent, "both commits to recent.txt fall in its last 30 days")
}
