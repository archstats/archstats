package required

import (
	"github.com/RyanSusana/archstats/analysis"
	"github.com/samber/lo"
)

type componentLinker struct{}

func (c *componentLinker) Init(settings analysis.Analyzer) error {
	return nil
}

func (c *componentLinker) interfaceAssertions() analysis.FileResultsEditor {
	return c
}

func (c *componentLinker) EditFileResults(allFileResults []*analysis.FileResults) {
	allSnippets := lo.FlatMap(allFileResults, func(fileResult *analysis.FileResults, idx int) []*analysis.Snippet {
		return fileResult.Snippets
	})

	setComponents(allSnippets)
}

func setComponents(s []*analysis.Snippet) {
	componentDeclarations := lo.Filter(s, func(snippet *analysis.Snippet, idx int) bool {
		return snippet.Type == analysis.ComponentDeclaration
	})
	snippetsByFile := lo.GroupBy(s, analysis.ByFile)
	componentDeclarationsByFile := lo.GroupBy(componentDeclarations, analysis.ByFile)

	filesWithUnknownComponent := lo.Without(lo.Keys(snippetsByFile), lo.Keys(componentDeclarationsByFile)...)
	snippetsWithUnknownComponent := lo.FlatMap(filesWithUnknownComponent, func(file string, idx int) []*analysis.Snippet {
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
