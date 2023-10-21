package commits

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestStats(t *testing.T) {
	var parts []*PartOfCommit
	authorPool := []string{"a", "b", "c", "d"}
	// Different days
	for day := 0; day < 20; day++ {
		for commit := 0; commit < 10; commit++ {
			commitHash := fmt.Sprintf("commit-%d-%d", day, commit)
			authorName := authorPool[commit%len(authorPool)]
			authorEmail := authorName + "@example.com"

			for component := 0; component < 4; component++ {
				componentName := fmt.Sprintf("component-%d", component)
				for part := 0; part < 5; part++ {
					file := fmt.Sprintf("%s-file-%d", componentName, part)
					partToAdd := &PartOfCommit{
						Component:   componentName,
						File:        file,
						Commit:      commitHash,
						Time:        daysBeforeJan12020(day),
						Author:      authorName,
						AuthorEmail: authorEmail,
						Message:     "",
						Additions:   3*part + 3,
						Deletions:   3*part + 2,
					}
					parts = append(parts, partToAdd)
				}
			}
		}
	}

	stats := GetStats(daysBeforeJan12020(0), parts)

	assert.Equal(t, 20*10, stats.CommitCount)
	assert.Equal(t, 36000, stats.AdditionCount)
	assert.Equal(t, 32000, stats.DeletionCount)
	assert.Equal(t, 4*5, stats.UniqueFileChangeCount)
	assert.Equal(t, 4, stats.UniqueComponentChangeCount)
	assert.Equal(t, 4, stats.UniqueAuthorCount)
	assert.Equal(t, 19, stats.OldestCommitAgeInDays)
}

func daysBeforeJan12020(dayOfYear int) time.Time {
	return time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC).AddDate(0, 0, -dayOfYear)
}
