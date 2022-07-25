package views

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestDirectoryTree(t *testing.T) {
	dirPaths := []string{
		"/users/bob/a",
		"/users/bob/a/b/c",
		"/users/bob/a/b/c/d/e/f",
		"/users/bob/a/b/d",
		"/users/bob/a/e/f",
		"/users/bob/a/e/g",
	}
	missingPaths := []string{
		"/users/bob/a/b",
		"/users/bob/a/e",
		"/users/bob/a/b/c/d/e",
		"/users/bob/a/b/c/d",
	}
	allPaths := append(dirPaths, missingPaths...)

	dirs := createDirectoryTree("/users/bob/a/", dirPaths)

	assert.Equal(t, len(allPaths), len(dirs))
	assert.Contains(t, toPaths(dirs["/users/bob/a"].subtree()), "/users/bob/a/b/c/d/e/f")
	assert.Len(t, toPaths(dirs["/users/bob/a/b/c/d/e/f"].subtree()), 1)

	for _, path := range missingPaths {
		assert.Contains(t, toPaths(dirs["/users/bob/a"].subtree()), path)
	}
	assert.Len(t, toPaths(dirs["/users/bob/a"].subtree()), len(missingPaths)+len(dirPaths))
}
