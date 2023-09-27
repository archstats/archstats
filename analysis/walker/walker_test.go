package walker

import (
	"github.com/RyanSusana/archstats/analysis/file"
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
