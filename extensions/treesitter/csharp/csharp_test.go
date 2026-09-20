package csharp

import (
	_ "embed"
	"github.com/archstats/archstats/core/file"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"os"
	"testing"
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
