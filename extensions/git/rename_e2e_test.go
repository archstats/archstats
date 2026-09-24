package git

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/archstats/archstats/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A moved file keeps its history: its commits before the move count, its
// age is its first commit's, and the move itself is recorded but not
// counted as a change.
func TestAMovedFileKeepsItsHistory(t *testing.T) {
	dir := t.TempDir()
	run := func(date string, args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=Ann", "GIT_AUTHOR_EMAIL=ann@x.org", "GIT_COMMITTER_NAME=Ann", "GIT_COMMITTER_EMAIL=ann@x.org",
			"GIT_AUTHOR_DATE="+date, "GIT_COMMITTER_DATE="+date, "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1")
		out, err := cmd.CombinedOutput()
		require.NoErrorf(t, err, "git %v: %s", args, out)
	}
	write := func(name, content string) {
		require.NoError(t, os.MkdirAll(filepath.Dir(filepath.Join(dir, name)), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644))
	}
	body := strings.Repeat("line\n", 40)
	run("2025-01-01T10:00:00Z", "init", "-q", "-b", "main")
	write("old/a.go", body)
	run("2025-01-01T10:00:00Z", "add", ".")
	run("2025-01-01T10:00:00Z", "commit", "-q", "-m", "write")
	write("old/a.go", body+"more\n")
	run("2025-02-01T10:00:00Z", "commit", "-q", "-am", "edit")
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "new"), 0o755))
	run("2025-03-01T10:00:00Z", "mv", "old/a.go", "new/b.go")
	run("2025-03-01T10:00:00Z", "commit", "-q", "-m", "move")
	write("new/b.go", body+"more\nagain\n")
	run("2025-04-01T10:00:00Z", "commit", "-q", "-am", "edit after the move")

	ext := Extension().(*extension)
	results, err := core.New(&core.Config{RootPath: dir, Extensions: []core.Extension{ext}}).Analyze()
	require.NoError(t, err)

	stat := func(file, typ string) string {
		for _, r := range results.StatRecordsByFile[file] {
			if r.StatType == typ {
				return fmt.Sprint(r.Value)
			}
		}
		return ""
	}
	total := stat("new/b.go", "git__commits__total")
	hashes := regexp.MustCompile(`\[([0-9a-f]{7}(?: [0-9a-f]{7})*)\]`).FindStringSubmatch(total)
	require.Len(t, hashes, 2, total)
	assert.Len(t, strings.Fields(hashes[1]), 3, "write, edit and the edit after the move; the move itself changed no lines")

	view, err := results.RenderView("git_commits")
	require.NoError(t, err)
	kinds := map[string]string{}
	for _, row := range view.Rows {
		kinds[fmt.Sprint(row.Data["commit_message"])] = fmt.Sprint(row.Data["change_kind"]) + " " + fmt.Sprint(row.Data["path_at_commit"]) + " -> " + fmt.Sprint(row.Data["file"])
	}
	assert.Equal(t, "modify old/a.go -> new/b.go", kinds["write"])
	assert.Equal(t, "rename new/b.go -> new/b.go", kinds["move"])
	assert.Equal(t, "modify new/b.go -> new/b.go", kinds["edit after the move"])
}
