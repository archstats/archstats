package required

import (
	"github.com/RyanSusana/archstats/core"
	"github.com/RyanSusana/archstats/core/file"
	"github.com/samber/lo"
)

type componentLinker struct{}

func (c *componentLinker) Init(settings core.Analyzer) error {
	return nil
}

func (c *componentLinker) interfaceAssertions() core.FileResultsEditor {
	return c
}

func (c *componentLinker) EditFileResults(allFileResults []*file.Results) {
	allSnippets := lo.FlatMap(allFileResults, func(fileResult *file.Results, idx int) []*file.Snippet {
		return fileResult.Snippets
	})

	setComponents(allSnippets)
}

func setComponents(s []*file.Snippet) {
	componentDeclarations := lo.Filter(s, func(snippet *file.Snippet, idx int) bool {
		return snippet.Type == file.ComponentDeclaration
	})
	snippetsByFile := lo.GroupBy(s, file.ByFile)
	componentDeclarationsByFile := lo.GroupBy(componentDeclarations, file.ByFile)

	filesWithUnknownComponent := lo.Without(lo.Keys(snippetsByFile), lo.Keys(componentDeclarationsByFile)...)
	snippetsWithUnknownComponent := lo.FlatMap(filesWithUnknownComponent, func(file string, idx int) []*file.Snippet {
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
