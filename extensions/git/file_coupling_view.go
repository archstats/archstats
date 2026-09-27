package git

import (
	"sort"

	"github.com/archstats/archstats/core"
	"github.com/archstats/archstats/extensions/git/commits"
)

// File-grain co-change, floored. Every pair of files that ever changed
// together would be millions of rows on a large repository, nearly all of
// them one shared commit; a pair is kept when the two share at least
// FileCouplingMinShared commits and that is at least FileCouplingMinRatio of
// the smaller side's commits. One row per unordered pair, file_1 < file_2.
const (
	FileCouplingMinShared = 3
	FileCouplingMinRatio  = 0.10

	File1                       = "file_1"
	File2                       = "file_2"
	PercentageOfAllCommitsFile1 = "percentage_of_all_commits_file_1"
	PercentageOfAllCommitsFile2 = "percentage_of_all_commits_file_2"
)

func (e *extension) fileCouplingViewFactory(results *core.Results) *core.View {
	rows := fileCouplingRows(e.couplingParts, coChangeCandidates(results), FileCouplingMinShared, FileCouplingMinRatio)
	return &core.View{
		Columns: []*core.Column{
			core.StringColumn(File1),
			core.StringColumn(File2),
			core.IntColumn(SharedCommitCount),
			core.FloatColumn(PercentageOfAllCommitsFile1),
			core.FloatColumn(PercentageOfAllCommitsFile2),
		},
		Rows: rows,
	}
}

// coChangeCandidates is the files whose co-change is counted: those in a
// component, and those that describe building, shipping or running the
// software. The second half is why this exists. Build files, pipelines and
// deployment descriptors belong to no component, so they never appeared in a
// single pair: Fineract's fineract-provider/build.gradle is in 563 commits and
// was paired with nothing. Other files with no component (documentation,
// images) are still left out; their pairs say nothing about the design.
func coChangeCandidates(results *core.Results) map[string]bool {
	in := make(map[string]bool, len(results.FileToComponent)+len(results.FileSystemKinds))
	for f := range results.FileToComponent {
		in[f] = true
	}
	for f := range results.FileSystemKinds {
		if !results.ThirdPartyFiles[f] && !results.GeneratedFiles[f] {
			in[f] = true
		}
	}
	return in
}

type filePair struct{ a, b string }

// fileCouplingRows counts, per commit, every pair of snapshot files it
// touched, then keeps the pairs above both floors.
func fileCouplingRows(parts []*commits.PartOfCommit, inSnapshot map[string]bool, minShared int, minRatio float64) []*core.Row {
	filesOf := map[string]map[string]bool{}
	for _, p := range parts {
		if !inSnapshot[p.File] {
			continue
		}
		set := filesOf[p.Commit]
		if set == nil {
			set = map[string]bool{}
			filesOf[p.Commit] = set
		}
		set[p.File] = true
	}
	commitsOf := map[string]int{}
	shared := map[filePair]int{}
	for _, set := range filesOf {
		files := make([]string, 0, len(set))
		for f := range set {
			files = append(files, f)
			commitsOf[f]++
		}
		sort.Strings(files)
		for i := 0; i < len(files); i++ {
			for j := i + 1; j < len(files); j++ {
				shared[filePair{files[i], files[j]}]++
			}
		}
	}
	pairs := make([]filePair, 0)
	for p, n := range shared {
		smaller := commitsOf[p.a]
		if commitsOf[p.b] < smaller {
			smaller = commitsOf[p.b]
		}
		if n >= minShared && smaller > 0 && float64(n)/float64(smaller) >= minRatio {
			pairs = append(pairs, p)
		}
	}
	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i].a != pairs[j].a {
			return pairs[i].a < pairs[j].a
		}
		return pairs[i].b < pairs[j].b
	})
	rows := make([]*core.Row, 0, len(pairs))
	for _, p := range pairs {
		n := shared[p]
		rows = append(rows, &core.Row{Data: core.RowData{
			File1:                       p.a,
			File2:                       p.b,
			SharedCommitCount:           n,
			PercentageOfAllCommitsFile1: 100 * float64(n) / float64(commitsOf[p.a]),
			PercentageOfAllCommitsFile2: 100 * float64(n) / float64(commitsOf[p.b]),
		}})
	}
	return rows
}
