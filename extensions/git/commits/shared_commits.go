package commits

import (
	"github.com/samber/lo"
	"strings"
)

// GetCommitsInCommon Puts components or files into pairs and counts the number of commits that they share.
// Returns a map of the pair to the number of shared commits.
//
// The structure of the map is: file1:file2 -> []string{CommitHashes...}
// The resulting map will contain pairs of components or files that have at least 1 shared commit.
func GetCommitsInCommon(filesOrComponents []string, componentOrFileToCommits map[string]CommitHashes) map[string]CommitHashes {
	pairs := uniquePairs(filesOrComponents)

	toReturn := map[string]CommitHashes{}

	seen := map[string]bool{}
	for _, pair := range pairs {

		key := strings.Join(pair, ":")

		if _, ok := seen[key]; ok {
			continue
		}

		seen[key] = true

		set1 := componentOrFileToCommits[pair[0]]
		set2 := componentOrFileToCommits[pair[1]]

		shared := lo.Intersect(set1, set2)

		if len(shared) > 0 {
			toReturn[key] = shared
		}
	}
	return toReturn
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
