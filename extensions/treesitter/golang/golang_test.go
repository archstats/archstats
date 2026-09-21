package golang

import (
	"testing"

	"github.com/archstats/archstats/core/file"
	"github.com/archstats/archstats/core/unit"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func analyse(t *testing.T, path, src string) *file.Results {
	t.Helper()
	res := createGoLanguagePack().AnalyzeFileContent(path, []byte(src))
	require.NotNilf(t, res, "%s was not analysed", path)
	res.Units = unitsFrom(path, res)
	return res
}

func unitsByID(units []*unit.Unit) map[string]*unit.Unit {
	out := map[string]*unit.Unit{}
	for _, u := range units {
		out[u.ID] = u
	}
	return out
}

const server = `package server

import (
	"fmt"
	tsitter "github.com/tree-sitter/go-tree-sitter"
)

type Handler interface {
	Serve() error
}

type Engine struct {
	*RouterGroup
	Handler
	Name string ` + "`json:\"name\" form:\"n\"`" + `
}

func New() *Engine { return nil }

func (e *Engine) ServeHTTP(w, r int) {}

func (e Engine) Name2() string { return "" }
`

// Go's architecture is mostly not made of types: gin is 1,344 functions to
// 178, and a reading that sees only types sees a seventh of it.
func TestFunctionsAreUnits(t *testing.T) {
	res := analyse(t, "internal/server/engine.go", server)
	byID := unitsByID(res.Units)

	// A plain function.
	require.Contains(t, byID, "internal/server.New")
	assert.Equal(t, unit.KindFunction, byID["internal/server.New"].Kind)
	assert.Empty(t, byID["internal/server.New"].Owner, "a package-level func belongs to nothing")

	// A type.
	require.Contains(t, byID, "internal/server.Engine")
	assert.Equal(t, unit.KindType, byID["internal/server.Engine"].Kind)
}

// A method belongs to its receiver and is written outside it, so where it
// lives and what it belongs to are different questions. Nothing in the file
// nests it inside the type.
func TestMethodsCarryTheirReceiver(t *testing.T) {
	byID := unitsByID(analyse(t, "internal/server/engine.go", server).Units)

	require.Contains(t, byID, "internal/server.Engine.ServeHTTP")
	m := byID["internal/server.Engine.ServeHTTP"]
	assert.Equal(t, unit.KindFunction, m.Kind)
	assert.Equal(t, "ServeHTTP", m.Name)
	assert.Equal(t, "internal/server.Engine", m.Owner, "the receiver, not the file")

	// A value receiver is the same claim as a pointer receiver.
	require.Contains(t, byID, "internal/server.Engine.Name2")
	assert.Equal(t, "internal/server.Engine", byID["internal/server.Engine.Name2"].Owner)
}

// A struct embeds rather than extends, and no import or `implements` says so.
// gin's Engine embeds RouterGroup exactly this way.
func TestEmbeddingIsRecordedLikeASupertype(t *testing.T) {
	byID := unitsByID(analyse(t, "internal/server/engine.go", server).Units)

	engine := byID["internal/server.Engine"]
	require.NotNil(t, engine)
	assert.True(t, engine.HasMarker("RouterGroup"), "a pointer embed is still an embed")
	assert.True(t, engine.HasMarker("Handler"))
	// A named field is not an embed.
	assert.False(t, engine.HasMarker("Name"))
}

// There are no annotations in Go. What it has instead is struct tags, and
// each key is a separate claim: `json:"name" form:"n"` says two things.
func TestStructTagsAreMarkers(t *testing.T) {
	byID := unitsByID(analyse(t, "internal/server/engine.go", server).Units)

	engine := byID["internal/server.Engine"]
	require.NotNil(t, engine)
	assert.Equal(t, []string{"name"}, engine.MarkerValues("json"))
	assert.Equal(t, []string{"n"}, engine.MarkerValues("form"))
}

func TestInterfacesAreMarkedAbstract(t *testing.T) {
	res := analyse(t, "internal/server/engine.go", server)
	byID := unitsByID(res.Units)
	assert.True(t, byID["internal/server.Handler"].HasMarker("interface"))
	assert.False(t, byID["internal/server.Engine"].HasMarker("interface"))

	var abstract, total int
	for _, s := range res.Snippets {
		switch s.Type {
		case file.AbstractType:
			abstract++
		case file.Type:
			total++
		}
	}
	assert.Equal(t, 1, abstract, "an interface is Go's only abstract type")
	assert.Equal(t, 2, total)
}

// The import path without its quotes, and an aliased import names the same
// package as an unaliased one.
func TestImports(t *testing.T) {
	res := analyse(t, "internal/server/engine.go", server)
	var imports []string
	for _, s := range res.Snippets {
		if s.Type == file.ComponentImport {
			imports = append(imports, s.Value)
		}
	}
	assert.ElementsMatch(t, []string{"fmt", "github.com/tree-sitter/go-tree-sitter"}, imports)
}

// `//go:` comment directives are the other thing Go has instead of
// annotations. archstats itself carries 15 of them.
func TestDirectivesAreMarkers(t *testing.T) {
	src := "package assets\n\n//go:embed definitions/**\nvar defs string\n\nfunc Load() string { return defs }\n"
	byID := unitsByID(analyse(t, "pkg/assets/load.go", src).Units)

	load := byID["pkg/assets.Load"]
	require.NotNil(t, load)
	assert.Equal(t, []string{"definitions/**"}, load.MarkerValues("embed"))
}

// A file at the repository root has no directory to be named after.
func TestRootLevelFile(t *testing.T) {
	byID := unitsByID(analyse(t, "main.go", "package main\n\nfunc main() {}\n").Units)
	assert.Contains(t, byID, "main")
}

func TestGlobMatchesRootAndNestedFiles(t *testing.T) {
	pack := createGoLanguagePack()
	for _, path := range []string{"main.go", "./main.go", "core/file/snippet.go"} {
		assert.NotNilf(t, pack.AnalyzeFileContent(path, []byte("package x\n")), "%s did not match the glob", path)
	}
	assert.Nil(t, pack.AnalyzeFileContent("notes.txt", []byte("package x\n")))
}
