package e2eTest

import (
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/archstats/archstats/core/walker"
	"github.com/archstats/archstats/engine"
	"github.com/archstats/archstats/internal/snapdigest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Graphs at their edges. Each of these shapes once produced a wrong number
// or a crash: communities on a graph with no edges, a cycle counted per
// member, infinity where no path exists, shared commits for a component with
// no history. Every view of every shape must render, with no file failing
// analysis and no NaN or infinity in any cell.
func TestScan_GraphEdgeCases(t *testing.T) {
	shapes := map[string]map[string]string{
		"empty":       {},
		"one file":    {"a/A.java": "package a;\nclass A {}\n"},
		"no edges":    {"a/A.java": "package a;\nclass A {}\n", "b/B.java": "package b;\nclass B {}\n"},
		"self import": {"a/A.java": "package a;\nimport a.A;\nclass A {}\n"},
		"two-cycle": {
			"a/A.java": "package a;\nimport b.B;\nclass A { B b; }\n",
			"b/B.java": "package b;\nimport a.A;\nclass B { A a; }\n",
		},
		"three-cycle and a tail": {
			"a/A.java": "package a;\nimport b.B;\nclass A { B b; }\n",
			"b/B.java": "package b;\nimport c.C;\nclass B { C c; }\n",
			"c/C.java": "package c;\nimport a.A;\nclass C { A a; }\n",
			"d/D.java": "package d;\nimport a.A;\nclass D { A a; }\n",
		},
		"only non-code": {"README.md": "# x\n", "data/x.json": "{}\n"},
	}
	for name, files := range shapes {
		for _, withGit := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/git=%v", name, withGit), func(t *testing.T) {
				if withGit {
					if _, err := exec.LookPath("git"); err != nil {
						t.Skip("git is not installed")
					}
				}
				root := t.TempDir()
				for path, content := range files {
					full := filepath.Join(root, filepath.FromSlash(path))
					require.NoError(t, os.MkdirAll(filepath.Dir(full), 0o755))
					require.NoError(t, os.WriteFile(full, []byte(content), 0o644))
				}
				if withGit {
					// A repository with no commits: history exists and is empty.
					runGit(t, root, "init", "-q", "-b", "main")
				}
				results, err := engine.Analyze(engine.Options{Root: root})
				require.NoError(t, err)
				for _, s := range results.SkippedFiles {
					assert.NotEqual(t, walker.SkipFailed, s.Reason, "%s failed analysis: %s", s.Path, s.Detail)
				}
				for _, factory := range results.GetViewFactories() {
					view, err := results.RenderView(factory.Name)
					require.NoError(t, err, factory.Name)
					for _, row := range view.Rows {
						for column, value := range row.Data {
							if f, ok := value.(float64); ok && (math.IsNaN(f) || math.IsInf(f, 0)) {
								t.Errorf("%s.%s is %v", factory.Name, column, f)
							}
						}
					}
				}
			})
		}
	}
}

// What a scan says depends on the tree, not on where it sits or how it was
// reached. Each case scans the same workspace two ways and requires the same
// snapshot.
func TestScan_Metamorphic(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	original := schemaWorkspace(t)
	base := readScan(t, original)

	// The folder keeps its name: CI/CD names a repository at the root after
	// it, as a person would.
	t.Run("a root folder moved elsewhere", func(t *testing.T) {
		moved := filepath.Join(t.TempDir(), "somewhere else", filepath.Base(original))
		require.NoError(t, os.MkdirAll(filepath.Dir(moved), 0o755))
		require.NoError(t, os.Rename(original, moved))
		t.Cleanup(func() { _ = os.Rename(moved, original) })
		assertSameScan(t, base, readScan(t, moved))
	})

	t.Run("a root reached through a symlink", func(t *testing.T) {
		link := filepath.Join(t.TempDir(), filepath.Base(original))
		if err := os.Symlink(original, link); err != nil {
			t.Skipf("cannot make a symlink here: %v", err)
		}
		assertSameScan(t, base, readScan(t, link))
	})

	t.Run("a root given with a trailing separator", func(t *testing.T) {
		assertSameScan(t, base, readScan(t, original+string(filepath.Separator)))
	})

	t.Run("an unrelated file leaves the dependencies alone", func(t *testing.T) {
		require.NoError(t, os.MkdirAll(filepath.Join(original, "notes"), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(original, "notes", "plan.md"), []byte("# plan\n"), 0o644))
		t.Cleanup(func() { _ = os.RemoveAll(filepath.Join(original, "notes")) })
		after := readScan(t, original)
		for _, table := range []string{"component_connections_direct", "component_connections_indirect", "unit_connections", "component_cycles"} {
			assertSameScan(t, only(base, table), only(after, table))
		}
	})
}

func readScan(t *testing.T, root string) []snapdigest.Table {
	t.Helper()
	tables, err := snapdigest.Read(scanToSnapshot(t, root), snapdigest.Options{})
	require.NoError(t, err)
	return tables
}

func only(tables []snapdigest.Table, name string) []snapdigest.Table {
	for _, t := range tables {
		if t.Name == name {
			return []snapdigest.Table{t}
		}
	}
	return nil
}

func assertSameScan(t *testing.T, want, got []snapdigest.Table) {
	t.Helper()
	if d := snapdigest.Diff(want, got, 10); d != "" {
		t.Errorf("the scans differ:\n%s", d)
	}
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = fixtureGitEnv()
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, string(out))
}
