package e2eTest

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// Budgets for a scan of a mid-sized codebase: how big its snapshot may grow
// and how long it may take.
//
// Twice the snapshot outgrew what the desktop app could open (revisions 3
// and 4 were both "smaller snapshots"), and a pass over file pairs that was
// quadratic in the number of files made large scans crawl. Neither showed in
// a test, because the fixtures are a few dozen files. This workspace is a
// thousand, with history, generated the same way every time.
//
// The ceilings sit well above today's numbers, so they catch a change of
// kind -- a table that grows with the square of the files, a pass that does
// -- and not noise. When one is crossed on purpose, raise it here and say
// why in the commit.
const (
	budgetSnapshotBytes = 20 << 20
	budgetScan          = 30 * time.Second
)

func TestScanBudget(t *testing.T) {
	if testing.Short() {
		t.Skip("scans a generated thousand-file workspace; run without -short")
	}
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	root := syntheticWorkspace(t, 40, 25)

	start := time.Now()
	db := scanToSnapshot(t, root)
	took := time.Since(start)
	info, err := os.Stat(db)
	require.NoError(t, err)
	t.Logf("1000 files: snapshot %.1f MB, scan and export %s", float64(info.Size())/(1<<20), took.Round(time.Millisecond))

	require.LessOrEqualf(t, info.Size(), int64(budgetSnapshotBytes), "the snapshot is %.1f MB, over its %d MB budget", float64(info.Size())/(1<<20), budgetSnapshotBytes>>20)
	require.LessOrEqualf(t, took, budgetScan, "the scan took %s, over its %s budget", took, budgetScan)
}

// BenchmarkScan scans the same workspace; compare two commits with
//
//	go test ./e2eTest -run '^$' -bench BenchmarkScan -count 6 | tee new.txt
//	go run golang.org/x/perf/cmd/benchstat@latest old.txt new.txt
func BenchmarkScan(b *testing.B) {
	root := syntheticWorkspace(b, 40, 25)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		scanToSnapshot(b, root)
	}
}

// syntheticWorkspace is packages × classes Java files, each importing three
// classes from other packages, committed over twenty commits that each touch
// a slice of them. The same arguments make the same workspace.
func syntheticWorkspace(t testing.TB, packages, classes int) string {
	t.Helper()
	root := t.TempDir()
	write := func(i, j, version int) {
		var imports, fields strings.Builder
		for k := 1; k <= 3; k++ {
			p, c := (i*7+j*k+k)%packages, (j*5+k)%classes
			if p == i {
				p = (p + 1) % packages
			}
			fmt.Fprintf(&imports, "import com.acme.p%d.C%d;\n", p, c)
			fmt.Fprintf(&fields, "  private C%d f%d;\n", c, k)
		}
		body := fmt.Sprintf("package com.acme.p%d;\n\n%s\npublic class C%d {\n%s  public int m(int x) {\n    if (x > %d) {\n      for (int i = 0; i < x; i++) { x += i; }\n    }\n    return x;\n  }\n}\n",
			i, imports.String(), j, fields.String(), version)
		path := filepath.Join(root, "src", "main", "java", "com", "acme", fmt.Sprintf("p%d", i), fmt.Sprintf("C%d.java", j))
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, []byte(body), 0o644))
	}
	git := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		cmd.Env = fixtureGitEnv()
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, string(out))
	}
	for i := 0; i < packages; i++ {
		for j := 0; j < classes; j++ {
			write(i, j, 0)
		}
	}
	git("init", "-q", "-b", "main")
	git("add", ".")
	git("commit", "-q", "-m", "initial")
	for c := 1; c <= 20; c++ {
		// Each commit touches about forty files across several packages,
		// so co-change has pairs to count.
		for k := 0; k < 40; k++ {
			write((c*3+k)%packages, (c*11+k*7)%classes, c)
		}
		git("commit", "-q", "-am", fmt.Sprintf("change %d", c))
	}
	return root
}
