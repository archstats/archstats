package common

import (
	"context"
	"github.com/archstats/archstats/core/file"
	"github.com/gobwas/glob"
	"github.com/samber/lo"
	sitter "github.com/tree-sitter/go-tree-sitter"
	"strings"
)

type ComponentResolutionFunc func(r *file.Results) string

type LanguagePack struct {
	FileGlob            glob.Glob
	Language            *sitter.Language
	Queries             []*sitter.Query
	ComponentResolution ComponentResolutionFunc
	SnippetTransformers map[string]func(*file.Snippet) *file.Snippet
}

type LanguagePackTemplate struct {
	FileGlob            string
	Language            *sitter.Language
	Queries             []string
	ComponentResolution ComponentResolutionFunc
	SnippetTransformers map[string]func(*file.Snippet) *file.Snippet
}

func PackFromTemplate(template *LanguagePackTemplate) (*LanguagePack, error) {
	lp := &LanguagePack{
		Language: template.Language,
	}
	g, err := glob.Compile(template.FileGlob)
	if err != nil {
		return nil, err
	}
	lp.FileGlob = g
	var queries []*sitter.Query
	for _, query := range template.Queries {
		newQuery, err := sitter.NewQuery(lp.Language, query)
		if err != nil {
			return nil, err
		}
		queries = append(queries, newQuery)
	}
	lp.Queries = queries
	lp.ComponentResolution = GetComponentResolutionFromTemplate(template)
	return lp, nil
}

func GetComponentResolutionFromTemplate(template *LanguagePackTemplate) ComponentResolutionFunc {
	if template.ComponentResolution != nil {
		return template.ComponentResolution
	}
	for _, query := range template.Queries {
		if strings.Contains(query, "modularity__component__declarations") {
			return DeclarationBasedComponentResolution
		}
	}
	return DirectoryBasedComponentResolution
}

// AnalyzeFile analyzes a file and returns the results.
// Snippets (which are just tree-sitter capture groups) starting with an underscore are only used for stats, and are not recorded as snippets.
func (lp *LanguagePack) AnalyzeFile(f file.File) *file.Results {
	return lp.AnalyzeFileContent(f.Path(), f.Content())
}

func (lp *LanguagePack) AnalyzeFileContent(path string, content []byte) *file.Results {
	if !lp.FileGlob.Match(path) {
		return nil
	}
	snippets := lp.analyzeFileContent(path, content)
	snippetsToRecord := lo.Filter(snippets, func(snippet *file.Snippet, idx int) bool {
		return !strings.HasPrefix("_", snippet.Type)
	})
	results := &file.Results{
		Snippets: snippetsToRecord,
		Stats:    file.SnippetsToStats(snippets),
	}
	component := lp.ComponentResolution(results)
	results.Component = component
	for _, snippet := range results.Snippets {
		snippet.Component = component
	}
	return results
}

func (lp *LanguagePack) analyzeFileContent(filePath string, content []byte) []*file.Snippet {
	parser := sitter.NewParser()

	err := parser.SetLanguage(lp.Language)
	if err != nil {
		panic(err)
	}
	tree := parser.ParseCtx(context.Background(), content, nil)
	var snippetsToReturn []*file.Snippet
	for _, qr := range lp.Queries {
		snippets := execQuery(filePath, qr, tree, content)
		snippetsToReturn = append(snippetsToReturn, snippets...)
	}
	return snippetsToReturn
}

func execQuery(filePath string, query *sitter.Query, ctx *sitter.Tree, content []byte) []*file.Snippet {
	var snippets []*file.Snippet
	cursor := sitter.NewQueryCursor()

	matches := cursor.Matches(query, ctx.RootNode(), content)

	captureNames := query.CaptureNames()

	for {
		m := matches.Next()
		if m == nil {
			break
		}

		if !m.SatisfiesTextPredicate(query, nil, nil, content) {
			continue
		}

		for _, capture := range m.Captures {

			node := capture.Node

			snippetType := captureNames[capture.Index]

			if strings.HasPrefix(snippetType, "_") {
				continue
			}
			startByte := node.StartByte()
			endByte := node.EndByte()
			snippets = append(snippets, &file.Snippet{
				File:  filePath,
				Type:  snippetType,
				Value: node.Utf8Text(content),
				Begin: pointToPosition(startByte, node.StartPosition()),
				End:   pointToPosition(endByte, node.EndPosition()),
			})

		}
	}
	return snippets
}

func pointToPosition(offset uint, position sitter.Point) *file.Position {
	return &file.Position{
		Offset: int(offset),

		// Tree-sitter uses 0-based indexing, so we add 1 to the row and column.
		Line:       int(position.Row) + 1,
		CharInLine: int(position.Column) + 1,
	}
}
