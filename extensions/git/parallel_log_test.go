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

// Diffing the history in chunks reads exactly what one `git log` reads, in
// the same order, merges included.
func TestChunkedLogReadsWhatOneLogReads(t *testing.T) {
	dir := t.TempDir()
	run := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=Ann", "GIT_AUTHOR_EMAIL=ann@x.org", "GIT_COMMITTER_NAME=Ann", "GIT_COMMITTER_EMAIL=ann@x.org",
			"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1")
		out, err := cmd.CombinedOutput()
		require.NoErrorf(t, err, "git %v: %s", args, out)
		return string(out)
	}
	write := func(name, content string) {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644))
	}
	run("init", "-q", "-b", "main")
	for i := range 12 {
		write(fmt.Sprintf("f%d.txt", i%4), fmt.Sprintf("line %d\n", i))
		run("add", ".")
		run("commit", "-q", "-m", fmt.Sprintf("commit %d", i))
		if i%4 == 3 {
			branch := fmt.Sprintf("b%d", i)
			run("checkout", "-q", "-b", branch)
			write(branch+".txt", "branch\n")
			run("add", ".")
			run("commit", "-q", "-m", branch)
			run("checkout", "-q", "main")
			run("merge", "-q", "--no-ff", "--no-commit", branch)
			write("resolved"+branch+".txt", "written while merging\n")
			run("add", ".")
			run("commit", "-q", "-m", "merge "+branch)
		}
	}

	old := minPerChunk
	minPerChunk = 1
	t.Cleanup(func() { minPerChunk = old })
	for range 20 {
		commits, err := (&extension{}).parseGitLog(dir)
		require.NoError(t, err)

		oneLog := parseGitLogString(dir, run("-c", "core.quotepath=off", "log", "HEAD", "--no-merges", "--numstat", "-M", "--pretty=format:"+gitLogFormat))
		var want, got []string
		for _, c := range oneLog {
			want = append(want, c.Hash)
		}
		merges := 0
		for _, c := range commits {
			if c.combined {
				merges++
				continue
			}
			got = append(got, c.Hash)
		}
		assert.Equal(t, want, got)
		assert.Equal(t, 3, merges, "each merge wrote a file of its own")
	}
}
