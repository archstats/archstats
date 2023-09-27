package regex

import (
	"github.com/stretchr/testify/assert"
	"os"
	"regexp"
	"sort"
	"testing"
)

func TestRegexBasedSnippetsProvider_GetSnippetsFromFile(t *testing.T) {
	// Arrange
	file := &fakeFile{
		name: "file1",
		content: `
package aCoolPackage;

import something;
field aField1: int;
field aField2: int;

function function1(){
//comment
}
`,
	}
	provider := &Extension{Patterns: []*regexp.Regexp{
		regexp.MustCompile(`package (?P<packageDeclaration>.*);`),
		regexp.MustCompile(`import (?P<importStatement>.*);`),
		regexp.MustCompile(`field (?P<fieldDeclaration>.*): [a-zA-Z]+;`),
		regexp.MustCompile(`function (?P<functionDeclaration>.*)\{[\S\s]*}`),
	}}

	//Act
	results := provider.AnalyzeFile(file)
	snippets := results.Snippets

	//Assert
	sort.Slice(snippets, func(i, j int) bool {
		return snippets[i].Begin.Offset < snippets[j].Begin.Offset
	})

	assert.Len(t, snippets, 5)

	for _, snippet := range snippets {
		assert.Equal(t, "file1", snippet.File)
	}

	assert.Equal(t, "aCoolPackage", snippets[0].Value)
	assert.Equal(t, "something", snippets[1].Value)
	assert.Equal(t, "aField1", snippets[2].Value)
	assert.Equal(t, "aField2", snippets[3].Value)
	assert.Equal(t, "function1()", snippets[4].Value)

	assert.Equal(t, "packageDeclaration", snippets[0].Type)
	assert.Equal(t, "importStatement", snippets[1].Type)
	assert.Equal(t, "fieldDeclaration", snippets[2].Type)
	assert.Equal(t, "fieldDeclaration", snippets[3].Type)
	assert.Equal(t, "functionDeclaration", snippets[4].Type)

	assert.Equal(t, 9, snippets[0].Begin.Offset)
	assert.Equal(t, 31, snippets[1].Begin.Offset)
	assert.Equal(t, 48, snippets[2].Begin.Offset)
	assert.Equal(t, 68, snippets[3].Begin.Offset)
	assert.Equal(t, 92, snippets[4].Begin.Offset)

	assert.Equal(t, 21, snippets[0].End.Offset)
	assert.Equal(t, 40, snippets[1].End.Offset)
	assert.Equal(t, 55, snippets[2].End.Offset)
	assert.Equal(t, 75, snippets[3].End.Offset)
	assert.Equal(t, 103, snippets[4].End.Offset)
}

type fakeFile struct {
	name    string
	content string
}

func (f *fakeFile) Path() string {
	return f.name
}

func (f *fakeFile) Info() os.FileInfo {
	panic("implement me")
}

func (f *fakeFile) Content() []byte {
	return []byte(f.content)
}
