package sqlite

import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/archstats/archstats/core"
	"github.com/stretchr/testify/assert"
)

func commitsView() *core.View {
	row := func(file, hash, message, path string, additions int) *core.Row {
		return &core.Row{Data: map[string]interface{}{
			"file": file, "component": "shop", "repository": ".", "commit_hash": hash,
			"commit_time": time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC), "author_name": "Ada", "author_email": "ada@example.com",
			"commit_message": message, "file_additions": additions, "file_deletions": 0, "path_at_commit": path, "change_kind": "modified",
		}}
	}
	return &core.View{
		Name: "git_commits",
		Columns: []*core.Column{
			core.StringColumn("file"), core.StringColumn("component"), core.StringColumn("repository"), core.StringColumn("commit_hash"),
			core.DateColumn("commit_time"), core.StringColumn("author_name"), core.StringColumn("author_email"), core.StringColumn("commit_message"),
			core.IntColumn("file_additions"), core.IntColumn("file_deletions"), core.StringColumn("path_at_commit"), core.StringColumn("change_kind"),
		},
		Rows: []*core.Row{
			row("a.go", "c1", "first", "a.go", 3),
			row("b.go", "c1", "first", "old/b.go", 1),
			row("a.go", "c2", "it's second", "a.go", 2),
		},
	}
}

func export(t *testing.T, path, report string, at time.Time, views ...*core.View) {
	t.Helper()
	if err := SaveToDB(&SqlOptions{DatabaseName: path, ReportId: report, ScanTime: at}, &core.Results{}, views); err != nil {
		t.Fatal(err)
	}
}

func rowsOf(t *testing.T, db *sql.DB, query string) [][]interface{} {
	t.Helper()
	rows, err := db.Query(query)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	columns, _ := rows.Columns()
	var out [][]interface{}
	for rows.Next() {
		values := make([]interface{}, len(columns))
		pointers := make([]interface{}, len(columns))
		for i := range values {
			pointers[i] = &values[i]
		}
		if err := rows.Scan(pointers...); err != nil {
			t.Fatal(err)
		}
		out = append(out, values)
	}
	return out
}

// A table the export creates stores report_id and timestamp once, as column
// defaults, and every row still reads them back as the report's own.
func TestReportColumnsAreDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "snap.db")
	at := time.Date(2026, 9, 25, 8, 30, 0, 123000000, time.UTC)
	export(t, path, "shop's", at, &core.View{
		Name:    "files",
		Columns: []*core.Column{core.StringColumn("name")},
		Rows:    []*core.Row{{Data: map[string]interface{}{"name": "a.go"}}, {Data: map[string]interface{}{"name": "b.go"}}},
	})
	db, _ := sql.Open("sqlite3", path)
	defer db.Close()

	var report string
	var stamp time.Time
	if err := db.QueryRow(`SELECT report_id, timestamp FROM files WHERE name = 'b.go'`).Scan(&report, &stamp); err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, "shop's", report)
	assert.True(t, stamp.Equal(at), "timestamp %v, want %v", stamp, at)

	// Nothing per row: the table's own columns hold only name.
	var ddl string
	db.QueryRow(`SELECT sql FROM sqlite_master WHERE name = 'files'`).Scan(&ddl)
	assert.Contains(t, ddl, "DEFAULT 'shop''s'")
}

// A second report appended to the file writes its values explicitly; both
// read back as themselves, and re-exporting one replaces only its rows.
func TestSecondReportKeepsItsOwnColumns(t *testing.T) {
	path := filepath.Join(t.TempDir(), "snap.db")
	view := func(name string) *core.View {
		return &core.View{Name: "files", Columns: []*core.Column{core.StringColumn("name")}, Rows: []*core.Row{{Data: map[string]interface{}{"name": name}}}}
	}
	export(t, path, "a", time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), view("a.go"))
	export(t, path, "b", time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC), view("b.go"))
	export(t, path, "a", time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC), view("c.go"))

	db, _ := sql.Open("sqlite3", path)
	defer db.Close()
	got := rowsOf(t, db, `SELECT report_id, name, strftime('%m', timestamp) FROM files ORDER BY report_id, name`)
	assert.Equal(t, [][]interface{}{{"a", "c.go", "03"}, {"b", "b.go", "02"}}, got)
}

// git_commits is two tables behind a view that reads as the flat table did.
func TestCommitsReadAsOneTable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "snap.db")
	export(t, path, "shop", time.Date(2026, 9, 25, 0, 0, 0, 0, time.UTC), commitsView())
	db, _ := sql.Open("sqlite3", path)
	defer db.Close()

	var kind string
	db.QueryRow(`SELECT type FROM sqlite_master WHERE name = 'git_commits'`).Scan(&kind)
	assert.Equal(t, "view", kind)

	var commits int
	db.QueryRow(`SELECT count(*) FROM git_commit_info`).Scan(&commits)
	assert.Equal(t, 2, commits)

	got := rowsOf(t, db, `SELECT file, commit_hash, commit_message, path_at_commit, file_additions, report_id FROM git_commits ORDER BY commit_hash, file`)
	assert.Equal(t, [][]interface{}{
		{"a.go", "c1", "first", "a.go", int64(3), "shop"},
		{"b.go", "c1", "first", "old/b.go", int64(1), "shop"},
		{"a.go", "c2", "it's second", "a.go", int64(2), "shop"},
	}, got)

	var columns []string
	for _, r := range rowsOf(t, db, `SELECT name FROM pragma_table_info('git_commits')`) {
		columns = append(columns, r[0].(string))
	}
	assert.Equal(t, []string{"file", "component", "repository", "commit_hash", "commit_time", "author_name", "author_email",
		"commit_message", "file_additions", "file_deletions", "path_at_commit", "change_kind", "report_id", "timestamp"}, columns)

	// The driver still reads commit_time as a time through the view.
	var when time.Time
	if err := db.QueryRow(`SELECT commit_time FROM git_commits LIMIT 1`).Scan(&when); err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, 2026, when.Year())

	// A second report lands beside the first and reads as its own.
	export(t, path, "other", time.Date(2026, 9, 26, 0, 0, 0, 0, time.UTC), commitsView())
	var perReport int
	db.QueryRow(`SELECT count(*) FROM git_commits WHERE report_id = 'other'`).Scan(&perReport)
	assert.Equal(t, 3, perReport)
}

// A file written before the split keeps git_commits as a table.
func TestCommitsTableKeepsItsLayout(t *testing.T) {
	path := filepath.Join(t.TempDir(), "snap.db")
	db, _ := sql.Open("sqlite3", path)
	defer db.Close()
	if _, err := db.Exec("CREATE TABLE git_commits (file TEXT, component TEXT, repository TEXT, commit_hash TEXT, commit_time DATE, author_name TEXT, author_email TEXT, commit_message TEXT, file_additions INTEGER, file_deletions INTEGER, path_at_commit TEXT, change_kind TEXT, report_id TEXT, timestamp DATE)"); err != nil {
		t.Fatal(err)
	}
	export(t, path, "shop", time.Now(), commitsView())
	var kind string
	var n int
	db.QueryRow(`SELECT type FROM sqlite_master WHERE name = 'git_commits'`).Scan(&kind)
	db.QueryRow(`SELECT count(*) FROM git_commits WHERE report_id = 'shop'`).Scan(&n)
	assert.Equal(t, "table", kind)
	assert.Equal(t, 3, n)
}
