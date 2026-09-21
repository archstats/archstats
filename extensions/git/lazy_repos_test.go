package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPartialCloneFilter_ReadsWhatTheCloneWasFilteredBy(t *testing.T) {
	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		assert.NoError(t, exec.Command("git", append([]string{"-C", dir}, args...)...).Run())
	}
	run("init", "-q")
	run("remote", "add", "origin", "https://example.invalid/thing.git")

	_, _, ok := partialCloneFilter(dir)
	assert.False(t, ok, "an ordinary clone carries no filter")

	// What `git clone --filter=blob:none` leaves behind.
	run("config", "remote.origin.partialclonefilter", "blob:none")

	setting, filter, ok := partialCloneFilter(dir)
	assert.True(t, ok)
	assert.Equal(t, "blob:none", filter)
	assert.Equal(t, "remote.origin.partialclonefilter", setting)
}

func TestLazyRepo_AdviceUnsetsTheFilterBeforeRefetching(t *testing.T) {
	// `git fetch --refetch` on its own honours the filter it finds and
	// downloads nothing, so advice that omits the unset is advice that does
	// not work. Measured: it returned in seven seconds and left the clone
	// exactly as partial as before.
	advice := LazyRepo{Dir: "/w/sakai", Filter: "blob:none", Setting: "remote.origin.partialclonefilter"}.Advice()
	assert.Contains(t, advice, "config --unset remote.origin.partialclonefilter")
	assert.Contains(t, advice, "fetch --refetch")
	assert.Less(t, indexOf(advice, "--unset"), indexOf(advice, "--refetch"), "the filter has to go before the fetch")
}

func indexOf(haystack, needle string) int {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}

// A repository that holds every object it needs must not be warned about,
// however it was cloned: `git fetch --refetch` re-adds the filter setting, so
// a clone that has since been made whole still reads as partial.
func TestMissesObjects_QuietWhenEverythingIsLocal(t *testing.T) {
	dir := t.TempDir()
	// Hermetic: whatever the machine has configured globally — a signing key,
	// a template, a hook — must not reach into this repository.
	run := func(args ...string) {
		t.Helper()
		base := []string{"-C", dir, "-c", "user.name=t", "-c", "user.email=t@t", "-c", "commit.gpgsign=false", "-c", "core.hooksPath=/dev/null"}
		cmd := exec.Command("git", append(base, args...)...)
		out, err := cmd.CombinedOutput()
		assert.NoErrorf(t, err, "git %v: %s", args, out)
	}
	run("init", "-q")
	assert.NoError(t, os.WriteFile(filepath.Join(dir, "a.txt"), []byte("hello\n"), 0o600))
	run("add", "a.txt")
	run("commit", "-qm", "first")

	assert.False(t, missesObjects(dir), "a complete repository needs no warning")
}
