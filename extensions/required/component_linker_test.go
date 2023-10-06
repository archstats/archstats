package required

//
//import (
//	"encoding/json"
//	"github.com/archstats/archstats/analysis"
//	"github.com/samber/lo"
//	"github.com/stretchr/testify/assert"
//	"os"
//	"testing"
//)
//
//func TestCalculateResults_Smoke_RealExample(t *testing.T) {
//	snippets := parseSnippets("real_example_snippets.json")
//
//	setComponents(snippets)
//	results := analysis.aggregateResults("/", &analysis.accumulator{}, toFileResults(snippets))
//
//	assert.Len(t, results.SnippetsByComponent, 1)
//	assert.Len(t, results.SnippetsByDirectory, 1)
//	assert.Len(t, results.SnippetsByType, 3)
//	assert.Len(t, results.SnippetsByFile, 62)
//}
//
//func TestCalculateResults_ComponentConnections(t *testing.T) {
//	snippets := []*analysis.Snippet{
//		// Package/ComponentToNode declarations
//		{
//			File:  "src/main/java/mainPackage/testFile1.java",
//			Type:  "component_declaration",
//			Value: "mainPackage",
//		},
//		{
//			File:  "src/main/java/mainPackage/subpackage1/testFile2.java",
//			Type:  "component_declaration",
//			Value: "mainPackage.subpackage1",
//		},
//		{
//			File:  "src/main/java/mainPackage/subpackage2/testFile3.java",
//			Type:  "component_declaration",
//			Value: "mainPackage.subpackage2",
//		},
//
//		// Package/ComponentToNode imports
//		{
//			File:  "src/main/java/mainPackage/testFile1.java",
//			Type:  "component_import",
//			Value: "mainPackage.subpackage1",
//		},
//		{
//			File:  "src/main/java/mainPackage/testFile1.java",
//			Type:  "component_import",
//			Value: "someRandomPackage.thatsNotInCodebase",
//		},
//		{
//			File:  "src/main/java/mainPackage/testFile1.java",
//			Type:  "component_import",
//			Value: "mainPackage.subpackage2",
//		},
//		{
//			File:  "src/main/java/mainPackage/subpackage1/testFile2.java",
//			Type:  "component_import",
//			Value: "mainPackage.subpackage2",
//		},
//	}
//	setComponents(snippets)
//
//	results := analysis.aggregateResults("/", &analysis.accumulator{}, toFileResults(snippets))
//
//	connections, from, to := results.Connections, results.ConnectionsFrom, results.ConnectionsTo
//	assertHasAConnection := func(from, to string) {
//		for _, connection := range connections {
//			if connection.From == from && connection.To == to {
//				return
//			}
//		}
//		assert.Fail(t, "No connection found from %s to %s", from, to)
//	}
//
//	assert.Len(t, connections, 3)
//	assert.Len(t, from, 2)
//	assert.Len(t, to, 2)
//
//	assertHasAConnection("mainPackage", "mainPackage.subpackage1")
//	assertHasAConnection("mainPackage", "mainPackage.subpackage2")
//	assertHasAConnection("mainPackage.subpackage1", "mainPackage.subpackage2")
//}
//
//func toFileResults(snippets []*analysis.Snippet) []*file.Results {
//	groupedByFile := lo.GroupBy(snippets, func(snippet *analysis.Snippet) string {
//		return snippet.File
//	})
//	allFileResults := lo.MapToSlice(groupedByFile, func(key string, snippets []*analysis.Snippet) *file.Results {
//		return &file.Results{
//			Name:     key,
//			Stats:    analysis.SnippetsToStats(snippets),
//			Snippets: snippets,
//		}
//	})
//	return allFileResults
//}
//
//func parseSnippets(fileName string) []*analysis.Snippet {
//	file, err := os.ReadFile(fileName)
//
//	if err != nil {
//		panic(err)
//	}
//
//	var snippets []*analysis.Snippet
//	err = json.Unmarshal(file, &snippets)
//	if err != nil {
//		panic(err)
//	}
//
//	return snippets
//}
