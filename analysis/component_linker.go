package analysis

import "github.com/samber/lo"

type componentLinker struct{}

func (c *componentLinker) interfaceAssertions() FileResultsEditor {
	return c
}

func (c *componentLinker) EditFileResults(allFileResults []*FileResults) {
	allSnippets := lo.FlatMap(allFileResults, func(fileResult *FileResults, idx int) []*Snippet {
		return fileResult.Snippets
	})

	setComponents(allSnippets)
}

func setComponents(s []*Snippet) {
	componentDeclarations := lo.Filter(s, func(snippet *Snippet, idx int) bool {
		return snippet.Type == ComponentDeclaration
	})
	snippetsByFile := lo.GroupBy(s, ByFile)
	componentDeclarationsByFile := lo.GroupBy(componentDeclarations, ByFile)

	filesWithUnknownComponent := lo.Without(lo.Keys(snippetsByFile), lo.Keys(componentDeclarationsByFile)...)
	snippetsWithUnknownComponent := lo.FlatMap(filesWithUnknownComponent, func(file string, idx int) []*Snippet {
		return snippetsByFile[file]
	})

	for _, snippet := range snippetsWithUnknownComponent {
		snippet.Component = "Unknown"
	}
	for fileName, componentDeclarationSnippets := range componentDeclarationsByFile {
		theComponent := componentDeclarationSnippets[0].Value

		snippets := snippetsByFile[fileName]
		for _, theSnippet := range snippets {
			theSnippet.Component = theComponent
		}
	}

}

func getConnections(snippetsByType SnippetGroup, snippetsByComponent SnippetGroup) []*ComponentConnection {
	var toReturn []*ComponentConnection
	from := snippetsByType[ComponentImport]
	for _, snippet := range from {
		if _, componentExistsInCodebase := snippetsByComponent[snippet.Value]; componentExistsInCodebase {
			toReturn = append(toReturn, &ComponentConnection{
				From: snippet.Component,
				To:   snippet.Value,
				File: snippet.File,
			})
		}
	}
	return toReturn
}
