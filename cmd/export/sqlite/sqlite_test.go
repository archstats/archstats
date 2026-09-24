package sqlite

import (
	"database/sql"
	"fmt"
	"github.com/archstats/archstats/core"
	"github.com/stretchr/testify/assert"
	"path/filepath"
	"strings"
	"testing"
	"time"
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

// A snapshot says which analysis wrote it, for which report and when, and
// every key it writes is a documented one.
func TestSnapshotRecordsItsIdentity(t *testing.T) {
	path := filepath.Join(t.TempDir(), "snap.db")
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	results := &core.Results{}
	results.SetSnapshotInfo("git_head_commit", "3f2a91c0d1e2f3a4b5c6d7e8f9a0b1c2d3e4f5a6")
	opts := &SqlOptions{ReportId: "shop", ScanTime: time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC)}
	if err := saveSnapshotInfo(db, results, opts); err != nil {
		t.Fatal(err)
	}
	// Writing twice (a re-export into the same file) keeps one row per key.
	if err := saveSnapshotInfo(db, results, opts); err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	rows, _ := db.Query(`SELECT key, value FROM _snapshot WHERE report_id = 'shop'`)
	for rows.Next() {
		var k, v string
		rows.Scan(&k, &v)
		got[k] = v
	}
	rows.Close()
	if got["analysis_revision"] != fmt.Sprint(core.AnalysisRevision) || got["report_id"] != "shop" ||
		got["scanned_at"] != "2026-09-24T10:00:00Z" || got["git_head_commit"] == "" {
		t.Fatalf("got %v", got)
	}
	known := map[string]bool{}
	for _, k := range core.KnownSnapshotKeys {
		known[k] = true
	}
	for k := range got {
		if !known[k] {
			t.Errorf("_snapshot key %q is written but not in core.KnownSnapshotKeys", k)
		}
	}
}

// A second report appended to the same file keeps its own identity, and a
// _snapshot written before report_id existed is upgraded, not duplicated.
func TestSnapshotIsKeyedByReport(t *testing.T) {
	path := filepath.Join(t.TempDir(), "snap.db")
	db, _ := sql.Open("sqlite3", path)
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE _snapshot (key TEXT PRIMARY KEY, value TEXT); INSERT INTO _snapshot VALUES ('analysis_revision', '1');`); err != nil {
		t.Fatal(err)
	}
	for _, report := range []string{"a", "b"} {
		if err := saveSnapshotInfo(db, &core.Results{}, &SqlOptions{ReportId: report}); err != nil {
			t.Fatal(err)
		}
	}
	var legacy, reports int
	db.QueryRow(`SELECT count(*) FROM _snapshot WHERE report_id = '' AND key = 'analysis_revision'`).Scan(&legacy)
	db.QueryRow(`SELECT count(DISTINCT report_id) FROM _snapshot WHERE key = 'report_id'`).Scan(&reports)
	if legacy != 1 || reports != 2 {
		t.Fatalf("legacy rows %d, reports %d", legacy, reports)
	}
}
