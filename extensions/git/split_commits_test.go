package git

import (
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestSplitBuckets(t *testing.T) {
	commits := []*partOfCommit{
		{
			commit: "30",
			time:   time.Date(2000, 5, 15, 0, 0, 0, 0, time.UTC),
		},

		{
			commit: "30",
			time:   time.Date(2000, 5, 14, 0, 0, 0, 0, time.UTC),
		},
		{
			commit: "90",
			time:   time.Date(2000, 3, 12, 0, 0, 0, 0, time.UTC),
		},
		{
			commit: "180",
			time:   time.Date(2000, 1, 11, 0, 0, 0, 0, time.UTC),
		},
		{
			commit: "210",
			time:   time.Date(1999, 11, 4, 0, 0, 0, 0, time.UTC),
		},
	}

	buckets :=
		splitCommitsIntoBucketsOfDays(time.Date(2000, 6, 1, 0, 0, 0, 0, time.UTC), commits,
			[]int{180, 90, 30})

	assert.Len(t, buckets, 3)
	assert.Len(t, buckets[180], 4)
	assert.Len(t, buckets[90], 3)
	assert.Len(t, buckets[30], 2)
}
