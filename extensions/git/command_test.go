package git

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fixtureRepo makes a repository with two commits, built without the
// config of whoever runs the test.
func fixtureRepo(t *testing.T, file, content string) string {
	t.Helper()
	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(gitEnv(os.Environ()),
			"GIT_AUTHOR_NAME=Zoë", "GIT_AUTHOR_EMAIL=zoe@x.org", "GIT_AUTHOR_DATE=2026-03-29T01:30:00Z",
			"GIT_COMMITTER_NAME=Zoë", "GIT_COMMITTER_EMAIL=zoe@x.org", "GIT_COMMITTER_DATE=2026-03-29T01:30:00Z",
			"GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_NOSYSTEM=1")
		out, err := cmd.CombinedOutput()
		require.NoErrorf(t, err, "git %v: %s", args, out)
	}
	write := func(content string) {
		require.NoError(t, os.WriteFile(filepath.Join(dir, file), []byte(content), 0o644))
	}
	run("init", "-q", "-b", "main")
	write(content)
	run("add", ".")
	run("commit", "-q", "-m", "première")
	write(content + "more\n")
	run("commit", "-q", "-am", "second")
	return dir
}

// read is what a scan keeps of a repository's history.
func read(t *testing.T, dir string) []string {
	t.Helper()
	commits, err := (&extension{}).parseGitLog(dir)
	require.NoError(t, err)
	var out []string
	for _, c := range commits {
		for _, f := range c.Files {
			out = append(out, fmt.Sprintf("%s %s %s %s %s +%d -%d", c.Hash, c.Time.Format("2006-01-02T15:04:05Z07:00"), c.AuthorName, c.Message, f.Path, f.Additions, f.Deletions))
		}
	}
	return out
}

// The user's git config holds settings that change git's output. None of
// them may change what a scan reads.
func TestUserGitConfigDoesNotChangeTheHistoryRead(t *testing.T) {
	dir := fixtureRepo(t, "a.txt", "one\n")
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	want := read(t, dir)
	require.Len(t, want, 2)

	global := filepath.Join(t.TempDir(), "gitconfig")
	require.NoError(t, os.WriteFile(global, []byte(
		"[core]\n\tabbrev = 16\n\tquotepath = on\n"+
			"[log]\n\tshowSignature = true\n"+
			"[diff]\n\talgorithm = patience\n\trenameLimit = 1\n"+
			"[i18n]\n\tlogOutputEncoding = ISO-8859-1\n"), 0o644))
	t.Setenv("GIT_CONFIG_GLOBAL", global)
	assert.Equal(t, want, read(t, dir))
}

// -C does not override GIT_DIR. Set by a git hook, or by anything else that
// runs the scan from inside a repository, it made every repository read as
// that one.
func TestAnInheritedGitDirDoesNotRedirectTheScan(t *testing.T) {
	scanned := fixtureRepo(t, "scanned.txt", "one\n")
	other := fixtureRepo(t, "other.txt", "two\n")
	want := read(t, scanned)

	t.Setenv("GIT_DIR", filepath.Join(other, ".git"))
	t.Setenv("GIT_WORK_TREE", other)
	assert.Equal(t, want, read(t, scanned))
}

func TestGitEnvDropsOnlyRepositoryLocatingVariables(t *testing.T) {
	got := gitEnv([]string{"GIT_DIR=/x", "GIT_INDEX_FILE=/i", "GIT_CONFIG_PARAMETERS='core.abbrev'='12'", "PATH=/bin", "GIT_SSH_COMMAND=ssh", "HOME=/h"})
	assert.Equal(t, []string{"PATH=/bin", "GIT_SSH_COMMAND=ssh", "HOME=/h"}, got)
}
