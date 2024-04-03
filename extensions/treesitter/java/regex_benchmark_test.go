//go:build experimental
// +build experimental

package java

import (
	_ "embed"
	"github.com/archstats/archstats/core/file"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"regexp"
	"strings"
	"testing"
)

//go:embed TestJavaFile.java
var javaCode []byte

//go:embed Test_Imports
var importsRaw string

func Test(t *testing.T) {

	javaLanguage := createJavaLanguagePack()
	snippets := javaLanguage.AnalyzeFileContent("TestJavaFile.java", javaCode).Snippets
	imports := lo.Uniq(lo.Map(snippets, func(item *file.Snippet, index int) string {
		return item.Value
	}))
	expectedImports := lo.Uniq(strings.Split(importsRaw, "\n"))

	assert.ElementsMatch(t, expectedImports, imports)
}
func Benchmark(b *testing.B) {
	javaLanguage := createJavaLanguagePack()
	benchmarks := []struct {
		name    string
		analyze func(name string, content []byte) *file.Results
	}{
		{"treesitter", javaLanguage.AnalyzeFileContent},
	}
	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			bytes := javaCode
			for i := 0; i < b.N; i++ {
				bm.analyze("TestJavaFile.java", bytes)
			}
		})
	}
}

var packageRegex = regexp.MustCompile("package (?P<modularity__component__declarations>[a-z0-9_.]*)")
var importRegex = regexp.MustCompile("import (?P<modularity__component__imports>[a-z0-9_.]*)\\.[A-Z]")
