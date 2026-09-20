package kotlin

import (
	_ "embed"
	"testing"

	"github.com/archstats/archstats/core/file"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
)

//go:embed TestFile.kt
var rawFile string

func TestKotlin(t *testing.T) {
	pack := createKotlinLanguagePack()

	content := pack.AnalyzeFileContent("TestFile.kt", []byte(rawFile))

	assert.Equal(t, "io.javalin.http", content.Component)

	// The package each import lands in, which is what a component is. The
	// class name comes off; a wildcard import already names the package and
	// keeps all of it.
	expectedImports := []string{
		"io.javalin.http.servlet",
		"io.javalin.http.staticfiles",
		"io.javalin.util",
		"java.net",
		"foo.bar",
		"foo.baz", // foo.baz.*
	}

	actualImports := lo.Map(lo.Filter(content.Snippets, func(s *file.Snippet, _ int) bool {
		return s.Type == file.ComponentImport
	}), func(s *file.Snippet, _ int) string { return s.Value })

	assert.ElementsMatch(t, expectedImports, actualImports)
}

func TestKotlinImportsResolveToTheirPackage(t *testing.T) {
	for from, want := range map[string]string{
		"kotlinx.datetime.internal.JSJoda":    "kotlinx.datetime.internal",
		"kotlinx.datetime.Clock.System":       "kotlinx.datetime",        // nested class
		"io.javalin.http.servlet.isLocalhost": "io.javalin.http.servlet", // top-level function
		"kotlin.time.Duration":                "kotlin.time",
		"Singleton":                           "Singleton", // nothing left to strip to
	} {
		assert.Equal(t, want, toPackage(&file.Snippet{Value: from}).Value, from)
	}
}
