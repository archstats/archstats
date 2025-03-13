package commits

import (
	"github.com/samber/lo"
	"golang.org/x/exp/slices"
	"strings"
)

// PairsToCommitsInCommon Puts components or files into pairs and counts the number of commits that they share.
// Returns a map of the pair to the number of shared commits.
//
// The structure of the map is: file1:file2 -> []string{CommitHashes...}
// The resulting map will contain pairs of components or files that have at least 1 shared commit.
func PairsToCommitsInCommon(filesOrComponents []string, componentOrFileToCommits map[string]CommitHashes) map[string]CommitHashes {
	pairs := uniquePairs(filesOrComponents)

	toReturn := map[string]CommitHashes{}

	seen := map[string]bool{}
	for _, pair := range pairs {
		slices.Sort(pair)
		key := strings.Join(pair, ":")

		if _, ok := seen[key]; ok {
			continue
		}

		seen[key] = true
		shared := SharedCommitsForGroup(pair, componentOrFileToCommits)

		if len(shared) > 0 {
			toReturn[key] = shared
		}
	}
	return toReturn
}

func SharedCommitsForGroup(group []string, componentOrFileToCommits map[string]CommitHashes) CommitHashes {
	var intersection CommitHashes
	for _, elem := range group {
		commits := componentOrFileToCommits[elem]
		if intersection == nil {
			intersection = commits
		} else {
			intersection = lo.Intersect(intersection, commits)
		}
	}

	return intersection
}

func uniquePairs(elems []string) [][]string {
	var pairs [][]string
	for i, elem1 := range elems {
		for _, elem2 := range elems[i+1:] {
			pairs = append(pairs, []string{elem1, elem2})
		}
	}
	return pairs
}
