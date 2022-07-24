package views

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestMergeStats(t *testing.T) {
	assert.Equal(t, Stats{"a": 2, "b": 4, "c": 3}, Stats{"a": 1, "b": 2, "c": 3}.Merge(Stats{"a": 1, "b": 2}))
}
