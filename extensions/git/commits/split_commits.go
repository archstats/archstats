package commits

import (
	"github.com/samber/lo"
	"slices"
	"time"
)

type CommitPartMap map[string][]*PartOfCommit

func Split(basedOn time.Time, dayBuckets []int, commitParts []*PartOfCommit) *Splitted {

	splittedByDayBucket := SplitCommitsIntoBucketsOfDays(basedOn, commitParts, dayBuckets)

	return &Splitted{
		commitParts: commitParts,
		dayBuckets: lo.MapValues(splittedByDayBucket, func(parts []*PartOfCommit, _ int) *Splitted {
			return &Splitted{
				commitParts: parts,
			}
		}),
	}
}

type Splitted struct {
	commitParts []*PartOfCommit

	commitPartsByFile      CommitPartMap
	commitPartsByDirectory CommitPartMap
	commitPartsByComponent CommitPartMap
	commitPartsByCommit    CommitPartMap
	commitPartsByAuthor    CommitPartMap

	fileToCommitHashes      map[string]CommitHashes
	directoryToCommitHashes map[string]CommitHashes
	componentToCommitHashes map[string]CommitHashes
	dayBuckets              map[int]*Splitted
}

func (ms *Splitted) CommitParts() []*PartOfCommit {
	return ms.commitParts
}

func (ms *Splitted) SplitByDirectory() CommitPartMap {
	if ms.commitPartsByDirectory == nil {
		ms.splitAll()
	}
	return ms.commitPartsByDirectory
}

func (ms *Splitted) SplitByFile() CommitPartMap {
	if ms.commitPartsByFile == nil {
		ms.splitAll()
	}
	return ms.commitPartsByFile
}

func (ms *Splitted) SplitByComponent() CommitPartMap {
	if ms.commitPartsByComponent == nil {
		ms.splitAll()
	}
	return ms.commitPartsByComponent
}

func (ms *Splitted) SplitByCommitHash() CommitPartMap {
	if ms.commitPartsByCommit == nil {
		ms.splitAll()
	}
	return ms.commitPartsByCommit
}

func (ms *Splitted) SplitByAuthor() CommitPartMap {
	if ms.commitPartsByAuthor == nil {
		ms.splitAll()
	}
	return ms.commitPartsByAuthor
}

func (ms *Splitted) FileToCommitHashes() map[string]CommitHashes {
	if ms.fileToCommitHashes == nil {
		ms.splitAll()
	}
	return ms.fileToCommitHashes
}
func (ms *Splitted) DirectoryToCommitHashes() map[string]CommitHashes {
	if ms.directoryToCommitHashes == nil {
		ms.splitAll()
	}
	return ms.directoryToCommitHashes
}

func (ms *Splitted) ComponentToCommitHashes() map[string]CommitHashes {
	if ms.componentToCommitHashes == nil {
		ms.splitAll()
	}
	return ms.componentToCommitHashes
}

// DayBuckets may return nil if the commits were not split by day buckets.
// This is the case if this set of Splitted commits were already split into day buckets.
func (ms *Splitted) DayBuckets() map[int]*Splitted {
	return ms.dayBuckets
}

func (ms *Splitted) splitAll() {
	funcs := map[string]func(commit *PartOfCommit) string{
		"file": func(commit *PartOfCommit) string {
			return commit.File
		},
		"directory": func(commit *PartOfCommit) string {
			return commit.Directory
		},
		"component": func(commit *PartOfCommit) string {
			return commit.Component
		},
		"commit": func(commit *PartOfCommit) string {
			return commit.Commit
		},
		"author": func(commit *PartOfCommit) string {
			return commit.Author
		},
	}

	allGroups := multiGroupBy(ms.commitParts, funcs)

	ms.commitPartsByFile = allGroups["file"]
	ms.commitPartsByDirectory = allGroups["directory"]
	ms.commitPartsByComponent = allGroups["component"]
	ms.commitPartsByCommit = allGroups["commit"]
	ms.commitPartsByAuthor = allGroups["author"]

	ms.fileToCommitHashes = lo.MapValues(ms.commitPartsByFile, func(parts []*PartOfCommit, _ string) CommitHashes {
		return getUniqueHashes(parts)
	})
	ms.directoryToCommitHashes = lo.MapValues(ms.commitPartsByDirectory, func(parts []*PartOfCommit, _ string) CommitHashes {
		return getUniqueHashes(parts)
	})
	ms.componentToCommitHashes = lo.MapValues(ms.commitPartsByComponent, func(parts []*PartOfCommit, _ string) CommitHashes {
		return getUniqueHashes(parts)
	})
}

func getUniqueHashes(parts []*PartOfCommit) CommitHashes {
	return lo.Uniq(lo.Map(parts, func(part *PartOfCommit, _ int) string {
		return part.Commit
	}))
}

// SplitCommitsIntoBucketsOfDays commits into buckets based on the number of days between the commit and the time
// passed in.  The buckets are the number of days in the bucketDays array.
// For example, if bucketDays is [7, 30, 90], then the commits will be split into
// 7 days, 30 days, and 90 days. This is useful for seeing code churn over time.
// The buckets are returned as a map of days to commits.
// The time passed in is the time that the buckets are relative to.
// The commitParts are the commits to split into buckets, and they are assumed to be sorted
// by time in descending order.
func SplitCommitsIntoBucketsOfDays(time time.Time, commitParts []*PartOfCommit, bucketDays []int) map[int][]*PartOfCommit {
	slices.Sort(bucketDays)

	if len(bucketDays) == 0 {
		return map[int][]*PartOfCommit{}
	}

	buckets := map[int][]*PartOfCommit{}

	for _, bucket := range bucketDays {
		buckets[bucket] = []*PartOfCommit{}
	}

	cutoff := time.AddDate(0, 0, -bucketDays[len(bucketDays)-1])
	for _, part := range commitParts {
		if part.Time.Before(cutoff) {
			break
		}
		for _, bucket := range bucketDays {

			diff := dayDiff(time, part.Time)
			if diff <= bucket {
				buckets[bucket] = append(buckets[bucket], part)
			}
		}
	}
	return buckets
}

func multiGroupBy(snippets []*PartOfCommit, groupBys map[string]func(commit *PartOfCommit) string) map[string]CommitPartMap {
	toReturn := make(map[string]CommitPartMap)
	for s, _ := range groupBys {
		toReturn[s] = make(map[string][]*PartOfCommit)
	}
	for _, snippet := range snippets {
		for name, groupBy := range groupBys {
			group := groupBy(snippet)
			toReturn[name][group] = append(toReturn[name][group], snippet)
		}
	}
	return toReturn
}
