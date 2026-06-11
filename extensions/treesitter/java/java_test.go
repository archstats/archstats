package java

import (
	"github.com/archstats/archstats/core"
	"github.com/archstats/archstats/core/file"
	"github.com/archstats/archstats/core/stats"
	"github.com/archstats/archstats/extensions/basic"
	"github.com/archstats/archstats/extensions/treesitter/common"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"io/fs"
	"os"
	"strings"
	"testing"
	"time"
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

type mockFile struct {
	path    string
	content []byte
}

func (m *mockFile) Path() string             { return m.path }
func (m *mockFile) Content() []byte         { return m.content }
func (m *mockFile) Name() string             { return m.path }
func (m *mockFile) Size() int64              { return int64(len(m.content)) }
func (m *mockFile) Mode() fs.FileMode        { return 0 }
func (m *mockFile) ModTime() time.Time       { return time.Time{} }
func (m *mockFile) IsDir() bool              { return false }
func (m *mockFile) Sys() interface{}         { return nil }

func TestJavaAnalyzerStats(t *testing.T) {
	extension := &Extension{IgnoreCommonJavaImports: false}
	lp := extension.createJavaLanguagePack()
	analyzer := &javaAnalyzer{lp: lp}

	fileName := "TestClass.java"
	fileRaw, err := os.ReadFile(fileName)
	assert.NoError(t, err)

	results := analyzer.AnalyzeFile(&mockFile{path: fileName, content: fileRaw})
	assert.NotNil(t, results)

	// Check that stats are extracted correctly
	javaClassStat, foundClass := lo.Find(results.Stats, func(item *stats.Record) bool {
		return item.StatType == "java_class"
	})
	assert.True(t, foundClass)
	assert.Equal(t, "JavalinService", javaClassStat.Value)

	javaFullClassStat, foundFullClass := lo.Find(results.Stats, func(item *stats.Record) bool {
		return item.StatType == "java_full_class"
	})
	assert.True(t, foundFullClass)
	assert.Equal(t, "com.elepy.javalin.JavalinService", javaFullClassStat.Value)
}

func TestViews(t *testing.T) {
	analyzer := core.New(&core.Config{
		RootPath:   ".",
		Extensions: []core.Extension{basic.Extension(), &Extension{}},
	})
	results, err := analyzer.Analyze()
	assert.NoError(t, err)

	// Test files view columns
	filesView, err := results.RenderView("files")
	assert.NoError(t, err)
	assert.NotNil(t, filesView)

	// Check that java_class and java_full_class columns exist in filesView
	hasJavaClassCol := false
	hasJavaFullClassCol := false
	for _, col := range filesView.Columns {
		if col.Name == "java_class" {
			hasJavaClassCol = true
			assert.Equal(t, core.String, col.Type)
		}
		if col.Name == "java_full_class" {
			hasJavaFullClassCol = true
			assert.Equal(t, core.String, col.Type)
		}
	}
	assert.True(t, hasJavaClassCol)
	assert.True(t, hasJavaFullClassCol)

	// Find the row for TestClass.java in filesView
	var testClassRow *core.Row
	for _, row := range filesView.Rows {
		if strings.HasSuffix(row.Data["name"].(string), "TestClass.java") {
			testClassRow = row
			break
		}
	}
	assert.NotNil(t, testClassRow)
	assert.Equal(t, "JavalinService", testClassRow.Data["java_class"])
	assert.Equal(t, "com.elepy.javalin.JavalinService", testClassRow.Data["java_full_class"])

	// Test class connections direct view
	directView, err := results.RenderView("java_class_connections_direct")
	assert.NoError(t, err)
	assert.NotNil(t, directView)

	// Test class connections indirect view
	indirectView, err := results.RenderView("java_class_connections_indirect")
	assert.NoError(t, err)
	assert.NotNil(t, indirectView)
}

func assertSnippetCount(t *testing.T, snippets []*file.Snippet, snippetType string, expected int) {
	actual := lo.Filter(snippets, func(snippet *file.Snippet, index int) bool {
		return snippet.Type == snippetType
	})
	assert.Len(t, actual, expected)
}

func createJavaLanguagePack() *common.LanguagePack {
	extension := &Extension{IgnoreCommonJavaImports: false}
	return extension.createJavaLanguagePack()
}
