package git

import (
	"github.com/archstats/archstats/core"
	"github.com/samber/lo"
	"slices"
	"strings"
)

func sharedCommitColumns(dayBuckets []int) []*core.Column {
	columns := []*core.Column{
		core.StringColumn("from"),
		core.StringColumn("to"),
		core.IntColumn("shared_commit_count"),
	}

	for _, days := range dayBuckets {
		columns = append(columns, core.IntColumn(toDayStat("shared_commit_count", days)))
	}
	return columns
}

// Takes total shared commit counts and shared commit counts per day bucket and returns rows.
func sharedCommitsToRows(componentsOrFiles []string, totals map[string]int, totalsPerDayBucket map[int]map[string]int) []*core.Row {
	var rows []*core.Row
	for _, component1 := range componentsOrFiles {
		for _, component2 := range componentsOrFiles {
			if component1 == component2 {
				continue
			}

			combined := []string{component1, component2}
			slices.Sort(combined)
			key := strings.Join(combined, ":")

			row1 := &core.Row{
				Data: map[string]interface{}{
					"from": component1,
					"to":   component2,
				},
			}
			row2 := &core.Row{
				Data: map[string]interface{}{
					"from": component2,
					"to":   component1,
				},
			}

			if total, hasTotal := totals[key]; hasTotal {
				row1.Data["shared_commit_count"] = total
				row2.Data["shared_commit_count"] = total
			}

			for days, sharedCommitCount := range totalsPerDayBucket {
				if count, hasCount := sharedCommitCount[key]; hasCount {
					row1.Data[toDayStat("shared_commit_count", days)] = count
					row2.Data[toDayStat("shared_commit_count", days)] = count
				}
			}

			rows = append(rows, row1, row2)
		}
	}
	return rows
}

// Puts components or files into pairs and counts the number of commits that they share.
// Returns a map of the pair to the number of shared commits.
// from the commit.
func getSharedCommitPairsFor(filesOrComponents []string, commits []*partOfCommit, doFiles bool) map[string]int {
	commitPartMap := splitByCommit(commits)
	commitToComponents := lo.MapValues(commitPartMapToCommitContentMap(commitPartMap), func(content *commitFileComponentContent, _ string) []string {
		if doFiles {
			return content.files
		}
		return content.components
	})

	groups := cartesianProduct(filesOrComponents)

	cache := map[string]int{}
	for _, group := range groups {
		slices.Sort(group)
		key := strings.Join(group, ":")
		if _, hasKey := cache[key]; !hasKey {
			cache[key] = getSharedCommitCount(group, commitToComponents)
		}
	}
	return cache
}

func cartesianProduct(elems []string) [][]string {
	var groups [][]string
	for _, groupMember1 := range elems {
		for _, groupMember2 := range elems {
			if groupMember1 == groupMember2 {
				continue
			}
			groups = append(groups, []string{groupMember1, groupMember2})
		}
	}
	return groups
}

func commitPartMapToCommitContentMap(hashMap commitPartMap) map[string]*commitFileComponentContent {
	return lo.MapValues(hashMap, func(parts []*partOfCommit, commit string) *commitFileComponentContent {
		allFiles := lo.Map(parts, func(part *partOfCommit, _ int) string {
			return part.file
		})
		allComponents := lo.Map(parts, func(part *partOfCommit, _ int) string {
			return part.component
		})

		return &commitFileComponentContent{
			files:      lo.Uniq(allFiles),
			components: lo.Uniq(allComponents),
		}
	})
}

type commitFileComponentContent struct {
	files      []string
	components []string
}

func getSharedCommitCount(components []string, commitToIncludedComponents map[string][]string) int {
	count := 0
	for _, includedComponents := range commitToIncludedComponents {
		if lo.Every(includedComponents, components) {
			count++
		}
	}
	return count
}
