package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Directories are named with forward slashes on every OS, like the files in
// them. On Windows they were named svc\store, which no git history, file or
// component matched. Only a Windows run can fail this.
func TestDirectoriesAreNamedWithForwardSlashes(t *testing.T) {
	got := mapDirectoryToFiles([]string{"svc/store/store.go", "svc/main.go", "go.mod"})
	assert.Equal(t, map[string][]string{
		"svc/store": {"svc/store/store.go"},
		"svc":       {"svc/store/store.go", "svc/main.go"},
	}, got)
}
