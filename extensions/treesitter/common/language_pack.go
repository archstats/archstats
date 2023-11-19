package common

import (
	"context"
	"github.com/archstats/archstats/core/file"
	"github.com/gobwas/glob"
	"github.com/samber/lo"
	sitter "github.com/smacker/go-tree-sitter"
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
		newQuery, err := sitter.NewQuery([]byte(query), lp.Language)
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
	parser.SetLanguage(lp.Language)
	tree, err := parser.ParseCtx(context.Background(), nil, content)
	if err != nil {
		panic(err)
	}
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
	cursor.Exec(query, ctx.RootNode())
	for {
		m, ok := cursor.NextMatch()
		if !ok {
			break
		}
		for _, capture := range m.Captures {
			startByte := capture.Node.StartByte()
			endByte := capture.Node.EndByte()
			snippets = append(snippets, &file.Snippet{
				File:  filePath,
				Type:  query.CaptureNameForId(capture.Index),
				Value: capture.Node.Content(content),
				Begin: pointToPosition(startByte, capture.Node.StartPoint()),
				End:   pointToPosition(endByte, capture.Node.EndPoint()),
			})

		}
	}
	return snippets
}

func pointToPosition(offset uint32, position sitter.Point) *file.Position {
	return &file.Position{
		Offset:     int(offset),
		Line:       int(position.Row),
		CharInLine: int(position.Column),
	}
}
