package walker

import (
	"github.com/stretchr/testify/assert"
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
	WalkDirectoryConcurrently("./test_example", func(file OpenedFile) {
		assert.NotContains(t, file.Path(), "ignore")
		assert.Equal(t, "should not be ignored", string(file.Content()), "file '%s' should be ignored", file.Path())
		lock.Lock()
		walkedFiles = append(walkedFiles, file.Path())
		lock.Unlock()
	})

	expectedFilesToWalk := []string{
		"./test_example/subdir2/file6.csv",
		"./test_example/file2.txt",
		"./test_example/file1.txt",
		"./test_example/subdir1/file5.txt",
	}
	assert.ElementsMatch(t, expectedFilesToWalk, walkedFiles)
}
