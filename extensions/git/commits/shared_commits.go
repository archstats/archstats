package commits

import (
	"github.com/samber/lo"
	"slices"
)

// Pair is two components or files, A sorting before B.
type Pair struct{ A, B string }

// PairsToCommitsInCommon returns, for every pair of the given components or
// files that changed in the same commit, the commits they share. Pairs that
// share none are absent.
//
// It walks each commit's members instead of every pair: 6,453 directories are
// 20.8 million pairs, and building them all held 12 GB on Sakai, though only
// the pairs that co-changed can share anything.
func PairsToCommitsInCommon(filesOrComponents []string, componentOrFileToCommits map[string]CommitHashes) map[Pair]CommitHashes {
	members := slices.Clone(filesOrComponents)
	slices.Sort(members)
	members = slices.Compact(members)

	toReturn := map[Pair]CommitHashes{}
	// The members seen so far in each commit, in sorted order, so a pair's
	// commits come out in the order its second member lists them.
	touched := map[string][]string{}
	for _, b := range members {
		for _, commit := range componentOrFileToCommits[b] {
			earlier := touched[commit]
			if len(earlier) > 0 && earlier[len(earlier)-1] == b {
				continue
			}
			for _, a := range earlier {
				pair := Pair{a, b}
				toReturn[pair] = append(toReturn[pair], commit)
			}
			touched[commit] = append(earlier, b)
		}
	}
	return toReturn
}

// SharedCommitsForGroup returns the commits every member of the group
// changed. A member with no commits shares none: the intersection used to
// start from nil and treat that member's empty list as "not started yet", so
// a pair whose first member had no commits reported all of the second's.
func SharedCommitsForGroup(group []string, componentOrFileToCommits map[string]CommitHashes) CommitHashes {
	if len(group) == 0 {
		return CommitHashes{}
	}
	intersection := componentOrFileToCommits[group[0]]
	for _, elem := range group[1:] {
		if len(intersection) == 0 {
			return CommitHashes{}
		}
		intersection = lo.Intersect(intersection, componentOrFileToCommits[elem])
	}
	if intersection == nil {
		return CommitHashes{}
	}
	return intersection
}
