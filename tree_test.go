package main

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
	dirs := createDirectoryTree("/users/bob/a/", dirPaths)
	assert.Contains(t, ToPaths(dirs["/users/bob/a"].Subtree()), "/users/bob/a/b/c/d/e/f")
	assert.Len(t, ToPaths(dirs["/users/bob/a/b/c/d/e/f"].Subtree()), 1)

	for _, path := range missingPaths {
		assert.Contains(t, ToPaths(dirs["/users/bob/a"].Subtree()), path)
	}
	assert.Len(t, ToPaths(dirs["/users/bob/a"].Subtree()), len(missingPaths)+len(dirPaths))
}
