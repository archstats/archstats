package unit

import (
	"math"
	"sort"
)

// Importance is how much of the codebase leans on a unit: what a reader
// should see first when there is room for only a few hundred names.
//
// It is PageRank over the unit graph with members rolled up into the unit
// that owns them, so a class is ranked for everything its methods are used
// for and a method is not ranked against its own class. The iteration runs
// over sorted ids in a fixed order, so the same codebase ranks the same way
// every time; gonum's component PageRank does not, and a map that reshuffles
// between two scans of one commit is worse than none.
type Importance struct {
	// Rank is PageRank scaled so the average top-level unit is 1: 4 means
	// four times the average. Zero for a member, which its owner speaks for.
	Rank float64
	// UsedBy is how many units use this one: top-level units for a top-level
	// unit, any unit for a member.
	UsedBy int
	// UsedByComponents is how many other components hold a unit that uses it.
	UsedByComponents int
}

// Rank computes every unit's importance from its connections. Only the
// units counted take part: a test helper used by four hundred tests is not
// what the system leans on, and a test using everything it tests would rank
// the system by how well it is tested. The rest are ranked zero and their
// uses are not counted.
func Rank(units []*Unit, connections []*Connection, counted func(*Unit) bool) map[string]Importance {
	byID := make(map[string]*Unit, len(units))
	for _, u := range units {
		if counted == nil || counted(u) {
			byID[u.ID] = u
		}
	}
	top := func(id string) string {
		for i := 0; i < 16; i++ {
			u := byID[id]
			if u == nil || u.Owner == "" || byID[u.Owner] == nil {
				return id
			}
			id = u.Owner
		}
		return id
	}

	var ids []string
	for _, u := range units {
		if byID[u.ID] != nil && top(u.ID) == u.ID {
			ids = append(ids, u.ID)
		}
	}
	sort.Strings(ids)
	index := make(map[string]int, len(ids))
	for i, id := range ids {
		index[id] = i
	}

	out := make([]map[int]bool, len(ids))
	in := make([]map[int]bool, len(ids))
	rawUsers := map[string]map[string]bool{}
	componentsUsing := map[string]map[string]bool{}
	for _, c := range connections {
		if byID[c.From] == nil || byID[c.To] == nil {
			continue
		}
		if rawUsers[c.To] == nil {
			rawUsers[c.To] = map[string]bool{}
		}
		rawUsers[c.To][c.From] = true
		from, to := top(c.From), top(c.To)
		if from == to {
			continue
		}
		f, okF := index[from]
		t, okT := index[to]
		if !okF || !okT {
			continue
		}
		if out[f] == nil {
			out[f] = map[int]bool{}
		}
		out[f][t] = true
		if in[t] == nil {
			in[t] = map[int]bool{}
		}
		in[t][f] = true
		if fu, tu := byID[from], byID[to]; fu != nil && tu != nil && fu.Component != tu.Component && fu.Component != "" {
			if componentsUsing[to] == nil {
				componentsUsing[to] = map[string]bool{}
			}
			componentsUsing[to][fu.Component] = true
		}
	}

	n := len(ids)
	rank := pagerank(n, out, in)
	result := make(map[string]Importance, len(units))
	for _, u := range units {
		imp := Importance{UsedBy: len(rawUsers[u.ID])}
		if i, ok := index[u.ID]; ok {
			imp.Rank = math.Round(rank[i]*float64(n)*10000) / 10000
			imp.UsedBy = len(in[i])
			imp.UsedByComponents = len(componentsUsing[u.ID])
		}
		result[u.ID] = imp
	}
	return result
}

// pagerank is the stationary distribution of a random walk that follows a
// use with probability 0.85 and jumps anywhere otherwise; a unit that uses
// nothing jumps anywhere.
func pagerank(n int, out, in []map[int]bool) []float64 {
	if n == 0 {
		return nil
	}
	const damping = 0.85
	rank := make([]float64, n)
	next := make([]float64, n)
	for i := range rank {
		rank[i] = 1 / float64(n)
	}
	sortedIn := make([][]int, n)
	for i, set := range in {
		for j := range set {
			sortedIn[i] = append(sortedIn[i], j)
		}
		sort.Ints(sortedIn[i])
	}
	for iter := 0; iter < 100; iter++ {
		dangling := 0.0
		for i := 0; i < n; i++ {
			if len(out[i]) == 0 {
				dangling += rank[i]
			}
		}
		base := (1-damping)/float64(n) + damping*dangling/float64(n)
		delta := 0.0
		for i := 0; i < n; i++ {
			sum := 0.0
			for _, j := range sortedIn[i] {
				sum += rank[j] / float64(len(out[j]))
			}
			next[i] = base + damping*sum
			delta += math.Abs(next[i] - rank[i])
		}
		rank, next = next, rank
		if delta < 1e-12 {
			break
		}
	}
	return rank
}
