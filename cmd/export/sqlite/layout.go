package sqlite

import (
	"database/sql"
	"strings"

	"github.com/archstats/archstats/core"
	"github.com/mattn/go-sqlite3"
	"github.com/samber/lo"
)

// How a snapshot stores what it says without changing what a query reads.
//
// Every table carries report_id and timestamp so several reports can share a
// file, but a snapshot is almost always one report, and writing the same two
// strings on every row was a tenth of the file. A table this export creates
// is written without them and gets them afterwards as columns with the
// report's values as defaults: SQLite reads a column added by ALTER TABLE from
// its default for every row written before it, and stores nothing per row. A
// later report appended to the same file writes its own values explicitly, so
// both still read back as themselves.
//
// git_commits repeated each commit's hash, time, author and message on every
// file it touched, nine rows a commit on a large repository. It is stored as
// git_commit_info (a commit) and git_commit_files (a file in a commit), with a
// view named git_commits that reads exactly as the table did.

const (
	gitCommits     = "git_commits"
	gitCommitInfo  = "git_commit_info"
	gitCommitFiles = "git_commit_files"
	pathAtCommit   = "path_at_commit"
)

// commitInfoColumns belong to the commit; the rest belong to a file in it.
// repository and commit_hash are on both sides: they are the join.
var commitInfoColumns = map[string]bool{"repository": true, "commit_hash": true, "commit_time": true, "author_name": true, "author_email": true, "commit_message": true}

// objectType is "table", "view" or "" for a name the file does not hold.
func objectType(db *sql.DB, name string) (string, error) {
	var kind string
	err := db.QueryRow(`SELECT type FROM sqlite_master WHERE name = ? AND type IN ('table', 'view')`, name).Scan(&kind)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return kind, err
}

// storageLayout swaps git_commits for its two tables, unless the file already
// holds git_commits as a table (written before the split), which keeps its
// layout so a second report lands beside the first.
func storageLayout(db *sql.DB, views []*core.View) ([]*core.View, *core.View, error) {
	out := make([]*core.View, 0, len(views)+1)
	var split *core.View
	for _, view := range views {
		if view.Name != gitCommits {
			out = append(out, view)
			continue
		}
		kind, err := objectType(db, gitCommits)
		if err != nil {
			return nil, nil, err
		}
		if kind == "table" {
			out = append(out, view)
			continue
		}
		info, files := splitCommits(view)
		out = append(out, info, files)
		split = view
	}
	return out, split, nil
}

func splitCommits(view *core.View) (*core.View, *core.View) {
	info := &core.View{Name: gitCommitInfo}
	files := &core.View{Name: gitCommitFiles}
	for _, column := range view.Columns {
		if commitInfoColumns[column.Name] {
			info.Columns = append(info.Columns, column)
		}
		if !commitInfoColumns[column.Name] || column.Name == "repository" || column.Name == "commit_hash" {
			files.Columns = append(files.Columns, column)
		}
	}

	seen := map[string]bool{}
	for _, row := range view.Rows {
		key := toString(row.Data["repository"]) + "\x00" + toString(row.Data["commit_hash"])
		if !seen[key] {
			seen[key] = true
			info.Rows = append(info.Rows, &core.Row{Data: lo.PickBy(row.Data, func(k string, _ interface{}) bool { return commitInfoColumns[k] })})
		}
		data := lo.PickBy(row.Data, func(k string, _ interface{}) bool {
			return !commitInfoColumns[k] || k == "repository" || k == "commit_hash"
		})
		// Most files keep their path; the view reads a missing one as the file.
		if path, ok := data[pathAtCommit].(string); ok && path == toString(data["file"]) {
			data[pathAtCommit] = nil
		}
		files.Rows = append(files.Rows, &core.Row{Data: data})
	}
	return info, files
}

func toString(v interface{}) string {
	s, _ := v.(string)
	return s
}

// createCommitsView names every column of the original view in its order.
func createCommitsView(db *sql.DB, original *core.View) error {
	selects := lo.Map(original.Columns, func(column *core.Column, _ int) string {
		name := "`" + column.Name + "`"
		switch {
		case column.Name == pathAtCommit:
			return "coalesce(f." + name + ", f.`file`) AS " + name
		case commitInfoColumns[column.Name] && column.Name != "repository" && column.Name != "commit_hash":
			return "i." + name
		default:
			return "f." + name
		}
	})
	selects = append(selects, "f.report_id", "f.timestamp")
	_, err := db.Exec("CREATE VIEW IF NOT EXISTS `" + gitCommits + "` AS SELECT " + strings.Join(selects, ", ") +
		" FROM `" + gitCommitFiles + "` f JOIN `" + gitCommitInfo + "` i" +
		" ON i.commit_hash = f.commit_hash AND i.repository IS f.repository AND i.report_id = f.report_id")
	return err
}

// addReportColumns gives the tables this export created their report_id and
// timestamp, as defaults read by every row already written.
func addReportColumns(db *sql.DB, options *SqlOptions, fresh map[string]bool) error {
	reportId := sqlString(options.ReportId)
	// The format the driver writes a time.Time in, so the default reads back
	// exactly as a stored value would.
	timestamp := sqlString(options.ScanTime.Format(sqlite3.SQLiteTimestampFormats[0]))
	for _, name := range lo.Keys(fresh) {
		if _, err := db.Exec("ALTER TABLE `" + name + "` ADD COLUMN report_id TEXT DEFAULT " + reportId); err != nil {
			return err
		}
		if _, err := db.Exec("ALTER TABLE `" + name + "` ADD COLUMN timestamp DATE DEFAULT " + timestamp); err != nil {
			return err
		}
	}
	return nil
}

func sqlString(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}
