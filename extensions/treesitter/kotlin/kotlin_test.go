//go:build experimental
// +build experimental

package kotlin

import (
	_ "embed"
	"github.com/archstats/archstats/core/file"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"testing"
)

//go:embed TestFile.kt
var rawFile string

func TestKotlin(t *testing.T) {
	pack := createKotlinPack()

	content := pack.AnalyzeFileContent("TestFile.kt", []byte(rawFile))
	expectedComponent := "OAA.Web.Controllers"
	expectedImports := []string{
		"io.javalin.http.servlet",
		"io.javalin.http.staticfiles",
		"io.javalin.repo",
		"java.net",
		"x.x.x.x.x.x.x.x",
	}
	actualImports := lo.Map(lo.Filter(content.Snippets, func(snippet *file.Snippet, idx int) bool {
		return snippet.Type == "modularity__component__imports"
	}), func(snippet *file.Snippet, idx int) string {
		return snippet.Value
	})
	assert.ElementsMatch(t, expectedImports, actualImports)
	assert.Equal(t, expectedComponent, content.Component)
	for _, snippet := range content.Snippets {
		assert.Equal(t, expectedComponent, snippet.Component)
	}
}
