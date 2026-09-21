package declbased

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// An alias map must only know about files the analysis actually saw.
//
// readAliases walked the tree itself, with its own skip list rather than the
// walker's ignore handling (ADR 0012). Run against archstats' own repository
// it found 23 alias entries, among them `elepy-vue` pointing into
// `e2eTest/temp_testdata/` -- a vendored checkout the repository explicitly
// ignores in .gitignore. An import could then resolve into a directory the
// user excluded on purpose, and nothing would say so.
func TestReadAliasesFrom_OnlyReadsGivenFiles(t *testing.T) {
	root := t.TempDir()
	write(t, root, "package.json", `{"name":"app"}`)
	write(t, root, "packages/shared/package.json", `{"name":"@acme/shared"}`)
	write(t, root, "packages/shared/src/index.ts", "export const x = 1;")
	// The shape of an ignored vendored checkout: a real manifest, in a
	// directory the walker would never hand us.
	write(t, root, "vendored/other/package.json", `{"name":"@other/thing"}`)
	write(t, root, "vendored/other/src/index.ts", "export const y = 2;")

	visible := []string{
		"package.json",
		"packages/shared/package.json",
		"packages/shared/src/index.ts",
	}
	m := readAliasesFrom(root, visible)

	assert.NotEmpty(t, prefixesOf(m), "the project's own packages must still be read")
	assert.Contains(t, prefixesOf(m), "@acme/shared")
	assert.NotContainsf(t, prefixesOf(m), "@other/thing",
		"a manifest the walker did not hand us must not be read: %v", prefixesOf(m))
}

// The walking variant still exists for tests and standalone callers, and
// must agree with the file-list variant when nothing is ignored.
func TestReadAliases_AgreesWithReadAliasesFromWhenNothingIsHidden(t *testing.T) {
	root := t.TempDir()
	write(t, root, "package.json", `{"name":"app"}`)
	write(t, root, "packages/ui/package.json", `{"name":"@acme/ui","main":"src/index.ts"}`)
	write(t, root, "packages/ui/src/index.ts", "export const z = 3;")

	walked := prefixesOf(readAliases(root))
	listed := prefixesOf(readAliasesFrom(root, []string{
		"package.json", "packages/ui/package.json", "packages/ui/src/index.ts",
	}))
	assert.ElementsMatch(t, walked, listed)
}

func TestReadAliasesFrom_EmptyRootIsNotAnError(t *testing.T) {
	assert.Empty(t, prefixesOf(readAliasesFrom("", []string{"package.json"})))
	assert.Empty(t, prefixesOf(readAliasesFrom(t.TempDir(), nil)))
}

// A manifest that is unreadable, malformed or nameless is skipped rather
// than failing the run: one bad package.json in a large monorepo must not
// take the analysis with it.
func TestReadAliasesFrom_SurvivesBadManifests(t *testing.T) {
	root := t.TempDir()
	write(t, root, "a/package.json", `{ this is not json`)
	write(t, root, "b/package.json", `{"version":"1.0.0"}`)
	write(t, root, "c/package.json", `{"name":"@acme/good"}`)

	m := readAliasesFrom(root, []string{"a/package.json", "b/package.json", "c/package.json", "missing/package.json"})
	assert.Equal(t, []string{"@acme/good", "@acme/good/"}, sortedUnique(prefixesOf(m)))
}

func prefixesOf(m *aliasMap) []string {
	var out []string
	for _, e := range m.entries {
		out = append(out, e.prefix)
	}
	return out
}

func sortedUnique(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	for i := range out {
		for j := i + 1; j < len(out); j++ {
			if out[j] < out[i] {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out
}

func write(t *testing.T, root, rel, content string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(rel))
	require.NoError(t, os.MkdirAll(filepath.Dir(full), 0o755))
	require.NoError(t, os.WriteFile(full, []byte(content), 0o644))
}
