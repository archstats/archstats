package e2eTest

import (
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/archstats/archstats/cmd/export/sqlite"
	"github.com/archstats/archstats/core"
	"github.com/archstats/archstats/engine"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The snapshot is the contract between the engine and everything that reads
// it: the desktop app, `archstats assert`, reports and Ask all query its
// tables by name. Nothing declared that contract, so a renamed or dropped
// column surfaced as "no such column" in the app. This test writes it down.
//
// The fixture is a small polyglot workspace with history, so the language,
// git, matrix and CI/CD tables all appear. When the schema changes on
// purpose, regenerate the golden file and review its diff like any other
// change to a public interface:
//
//	UPDATE_GOLDEN=1 go test ./e2eTest -run TestSnapshotSchema
const schemaGolden = "testdata/snapshot_schema.golden"

func TestSnapshotSchema(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	root := schemaWorkspace(t)

	results, err := engine.Analyze(engine.Options{Root: root})
	require.NoError(t, err)

	var views []*core.View
	var problems []string
	for _, factory := range results.GetViewFactories() {
		view, err := results.RenderView(factory.Name)
		require.NoError(t, err, factory.Name)
		views = append(views, view)
		problems = append(problems, view.Check()...)
	}
	sort.Strings(problems)
	assert.Empty(t, problems, "views whose rows do not match their declared columns")

	dbFile := filepath.Join(t.TempDir(), "schema.db")
	require.NoError(t, sqlite.SaveToDB(&sqlite.SqlOptions{DatabaseName: dbFile, ReportId: "schema", ScanTime: time.Unix(0, 0), StoreContent: false}, results, views))
	actual := dumpSchema(t, dbFile)

	if os.Getenv("UPDATE_GOLDEN") != "" {
		require.NoError(t, os.MkdirAll(filepath.Dir(schemaGolden), 0o755))
		require.NoError(t, os.WriteFile(schemaGolden, []byte(actual), 0o644))
		return
	}
	expected, err := os.ReadFile(schemaGolden)
	require.NoError(t, err, "no golden schema yet: run with UPDATE_GOLDEN=1")
	assert.Equal(t, string(expected), actual, "the snapshot schema changed; if that was meant, run with UPDATE_GOLDEN=1 and review the diff")
}

// schemaWorkspace copies the architecture, enterprise and directory coupling
// fixtures (the last has cycles), adds Go,
// a pipeline and a container, and commits it all in two commits.
func schemaWorkspace(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, fixture := range []string{"architecture", "enterprise", "directory_coupling"} {
		require.NoError(t, os.CopyFS(filepath.Join(root, fixture), os.DirFS(fixture)))
	}
	write := func(path, content string) {
		full := filepath.Join(root, path)
		require.NoError(t, os.MkdirAll(filepath.Dir(full), 0o755))
		require.NoError(t, os.WriteFile(full, []byte(content), 0o644))
	}
	git := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Fixture", "GIT_AUTHOR_EMAIL=fixture@example.com", "GIT_AUTHOR_DATE=2026-01-01T00:00:00Z",
			"GIT_COMMITTER_NAME=Fixture", "GIT_COMMITTER_EMAIL=fixture@example.com", "GIT_COMMITTER_DATE=2026-01-01T00:00:00Z")
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, string(out))
	}

	write("svc/go.mod", "module example.com/svc\n\ngo 1.23\n")
	write("svc/main.go", "package main\n\nimport \"example.com/svc/store\"\n\nfunc main() { store.Open() }\n")
	write("svc/store/store.go", "package store\n\nfunc Open() {\n\tfor i := 0; i < 3; i++ {\n\t\tif i > 1 {\n\t\t\treturn\n\t\t}\n\t}\n}\n")
	write("svc/Dockerfile", "FROM golang:1.23 AS build\nWORKDIR /src\nCOPY . .\nRUN go build -o /out/svc .\n\nFROM gcr.io/distroless/base\nCOPY --from=build /out/svc /svc\nENTRYPOINT [\"/svc\"]\n")
	write(".github/workflows/ci.yml", "name: ci\non: [push]\njobs:\n  test:\n    runs-on: ubuntu-latest\n    steps:\n      - uses: actions/checkout@v4\n      - run: go test ./...\n        working-directory: svc\n  image:\n    needs: test\n    runs-on: ubuntu-latest\n    steps:\n      - uses: actions/checkout@v4\n      - run: docker build -t svc svc\n")

	git("init", "-q", "-b", "main")
	git("add", ".")
	git("commit", "-q", "-m", "first")
	write("svc/store/store.go", "package store\n\nfunc Open() {}\n\nfunc Close() {}\n")
	write("svc/main.go", "package main\n\nimport \"example.com/svc/store\"\n\nfunc main() { store.Open(); store.Close() }\n")
	git("commit", "-q", "-am", "second")
	return root
}

// dumpSchema is every table and view in the snapshot with its columns and
// their declared types, one line per table, in name order.
func dumpSchema(t *testing.T, dbFile string) string {
	t.Helper()
	db, err := sql.Open("sqlite3", dbFile)
	require.NoError(t, err)
	defer db.Close()

	rows, err := db.Query("SELECT type, name FROM sqlite_master WHERE type IN ('table', 'view') AND name NOT LIKE 'sqlite_%' ORDER BY name")
	require.NoError(t, err)
	type object struct{ kind, name string }
	var objects []object
	for rows.Next() {
		var o object
		require.NoError(t, rows.Scan(&o.kind, &o.name))
		objects = append(objects, o)
	}
	require.NoError(t, rows.Close())

	var b strings.Builder
	for _, o := range objects {
		cols, err := db.Query(fmt.Sprintf("SELECT name, type FROM pragma_table_info('%s') ORDER BY cid", o.name))
		require.NoError(t, err)
		var parts []string
		for cols.Next() {
			var name, typ string
			require.NoError(t, cols.Scan(&name, &typ))
			parts = append(parts, name+" "+typ)
		}
		require.NoError(t, cols.Close())
		fmt.Fprintf(&b, "%s %s: %s\n", o.kind, o.name, strings.Join(parts, ", "))
	}
	return b.String()
}
