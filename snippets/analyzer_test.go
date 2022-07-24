package snippets

import (
	"encoding/json"
	"github.com/stretchr/testify/assert"
	"os"
	"testing"
)

func TestCalculateResults_Smoke_RealExample(t *testing.T) {

	snippets := parseSnippets("real_example_snippets.json")

	results := CalculateResults(snippets)

	assert.Len(t, results.SnippetsByComponent, 1)
	assert.Len(t, results.SnippetsByDirectory, 1)
	assert.Len(t, results.SnippetsByType, 3)
	assert.Len(t, results.SnippetsByFile, 62)
}

func TestCalculateResults_ComponentConnections(t *testing.T) {
	snippets := []*Snippet{
		// Package/Component declarations
		{
			File:  "src/main/java/mainPackage/testFile1.java",
			Type:  "component_declaration",
			Value: "mainPackage",
		},
		{
			File:  "src/main/java/mainPackage/subpackage1/testFile2.java",
			Type:  "component_declaration",
			Value: "mainPackage.subpackage1",
		},
		{
			File:  "src/main/java/mainPackage/subpackage2/testFile3.java",
			Type:  "component_declaration",
			Value: "mainPackage.subpackage2",
		},

		// Package/Component imports
		{
			File:  "src/main/java/mainPackage/testFile1.java",
			Type:  "component_import",
			Value: "mainPackage.subpackage1",
		},
		{
			File:  "src/main/java/mainPackage/testFile1.java",
			Type:  "component_import",
			Value: "someRandomPackage.thatsNotInCodebase",
		},
		{
			File:  "src/main/java/mainPackage/testFile1.java",
			Type:  "component_import",
			Value: "mainPackage.subpackage2",
		},
		{
			File:  "src/main/java/mainPackage/subpackage1/testFile2.java",
			Type:  "component_import",
			Value: "mainPackage.subpackage2",
		},
	}

	results := CalculateResults(snippets)

	connections, from, to := results.Connections, results.ConnectionsFrom, results.ConnectionsTo
	assertHasAConnection := func(from, to string) {
		assert.Contains(t, connections, &ComponentConnection{From: from, To: to})
	}

	assert.Len(t, connections, 3)
	assert.Len(t, from, 2)
	assert.Len(t, to, 2)

	assertHasAConnection("mainPackage", "mainPackage.subpackage1")
	assertHasAConnection("mainPackage", "mainPackage.subpackage2")
	assertHasAConnection("mainPackage.subpackage1", "mainPackage.subpackage2")
}

func parseSnippets(fileName string) []*Snippet {
	file, err := os.ReadFile(fileName)

	if err != nil {
		panic(err)
	}

	var snippets []*Snippet
	err = json.Unmarshal(file, &snippets)
	if err != nil {
		panic(err)
	}

	return snippets
}
