package core

import (
	"github.com/stretchr/testify/assert"
	"os"
	"runtime"
	"testing"
)

func TestAnalyzeNonexistentRootReturnsError(t *testing.T) {
	results, err := New(&Config{RootPath: t.TempDir() + "/does-not-exist"}).Analyze()

	assert.Error(t, err)
	assert.Nil(t, results)
}

func TestAnalyzeSkipsUnreadableSubdirectory(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("chmod semantics differ on Windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("running as root bypasses permission checks")
	}

	rootDir := t.TempDir()
	err := os.WriteFile(rootDir+"/readable.txt", []byte("readable content"), 0o644)
	assert.NoError(t, err)

	lockedDir := rootDir + "/locked"
	err = os.Mkdir(lockedDir, 0o755)
	assert.NoError(t, err)
	err = os.WriteFile(lockedDir+"/hidden.txt", []byte("hidden content"), 0o644)
	assert.NoError(t, err)
	err = os.Chmod(lockedDir, 0o000)
	assert.NoError(t, err)
	t.Cleanup(func() {
		_ = os.Chmod(lockedDir, 0o755)
	})

	results, err := New(&Config{RootPath: rootDir}).Analyze()

	assert.NoError(t, err)
	assert.NotNil(t, results)
	assert.Contains(t, results.StatRecordsByFile, "./readable.txt")
	assert.NotContains(t, results.StatRecordsByFile, "locked/hidden.txt")
}

// An analyzer runs once, and takes registrations only from Init: a view or
// a convention added during the walk would be seen by some files and not
// others, and a second run would run every analyzer and editor twice.
func TestAnalyzerRunsOnceAndRegistersOnlyInInit(t *testing.T) {
	a := New(&Config{RootPath: t.TempDir()})
	_, err := a.Analyze()
	assert.NoError(t, err)

	_, err = a.Analyze()
	assert.Error(t, err)
	assert.Panics(t, func() { a.RegisterView(&ViewFactory{Name: "late"}) })
}
