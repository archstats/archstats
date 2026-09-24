package git

import (
	"testing"

	"github.com/archstats/archstats/extensions/git/commits"
)

func TestFileCouplingRowsFloorsAndOrders(t *testing.T) {
	var parts []*commits.PartOfCommit
	add := func(commit string, files ...string) {
		for _, f := range files {
			parts = append(parts, &commits.PartOfCommit{Commit: commit, File: f})
		}
	}
	for i := 0; i < 4; i++ {
		add(string(rune('a'+i)), "b.go", "a.go")
	}
	add("x", "a.go", "gone.go")
	add("y", "a.go", "c.go")
	rows := fileCouplingRows(parts, map[string]bool{"a.go": true, "b.go": true, "c.go": true}, 3, 0.10)
	if len(rows) != 1 {
		t.Fatalf("rows = %d, want only a.go/b.go (c.go shares one commit; gone.go is not in the snapshot)", len(rows))
	}
	r := rows[0].Data
	if r[File1] != "a.go" || r[File2] != "b.go" || r[SharedCommitCount] != 4 {
		t.Errorf("row = %v", r)
	}
	// a.go has 6 commits (4 + gone.go + c.go), b.go has 4.
	if r[PercentageOfAllCommitsFile1].(float64) < 66 || r[PercentageOfAllCommitsFile2].(float64) != 100 {
		t.Errorf("percentages = %v / %v", r[PercentageOfAllCommitsFile1], r[PercentageOfAllCommitsFile2])
	}
}
