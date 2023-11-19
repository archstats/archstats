package csharp

import (
	_ "embed"
	"github.com/archstats/archstats/core/file"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
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
