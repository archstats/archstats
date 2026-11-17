package e2eTest

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/archstats/archstats/cmd/export/sqlite"
	"github.com/archstats/archstats/core"
	"github.com/archstats/archstats/engine"
	"github.com/archstats/archstats/internal/snapdigest"
	"github.com/stretchr/testify/require"
)

// What the fixtures' scans say, pinned table by table, and held to
// core.AnalysisRevision.
//
// The schema golden pins the snapshot's shape; this pins its content. A
// change that alters what a scan produces for the same source must bump
// core.AnalysisRevision, and nothing used to check that it did. Now the
// digest records the revision it was taken at, and the test fails when the
// content moved but the revision did not: bump it in core/revision.go, say
// why, then record the new digest:
//
//	UPDATE_GOLDEN=1 go test ./e2eTest -run TestScanDigest
//
// The update itself refuses to record changed content under an unchanged
// revision. A refactor must leave this test passing untouched; to see which
// rows moved, run scripts/scan-parity.sh.
const digestGolden = "testdata/scan_digest.golden"

func TestScanDigest(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	fixtures := []struct {
		name string
		root func(t *testing.T) string
	}{
		{"workspace", schemaWorkspace},
		{"simple_components", copyFixture("simple_components")},
		{"rootlevel", copyFixture("rootlevel")},
		{"vendored", copyFixture("vendored")},
	}
	var body strings.Builder
	for _, f := range fixtures {
		tables, err := snapdigest.Read(scanToSnapshot(t, f.root(t)), snapdigest.Options{})
		require.NoError(t, err)
		fmt.Fprintf(&body, "## %s\n%s", f.name, snapdigest.Digest(tables))
	}
	actual := body.String()

	golden, err := os.ReadFile(digestGolden)
	if err != nil && !os.IsNotExist(err) {
		require.NoError(t, err)
	}
	recordedRevision, recorded := parseDigest(string(golden))
	changed := golden == nil || recorded != actual

	if os.Getenv("UPDATE_GOLDEN") != "" {
		if golden != nil && changed && recordedRevision == core.AnalysisRevision {
			t.Fatalf("the scans changed but core.AnalysisRevision is still %d: bump it in core/revision.go and say why, then update", core.AnalysisRevision)
		}
		header := fmt.Sprintf("# Scan digest of the e2eTest fixtures. Regenerate with UPDATE_GOLDEN=1 go test ./e2eTest -run TestScanDigest\nrevision %d\n", core.AnalysisRevision)
		require.NoError(t, os.WriteFile(digestGolden, []byte(header+actual), 0o644))
		return
	}
	require.NotNil(t, golden, "no scan digest yet: run with UPDATE_GOLDEN=1")
	switch {
	case changed && recordedRevision == core.AnalysisRevision:
		t.Errorf("the fixtures scan differently but core.AnalysisRevision is still %d.\n"+
			"If the change is meant, bump it in core/revision.go, say why, and run UPDATE_GOLDEN=1 go test ./e2eTest -run TestScanDigest.\n"+
			"If it is a refactor, it changed what a scan says: scripts/scan-parity.sh shows the rows.\n%s",
			core.AnalysisRevision, lineDiff(recorded, actual))
	case changed:
		t.Errorf("core.AnalysisRevision moved to %d and the scans changed: record them with UPDATE_GOLDEN=1 go test ./e2eTest -run TestScanDigest\n%s",
			core.AnalysisRevision, lineDiff(recorded, actual))
	case recordedRevision != core.AnalysisRevision:
		t.Errorf("core.AnalysisRevision moved to %d but the digest was recorded at %d: run UPDATE_GOLDEN=1 go test ./e2eTest -run TestScanDigest",
			core.AnalysisRevision, recordedRevision)
	}
}

// scanToSnapshot scans root with every extension and writes the snapshot the
// CLI and the desktop app would, returning its path.
func scanToSnapshot(t testing.TB, root string) string {
	t.Helper()
	results, err := engine.Analyze(engine.Options{Root: root})
	require.NoError(t, err)
	var views []*core.View
	for _, factory := range results.GetViewFactories() {
		view, err := results.RenderView(factory.Name)
		require.NoError(t, err, factory.Name)
		views = append(views, view)
	}
	dbFile := filepath.Join(t.TempDir(), "scan.db")
	require.NoError(t, sqlite.SaveToDB(&sqlite.SqlOptions{DatabaseName: dbFile, ReportId: "digest", ScanTime: time.Unix(0, 0)}, results, views))
	return dbFile
}

// copyFixture copies a fixture out of this repository, so the git extension
// reads no history of archstats' own.
func copyFixture(name string) func(t *testing.T) string {
	return func(t *testing.T) string {
		t.Helper()
		root := filepath.Join(t.TempDir(), name)
		require.NoError(t, os.CopyFS(root, os.DirFS(name)))
		return root
	}
}

// parseDigest splits a digest file into its revision and its content.
func parseDigest(s string) (int, string) {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	revision := -1
	var body strings.Builder
	for _, line := range strings.SplitAfter(s, "\n") {
		switch {
		case line == "" || strings.HasPrefix(line, "# "):
		case strings.HasPrefix(line, "revision "):
			revision, _ = strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(line, "revision ")))
		default:
			body.WriteString(line)
		}
	}
	return revision, body.String()
}

// lineDiff lists the lines each side has that the other lacks, each under
// the fixture it belongs to.
func lineDiff(old, new string) string {
	keyed := func(s string) []string {
		var out []string
		section := ""
		for _, l := range strings.Split(s, "\n") {
			if strings.HasPrefix(l, "## ") {
				section = strings.TrimPrefix(l, "## ")
				continue
			}
			if l != "" {
				out = append(out, section+": "+l)
			}
		}
		return out
	}
	set := func(ls []string) map[string]bool {
		m := map[string]bool{}
		for _, l := range ls {
			m[l] = true
		}
		return m
	}
	o, n := keyed(old), keyed(new)
	inOld, inNew := set(o), set(n)
	var b strings.Builder
	for _, l := range o {
		if !inNew[l] {
			fmt.Fprintf(&b, "  - %s\n", l)
		}
	}
	for _, l := range n {
		if !inOld[l] {
			fmt.Fprintf(&b, "  + %s\n", l)
		}
	}
	return b.String()
}
