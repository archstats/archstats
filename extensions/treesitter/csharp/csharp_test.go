package csharp

import (
	_ "embed"
	"os"
	"testing"

	"github.com/archstats/archstats/core/file"
	"github.com/archstats/archstats/core/unit"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

//go:embed TestFile.cs
var rawFile string

func TestCSharp(t *testing.T) {
	pack := createCSharpLanguagePack()
	content := pack.AnalyzeFileContent("TestFile.cs", []byte(rawFile))
	expectedImports := []string{
		"System",
		"System.Collections.Generic",
		"System.Linq",
		"System.Threading.Tasks",
		"Microsoft.AspNetCore.Mvc",
		"OAA.Service",
		"OAA.Web.Models",
		"Microsoft.AspNetCore.Mvc.Rendering",
		"OAA.Data",
		"Microsoft.AspNetCore.Http",
	}
	actualImports := lo.Map(lo.Filter(content.Snippets, func(snippet *file.Snippet, idx int) bool {
		return snippet.Type == "modularity__component__imports"
	}), func(snippet *file.Snippet, idx int) string {
		return snippet.Value
	})
	assert.ElementsMatch(t, expectedImports, actualImports)
	assert.Equal(t, "OAA.Web.Controllers", content.Component)
	for _, snippet := range content.Snippets {
		assert.Equal(t, "OAA.Web.Controllers", snippet.Component)
	}
}

func TestAbstractTypes(t *testing.T) {
	pack := createCSharpLanguagePack()

	fileName := "TestAbstractTypes.cs"
	fileRaw, err := os.ReadFile(fileName)
	if err != nil {
		t.Error(err)
	}
	results := pack.AnalyzeFileContent(fileName, fileRaw)

	assertSnippetCount(t, results.Snippets, "modularity__types__abstract", 3)
	assertSnippetCount(t, results.Snippets, "modularity__types__total", 6)
}

func assertSnippetCount(t *testing.T, snippets []*file.Snippet, snippetType string, expected int) {
	actual := lo.Filter(snippets, func(snippet *file.Snippet, index int) bool {
		return snippet.Type == snippetType
	})
	assert.Len(t, actual, expected)
}

// A file-scoped namespace is a different node from a block-scoped one, so a
// query written for `namespace X { }` finds nothing in a codebase written in
// the style every .NET 6+ project template uses. Importing nopCommerce that
// way produced five components for thousands of files.
func TestFileScopedNamespace(t *testing.T) {
	pack := createCSharpLanguagePack()

	fileRaw, err := os.ReadFile("TestFileScoped.cs")
	if err != nil {
		t.Fatal(err)
	}
	results := pack.AnalyzeFileContent("TestFileScoped.cs", fileRaw)

	assert.Equal(t, "Nop.Services.Catalog", results.Component)
	for _, snippet := range results.Snippets {
		assert.Equal(t, "Nop.Services.Catalog", snippet.Component)
	}

	imports := lo.Map(lo.Filter(results.Snippets, func(s *file.Snippet, _ int) bool {
		return s.Type == "modularity__component__imports"
	}), func(s *file.Snippet, _ int) string { return s.Value })
	assert.ElementsMatch(t, []string{
		"System.Text", "System", "System.Collections.Generic",
		"Nop.Core.Domain.Catalog", "System.Math", "Nop.Core.Domain.Catalog",
	}, imports)
}

// A partial class is one type spread across several files. nopCommerce
// declares 1,567 of them, and a per-file record of "the type in this file"
// keeps whichever file was analysed last and discards the rest in silence.
func TestPartialClassIsOneUnit(t *testing.T) {
	units := unitsInFiles(t, "TestPartialA.cs", "TestPartialB.cs")

	merged := unit.Merge(units)
	require.Len(t, merged, 1, "a partial class is one type, got %v", idsOf(merged))

	customer := merged[0]
	assert.Equal(t, "Acme.Core.Domain.Customer", customer.ID)
	assert.Equal(t, unit.KindType, customer.Kind)
	assert.ElementsMatch(t, []string{"TestPartialA.cs", "TestPartialB.cs"}, customer.Files)

	// The half carrying the attribute and the half carrying the base types
	// both survive the fold. Losing either is how a type gets classified as
	// nothing in particular.
	assert.True(t, customer.HasMarker("Table"), "lost the attribute from the other file")
	assert.True(t, customer.HasMarker("BaseEntity"), "lost the base class")
	assert.True(t, customer.HasMarker("ISoftDeletable"), "lost the interface")
}

// Several top-level types in one file is ordinary C#: 141 of nopCommerce's
// files are written this way. Each is its own unit, and an attribute belongs
// only to the type it sits above.
func TestSeveralTypesInOneFile(t *testing.T) {
	units := unit.Merge(unitsInFiles(t, "TestManyTypes.cs"))

	// The enum belongs here too. It was invisible until enums were counted
	// as types, which undercounted nopCommerce by 143 of 4,067.
	assert.ElementsMatch(t, []string{
		"Acme.Core.Contracts.OrderRequest",
		"Acme.Core.Contracts.IOrderHandler",
		"Acme.Core.Contracts.OrderPlaced",
		"Acme.Core.Contracts.OrderState",
	}, idsOf(units))

	for _, u := range units {
		if u.Name == "OrderRequest" {
			assert.True(t, u.HasMarker("Serializable"))
		} else {
			assert.Falsef(t, u.HasMarker("Serializable"),
				"%s does not carry [Serializable]; it sits above OrderRequest", u.Name)
		}
	}
}

func unitsInFiles(t *testing.T, names ...string) []*unit.Unit {
	t.Helper()
	analyzer := &csharpAnalyzer{lp: createCSharpLanguagePack()}
	var out []*unit.Unit
	for _, name := range names {
		raw, err := os.ReadFile(name)
		require.NoError(t, err)
		res := analyzer.lp.AnalyzeFileContent(name, raw)
		require.NotNilf(t, res, "%s was not analysed", name)
		out = append(out, unitsFrom(name, res)...)
	}
	return out
}

func idsOf(units []*unit.Unit) []string {
	var out []string
	for _, u := range units {
		out = append(out, u.ID)
	}
	return out
}

// An enum is a type. Excluding it undercounted nopCommerce by 143 types of
// 4,067 and, since abstractness is abstract types over total types,
// overstated abstractness everywhere enums are common.
func TestEnumsCountAsTypes(t *testing.T) {
	pack := createCSharpLanguagePack()
	raw, err := os.ReadFile("TestManyTypes.cs")
	require.NoError(t, err)
	results := pack.AnalyzeFileContent("TestManyTypes.cs", raw)

	assertSnippetCount(t, results.Snippets, "modularity__types__total", 4)
	// An enum is concrete: it must not move abstractness.
	assertSnippetCount(t, results.Snippets, "modularity__types__abstract", 1)
}
