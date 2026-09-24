package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A file created while resolving a merge exists in no branch commit. Git
// reports it only for the merge, and a merge reported no files, so the file
// read as never committed.
func TestAFileChangedOnlyInAMergeHasHistory(t *testing.T) {
	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=Ann", "GIT_AUTHOR_EMAIL=ann@x.org", "GIT_COMMITTER_NAME=Ann", "GIT_COMMITTER_EMAIL=ann@x.org",
			// No signing, hooks or aliases from whoever runs the test.
			"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1")
		out, err := cmd.CombinedOutput()
		require.NoErrorf(t, err, "git %v: %s", args, out)
	}
	write := func(name, content string) {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644))
	}
	run("init", "-q", "-b", "main")
	write("a.txt", "one\n")
	run("add", ".")
	run("commit", "-q", "-m", "first")
	run("checkout", "-q", "-b", "feature")
	write("b.txt", "feature\n")
	run("add", ".")
	run("commit", "-q", "-m", "feature")
	run("checkout", "-q", "main")
	write("a.txt", "one\ntwo\n")
	run("commit", "-q", "-am", "main")
	run("merge", "-q", "--no-ff", "--no-commit", "feature")
	write("merge-only.txt", "written while merging\n")
	run("add", ".")
	run("commit", "-q", "-m", "merge")

	// And a merge with nothing of its own: it reports no files at all.
	run("checkout", "-q", "-b", "other")
	write("c.txt", "other\n")
	run("add", ".")
	run("commit", "-q", "-m", "other")
	run("checkout", "-q", "main")
	run("merge", "-q", "--no-ff", "-m", "clean merge", "other")

	commits, err := (&extension{}).parseGitLog(dir)
	require.NoError(t, err)
	perFile := map[string]int{}
	for _, c := range commits {
		for _, f := range c.Files {
			perFile[f.Path]++
		}
	}
	assert.Equal(t, 1, perFile["merge-only.txt"], "the merge wrote it")
	// Branch work is still counted once, not again by the merge.
	assert.Equal(t, 1, perFile["b.txt"])
	assert.Equal(t, 2, perFile["a.txt"])
	assert.Equal(t, 1, perFile["c.txt"])
}
