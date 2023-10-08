package git

import (
	"github.com/samber/lo"
	"slices"
	"time"
)

type commitPartMap map[string][]*partOfCommit

func splitByCommit(commitParts []*partOfCommit) commitPartMap {
	return lo.GroupBy(commitParts, func(part *partOfCommit) string {
		return part.commit
	})
}

func splitByFile(commitParts []*partOfCommit) map[string][]*partOfCommit {
	return lo.GroupBy(commitParts, func(part *partOfCommit) string {
		return part.file
	})
}

func splitByComponent(commitParts []*partOfCommit) map[string][]*partOfCommit {
	return lo.GroupBy(commitParts, func(part *partOfCommit) string {
		return part.component
	})
}

func splitByAuthor(commitParts []*partOfCommit) map[string][]*partOfCommit {
	return lo.GroupBy(commitParts, func(part *partOfCommit) string {
		return part.author
	})
}

// Split commits into buckets based on the number of days between the commit and the time
// passed in.  The buckets are the number of days in the bucketDays array.
// For example, if bucketDays is [7, 30, 90], then the commits will be split into
// 7 days, 30 days, and 90 days. This is useful for seeing code churn over time.
// The buckets are returned as a map of days to commits.
// The time passed in is the time that the buckets are relative to.
// The commitParts are the commits to split into buckets, and they are assumed to be sorted
// by time in descending order.
func splitCommitsIntoBucketsOfDays(time time.Time, commitParts []*partOfCommit, bucketDays []int) map[int][]*partOfCommit {
	slices.Sort(bucketDays)

	buckets := map[int][]*partOfCommit{}

	for _, bucket := range bucketDays {
		buckets[bucket] = []*partOfCommit{}
	}

	cutoff := time.AddDate(0, 0, -bucketDays[len(bucketDays)-1])
	for _, part := range commitParts {
		if part.time.Before(cutoff) {
			break
		}
		for _, bucket := range bucketDays {

			diff := dayDiff(time, part.time)
			if diff <= bucket {
				buckets[bucket] = append(buckets[bucket], part)
			}
		}
	}
	return buckets
}
