package component

import (
	"testing"

	"github.com/archstats/archstats/core/file"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func imp(snippetType, from, to string) *file.Snippet {
	return &file.Snippet{
		Type: snippetType, Component: from, Value: to, File: from + "/a.ts",
		Begin: &file.Position{Line: 1}, End: &file.Position{Line: 1},
	}
}

func groups(snippets ...*file.Snippet) (file.SnippetGroup, file.SnippetGroup) {
	byType := file.SnippetGroup{}
	byComponent := file.SnippetGroup{}
	for _, s := range snippets {
		byType[s.Type] = append(byType[s.Type], s)
	}
	for _, name := range []string{"app", "shared", "ui"} {
		byComponent[name] = []*file.Snippet{{Component: name}}
	}
	return byType, byComponent
}

// An edge nobody classified is an ordinary static import. That is the zero
// value on purpose: every edge recorded before kinds existed must keep
// behaving exactly as it did.
func TestConnections_UnclassifiedIsAnImport(t *testing.T) {
	byType, byComponent := groups(imp(file.ComponentImport, "app", "shared"))
	conns := GetConnectionsFromSnippetImports(byType, byComponent)

	require.Len(t, conns, 1)
	assert.Equal(t, KindImport, conns[0].Type)
	assert.Equal(t, "import", conns[0].Kind(), "displayed as import, not as an empty string")
	assert.True(t, conns[0].IsRuntime())
}

func TestConnections_KindsAreCarried(t *testing.T) {
	byType, byComponent := groups(
		imp(file.ComponentImport, "app", "shared"),
		imp(file.ComponentImportTypeOnly, "app", "ui"),
		imp(file.ComponentImportDynamic, "app", "shared"),
	)
	conns := GetConnectionsFromSnippetImports(byType, byComponent)

	kinds := map[string]int{}
	for _, c := range conns {
		kinds[c.Kind()]++
	}
	assert.Equal(t, map[string]int{"import": 1, "type_only": 1, "dynamic": 1}, kinds)
}

// The whole point of the kind: a type-only import is a real dependency on a
// shape and none at all once the program runs, so it must not reach the
// graph that coupling is computed from.
func TestRuntimeOnly_ExcludesErasedImports(t *testing.T) {
	byType, byComponent := groups(
		imp(file.ComponentImport, "app", "shared"),
		imp(file.ComponentImportTypeOnly, "app", "ui"),
		imp(file.ComponentImportDynamic, "app", "shared"),
	)
	all := GetConnectionsFromSnippetImports(byType, byComponent)
	runtime := RuntimeOnly(all)

	assert.Len(t, all, 3)
	assert.Len(t, runtime, 2, "only the erased import is dropped")
	for _, c := range runtime {
		assert.NotEqual(t, KindTypeOnly, c.Type)
	}
	// A dynamic edge is inferred rather than read, but it is still there at
	// runtime -- that is the only reason it is worth resolving.
	assert.True(t, (&Connection{Type: KindDynamic}).IsRuntime())
	assert.True(t, (&Connection{Type: KindEmbed}).IsRuntime())
	assert.True(t, (&Connection{Type: KindManifest}).IsRuntime())
}

// An import naming something outside the codebase is not an edge. A resolver
// loose enough to invent one fills an enterprise graph with phantom
// components, so this holds for every kind.
func TestConnections_UnknownTargetsAreNotEdges(t *testing.T) {
	byType, byComponent := groups(
		imp(file.ComponentImport, "app", "react"),
		imp(file.ComponentImportTypeOnly, "app", "@types/node"),
		imp(file.ComponentImportDynamic, "app", "nowhere.at.all"),
	)
	assert.Empty(t, GetConnectionsFromSnippetImports(byType, byComponent))
}

// A component importing itself is not a dependency, whatever the kind.
func TestConnections_SelfReferencesAreSkipped(t *testing.T) {
	byType, byComponent := groups(
		imp(file.ComponentImport, "app", "app"),
		imp(file.ComponentImportTypeOnly, "app", "app"),
		imp(file.ComponentImportDynamic, "app", "app"),
	)
	assert.Empty(t, GetConnectionsFromSnippetImports(byType, byComponent))
}

func TestRuntimeOnly_EmptyInput(t *testing.T) {
	assert.Empty(t, RuntimeOnly(nil))
}
