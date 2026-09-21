package git

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
)

// A repository, as discovery recognises one: a directory with a .git in it.
func checkout(t *testing.T, dir string) {
	t.Helper()
	assert.NoError(t, os.MkdirAll(filepath.Join(dir, ".git"), 0o755))
}

func TestHasRepos_SeesTheCheckoutsInsideAWorkspace(t *testing.T) {
	// The layout that reported no git at all: nothing at the root, the
	// repositories a couple of levels down.
	root := t.TempDir()
	checkout(t, filepath.Join(root, "repos", "alpha"))
	checkout(t, filepath.Join(root, "repos", "beta"))

	assert.True(t, HasRepos(root))
	assert.ElementsMatch(t, []string{"repos/alpha", "repos/beta"}, findGitRepos(root).repos)
}

func TestHasRepos_SeesARootThatIsItselfARepository(t *testing.T) {
	root := t.TempDir()
	checkout(t, root)

	assert.True(t, HasRepos(root))
	assert.Equal(t, []string{""}, findGitRepos(root).repos, "the root repository is the empty path")
}

func TestHasRepos_SaysNoWithoutOne(t *testing.T) {
	root := t.TempDir()
	assert.NoError(t, os.MkdirAll(filepath.Join(root, "src", "main"), 0o755))

	assert.False(t, HasRepos(root))
}

func TestFindGitRepos_StopsAtARepositoryRatherThanWalkingIt(t *testing.T) {
	// A repository's own working tree is not searched for more repositories:
	// a vendored checkout is part of the repository that holds it.
	root := t.TempDir()
	checkout(t, root)
	checkout(t, filepath.Join(root, "vendor", "thing"))

	assert.Equal(t, []string{""}, findGitRepos(root).repos)
}

func TestFindGitRepos_KeepsWalkingPastADirectoryItCannotRead(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root can read every directory, so nothing here is unreadable")
	}
	if runtime.GOOS == "windows" {
		// Mode bits do not deny a directory listing on Windows, so the
		// directory below is perfectly readable there and there is nothing
		// for the walk to skip. Making one genuinely unreadable needs an ACL
		// this test has no business setting.
		t.Skip("directory permissions are not mode bits on Windows")
	}
	root := t.TempDir()
	checkout(t, filepath.Join(root, "repos", "alpha"))
	locked := filepath.Join(root, "locked")
	assert.NoError(t, os.Mkdir(locked, 0o000))
	// Before the temp directory is removed again.
	t.Cleanup(func() { _ = os.Chmod(locked, 0o700) })

	scan := findGitRepos(root)

	assert.Equal(t, []string{"repos/alpha"}, scan.repos, "one unreadable directory used to cost every repository")
	assert.Contains(t, scan.unreadable, locked, "and the one it could not read is named, not swallowed")
}
