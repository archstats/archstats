package walker

import (
	"bytes"
	"github.com/archstats/archstats/core/file"
	"github.com/stretchr/testify/assert"
	"os"
	"sync"
	"testing"
)

func TestGetAllFiles(t *testing.T) {
	allFiles := GetAllFiles("./test_example")

	assert.Len(t, allFiles, 4)
	for _, file := range allFiles {
		assert.NotContains(t, file.Path(), "ignore")
	}
}

func TestWalkDirectoryConcurrently(t *testing.T) {

	lock := sync.Mutex{}
	var walkedFiles []string
	WalkDirectoryConcurrently("./test_example", func(file file.File) {
		assert.NotContains(t, file.Path(), "ignore")
		content := string(file.Content())
		assert.Equal(t, "should not be ignored", content, "file '%s' should be ignored", file.Path())
		lock.Lock()
		walkedFiles = append(walkedFiles, file.Path())
		lock.Unlock()
	})

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
	WalkDirectoryConcurrently(tempDir, func(file file.File) {
		lock.Lock()
		walkedFiles = append(walkedFiles, file.Path())
		lock.Unlock()
	})

	assert.Len(t, walkedFiles, 1)
	assert.Contains(t, walkedFiles[0], "test.txt")
	assert.NotContains(t, walkedFiles[0], "test.db")
}
