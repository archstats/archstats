package walker

import (
	"bytes"
	"github.com/archstats/archstats/core/file"
	"github.com/stretchr/testify/assert"
	"os"
	"runtime"
	"strings"
	"sync"
	"testing"
)

func TestGetAllFiles(t *testing.T) {
	allFiles, err := GetAllFiles("./test_example")

	assert.NoError(t, err)
	assert.Len(t, allFiles, 4)
	for _, file := range allFiles {
		assert.NotContains(t, file.Path(), "ignore")
	}
}

func TestWalkDirectoryConcurrently(t *testing.T) {

	lock := sync.Mutex{}
	var walkedFiles []string
	err := WalkDirectoryConcurrently("./test_example", func(file file.File) {
		assert.NotContains(t, file.Path(), "ignore")
		content := string(file.Content())
		assert.Equal(t, "should not be ignored", content, "file '%s' should be ignored", file.Path())
		lock.Lock()
		walkedFiles = append(walkedFiles, file.Path())
		lock.Unlock()
	})
	assert.NoError(t, err)

	expectedFilesToWalk := []string{
		"subdir2/file6.csv",
		"./file2.txt",
		"./file1.txt",
		"subdir1/file5.txt",
	}
	assert.ElementsMatch(t, expectedFilesToWalk, walkedFiles)
}

func TestIsBinary(t *testing.T) {
	tests := []struct {
		name     string
		content  []byte
		expected bool
	}{
		{"empty content", []byte(""), false},
		{"pure text", []byte("hello world this is text"), false},
		{"null byte at start", append([]byte{0}, []byte("text")...), true},
		{"null byte in middle", []byte("hello\x00world"), true},
		{"null byte at end", []byte("hello world\x00"), true},
		{"large text no null", bytes.Repeat([]byte("a"), 10000), false},
		{"large text with null inside limit", append(bytes.Repeat([]byte("a"), 4000), append([]byte{0}, bytes.Repeat([]byte("a"), 4000)...)...), true},
		{"large text with null outside limit", append(bytes.Repeat([]byte("a"), 9000), append([]byte{0}, bytes.Repeat([]byte("a"), 1000)...)...), false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.expected, isBinary(test.content))
		})
	}
}

func TestWalkerSkipsBinaryFiles(t *testing.T) {
	tempDir := t.TempDir()

	// Create a text file
	textFile := tempDir + "/test.txt"
	err := os.WriteFile(textFile, []byte("this is some text"), 0644)
	assert.NoError(t, err)

	// Create a binary file (contains null byte)
	binaryFile := tempDir + "/test.db"
	err = os.WriteFile(binaryFile, []byte("sqlite\x00database\x00file"), 0644)
	assert.NoError(t, err)

	var walkedFiles []string
	lock := sync.Mutex{}
	err = WalkDirectoryConcurrently(tempDir, func(file file.File) {
		lock.Lock()
		walkedFiles = append(walkedFiles, file.Path())
		lock.Unlock()
	})
	assert.NoError(t, err)

	assert.Len(t, walkedFiles, 1)
	assert.Contains(t, walkedFiles[0], "test.txt")
	assert.NotContains(t, walkedFiles[0], "test.db")
}

func TestWalkDirectoryConcurrentlyNonexistentRoot(t *testing.T) {
	err := WalkDirectoryConcurrently(t.TempDir()+"/does-not-exist", func(file file.File) {
		t.Errorf("visitor should not be called, but was called with %s", file.Path())
	})
	assert.Error(t, err)
}

func TestGetAllFilesNonexistentRoot(t *testing.T) {
	allFiles, err := GetAllFiles(t.TempDir() + "/does-not-exist")
	assert.Error(t, err)
	assert.Nil(t, allFiles)
}

func TestGetAllFilesSkipsUnreadableSubdirectory(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("chmod semantics differ on Windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("running as root bypasses permission checks")
	}

	tempDir := t.TempDir()
	err := os.WriteFile(tempDir+"/readable.txt", []byte("readable"), 0o644)
	assert.NoError(t, err)

	lockedDir := tempDir + "/locked"
	err = os.Mkdir(lockedDir, 0o755)
	assert.NoError(t, err)
	err = os.WriteFile(lockedDir+"/hidden.txt", []byte("hidden"), 0o644)
	assert.NoError(t, err)
	err = os.Chmod(lockedDir, 0o000)
	assert.NoError(t, err)
	t.Cleanup(func() {
		_ = os.Chmod(lockedDir, 0o755)
	})

	allFiles, err := GetAllFiles(tempDir)
	assert.NoError(t, err)
	assert.Len(t, allFiles, 1)
	assert.Contains(t, allFiles[0].Path(), "readable.txt")
}

func TestWalkFilesRecoversFromVisitorPanic(t *testing.T) {
	tempDir := t.TempDir()
	err := os.WriteFile(tempDir+"/panics.txt", []byte("this file panics"), 0o644)
	assert.NoError(t, err)
	err = os.WriteFile(tempDir+"/fine.txt", []byte("this file is fine"), 0o644)
	assert.NoError(t, err)

	var walkedFiles []string
	lock := sync.Mutex{}
	err = WalkDirectoryConcurrently(tempDir, func(file file.File) {
		if strings.Contains(file.Path(), "panics") {
			panic("extension blew up")
		}
		lock.Lock()
		walkedFiles = append(walkedFiles, file.Path())
		lock.Unlock()
	})

	assert.NoError(t, err)
	assert.Len(t, walkedFiles, 1)
	assert.Contains(t, walkedFiles[0], "fine.txt")
}
