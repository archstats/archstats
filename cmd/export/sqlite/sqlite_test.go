package sqlite

import (
	"database/sql"
	"fmt"
	"github.com/archstats/archstats/core"
	"github.com/stretchr/testify/assert"
	"path/filepath"
	"strings"
	"testing"
)

func TestIncludedExcluded(t *testing.T) {

}

func TestViewInclusion(t *testing.T) {
	tests := []struct {
		name     string
		possible string
		excluded string
		included string

		shouldError  bool
		shouldReturn []string
	}{

		{"no flags", "a,b,c", "", "", false, []string{"a", "b", "c"}},

		{"only excluded", "a,b,c", "b", "", false, []string{"a", "c"}},
		{"only excluded multiple", "a,b,c", "b,c", "", false, []string{"a"}},

		{"only included", "a,b,c", "", "b", false, []string{"b"}},
		{"only included multiple", "a,b,c", "", "b,c", false, []string{"b", "c"}},

		{"included and excluded", "a,b,c", "b", "c", false, []string{"c"}},

		{"invalid included", "a,b,c", "", "d", true, nil},
		{"invalid excluded", "a,b,c", "d", "", true, nil},
		{"invalid included and excluded", "a,b,c", "d", "e", true, nil},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {

			possibleSlice := strings.Split(test.possible, ",")
			excludedSlice := strings.Split(test.excluded, ",")
			includedSlice := strings.Split(test.included, ",")

			show, err := getViewsToShow(includedSlice, excludedSlice, possibleSlice)

			if test.shouldError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.ElementsMatch(t, test.shouldReturn, show)
			}

		})
	}
}

// A snapshot says which analysis wrote it, so the desktop can tell a scan
// taken before a fix from one taken after.
func TestSnapshotRecordsAnalysisRevision(t *testing.T) {
	path := filepath.Join(t.TempDir(), "snap.db")
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := saveSnapshotInfo(db); err != nil {
		t.Fatal(err)
	}
	// Writing twice (a re-export into the same file) keeps one row.
	if err := saveSnapshotInfo(db); err != nil {
		t.Fatal(err)
	}
	var value string
	var rows int
	if err := db.QueryRow(`SELECT value, (SELECT count(*) FROM _snapshot) FROM _snapshot WHERE key = 'analysis_revision'`).Scan(&value, &rows); err != nil {
		t.Fatal(err)
	}
	if value != fmt.Sprint(core.AnalysisRevision) || rows != 1 {
		t.Fatalf("got revision %q in %d rows, want %d in 1", value, rows, core.AnalysisRevision)
	}
}
