package commits

import (
	"github.com/samber/lo"
	"slices"
	"strings"
)

// GetCommitsInCommonForFilePairs Puts components or files into pairs and counts the number of commits that they share.
// Returns a map of the pair to the number of shared commits.
// from the commit.
//
// The structure of the map is: file1:file2 -> []string{CommitHashes...}
// The resulting map will contain all pairs of components or files, only if they have no shared commits,
// in the resulting map, keys that have no shared commits are not included.
func GetCommitsInCommonForFilePairs(filesOrComponents []string, commitPartMap CommitPartMap) map[string]CommitHashes {
	commitToComponents := lo.MapValues(commitPartMapToCommitContentMap(commitPartMap), func(content *commitFileComponentContent, _ string) []string {
		return content.files
	})

	return getSharedCommitPairsFor(filesOrComponents, commitToComponents)
}

func GetCommitsInCommonForComponentPairs(filesOrComponents []string, commitPartMap CommitPartMap) map[string]CommitHashes {
	commitToComponents := lo.MapValues(commitPartMapToCommitContentMap(commitPartMap), func(content *commitFileComponentContent, _ string) []string {
		return content.components
	})

	return getSharedCommitPairsFor(filesOrComponents, commitToComponents)
}

func getSharedCommitPairsFor(filesOrComponents []string, commitToComponents map[string][]string) map[string]CommitHashes {
	groups := cartesianProduct(filesOrComponents)

	cache := map[string]CommitHashes{}
	for _, group := range groups {
		slices.Sort(group)
		key := strings.Join(group, ":")
		if _, hasKey := cache[key]; !hasKey {
			sharedCommits := getSharedCommits(group, commitToComponents)

			if len(sharedCommits) <= 0 {
				continue
			}
			cache[key] = sharedCommits
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

func commitPartMapToCommitContentMap(hashMap CommitPartMap) map[string]*commitFileComponentContent {
	return lo.MapValues(hashMap, func(parts []*PartOfCommit, commit string) *commitFileComponentContent {
		allFiles := lo.Map(parts, func(part *PartOfCommit, _ int) string {
			return part.File
		})
		allComponents := lo.Map(parts, func(part *PartOfCommit, _ int) string {
			return part.Component
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

func getSharedCommits(components []string, commitToIncludedComponents map[string][]string) []string {
	var commitsToReturn []string
	for commit, includedComponents := range commitToIncludedComponents {
		if lo.Every(includedComponents, components) {
			commitsToReturn = append(commitsToReturn, commit)
		}
	}
	return commitsToReturn
}
