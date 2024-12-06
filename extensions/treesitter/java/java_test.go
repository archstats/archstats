package java

import (
	"github.com/archstats/archstats/core/file"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"os"
	"testing"
)

func TestImports(t *testing.T) {
	pack := createJavaLanguagePack()

	fileName := "TestClass.java"
	fileRaw, err := os.ReadFile(fileName)
	if err != nil {
		t.Error(err)
	}
	results := pack.AnalyzeFileContent(fileName, fileRaw)

	assertSnippetCount(t, results.Snippets, "modularity__component__imports", 6)
}

func TestDeclarations(t *testing.T) {
	pack := createJavaLanguagePack()

	fileName := "TestClass.java"
	fileRaw, err := os.ReadFile(fileName)
	if err != nil {
		t.Error(err)
	}
	results := pack.AnalyzeFileContent(fileName, fileRaw)

	assertSnippetCount(t, results.Snippets, "modularity__component__declarations", 1)
}

func TestInterfaces(t *testing.T) {
	pack := createJavaLanguagePack()

	fileName := "TestInterface.java"
	fileRaw, err := os.ReadFile(fileName)
	if err != nil {
		t.Error(err)
	}
	results := pack.AnalyzeFileContent(fileName, fileRaw)

	assertSnippetCount(t, results.Snippets, "modularity__types__abstract", 1)
	assertSnippetCount(t, results.Snippets, "modularity__types__total", 1)
}

func TestAbstractClasses(t *testing.T) {
	pack := createJavaLanguagePack()

	fileName := "TestAbstractClass.java"
	fileRaw, err := os.ReadFile(fileName)
	if err != nil {
		t.Error(err)
	}
	results := pack.AnalyzeFileContent(fileName, fileRaw)

	assertSnippetCount(t, results.Snippets, "modularity__types__abstract", 1)
	assertSnippetCount(t, results.Snippets, "modularity__types__total", 1)
}

func TestClasses(t *testing.T) {
	pack := createJavaLanguagePack()

	fileName := "TestClass.java"
	fileRaw, err := os.ReadFile(fileName)
	if err != nil {
		t.Error(err)
	}
	results := pack.AnalyzeFileContent(fileName, fileRaw)

	assertSnippetCount(t, results.Snippets, "modularity__types__abstract", 0)
	assertSnippetCount(t, results.Snippets, "modularity__types__total", 1)
}

func TestRecords(t *testing.T) {
	pack := createJavaLanguagePack()

	fileName := "TestRecord.java"
	fileRaw, err := os.ReadFile(fileName)
	if err != nil {
		t.Error(err)
	}
	results := pack.AnalyzeFileContent(fileName, fileRaw)

	assertSnippetCount(t, results.Snippets, "modularity__types__abstract", 0)
	assertSnippetCount(t, results.Snippets, "modularity__types__total", 1)
}

func assertSnippetCount(t *testing.T, snippets []*file.Snippet, snippetType string, expected int) {
	actual := lo.Filter(snippets, func(snippet *file.Snippet, index int) bool {
		return snippet.Type == snippetType
	})
	assert.Len(t, actual, expected)
}
