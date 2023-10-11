package commits

import (
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestSplitBuckets(t *testing.T) {
	commits := []*PartOfCommit{
		{
			Commit: "30",
			Time:   time.Date(2000, 5, 15, 0, 0, 0, 0, time.UTC),
		},

		{
			Commit: "30",
			Time:   time.Date(2000, 5, 14, 0, 0, 0, 0, time.UTC),
		},
		{
			Commit: "90",
			Time:   time.Date(2000, 3, 12, 0, 0, 0, 0, time.UTC),
		},
		{
			Commit: "180",
			Time:   time.Date(2000, 1, 11, 0, 0, 0, 0, time.UTC),
		},
		{
			Commit: "210",
			Time:   time.Date(1999, 11, 4, 0, 0, 0, 0, time.UTC),
		},
	}

	buckets :=
		SplitCommitsIntoBucketsOfDays(time.Date(2000, 6, 1, 0, 0, 0, 0, time.UTC), commits,
			[]int{180, 90, 30})

	assert.Len(t, buckets, 3)
	assert.Len(t, buckets[180], 4)
	assert.Len(t, buckets[90], 3)
	assert.Len(t, buckets[30], 2)
}
