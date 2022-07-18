package walker

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestWalker(t *testing.T) {
	allFiles := GetAllFiles("./test_example")

	assert.Len(t, allFiles, 3)
	for _, file := range allFiles {
		assert.NotContains(t, file.Path, "ignore")
	}
}
