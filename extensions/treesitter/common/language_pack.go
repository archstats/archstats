package common

import (
	"context"
	"errors"
	"github.com/archstats/archstats/core/file"
	"github.com/gobwas/glob"
	"github.com/rs/zerolog/log"
	sitter "github.com/tree-sitter/go-tree-sitter"
	"strings"
	"time"
)

type ComponentResolutionFunc func(r *file.Results) string

type LanguagePack struct {
	FileGlob            glob.Glob
	Language            *sitter.Language
	QueriesForStats     []*sitter.Query
	QueriesForSnippets  []*sitter.Query
	QueriesForCounts    []*sitter.Query
	ComponentResolution ComponentResolutionFunc
	SnippetTransformers map[string]func(*file.Snippet) *file.Snippet
	complexity          *complexityKinds
}

type LanguagePackTemplate struct {
	FileGlob            string
	Language            *sitter.Language
	QueriesForStats     []string
	ComponentResolution ComponentResolutionFunc
	SnippetTransformers map[string]func(*file.Snippet) *file.Snippet
	QueriesForSnippets  []string
	// QueriesForCounts feed stats but are not kept as snippets: each one
	// counts what a snippet query already records at the same place (every
	// java__method_declarations row had a java__method__declaration twin),
	// so storing both doubled those rows and told a reader nothing.
	QueriesForCounts []string
	// Complexity, when set, measures every function on the tree the queries
	// ran on: code lines, cognitive complexity, the complex code health
	// counts against. See complexity.go.
	Complexity *Complexity
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

	queriesForStats, err := stringsToQueries(lp.Language, template.QueriesForStats)
	if err != nil {
		return nil, err
	}
	queriesForSnippets, err := stringsToQueries(lp.Language, template.QueriesForSnippets)
	if err != nil {
		return nil, err
	}

	queriesForCounts, err := stringsToQueries(lp.Language, template.QueriesForCounts)
	if err != nil {
		return nil, err
	}

	lp.QueriesForStats = queriesForStats
	lp.QueriesForSnippets = queriesForSnippets
	lp.QueriesForCounts = queriesForCounts
	lp.SnippetTransformers = template.SnippetTransformers
	lp.complexity = template.Complexity.compile()
	lp.ComponentResolution = GetComponentResolutionFromTemplate(template)
	return lp, nil
}

func stringsToQueries(language *sitter.Language, queries []string) ([]*sitter.Query, error) {
	var queriesForStats []*sitter.Query
	for _, query := range queries {
		newQuery, err := sitter.NewQuery(language, query)
		if err != nil {
			return nil, err
		}
		queriesForStats = append(queriesForStats, newQuery)
	}
	return queriesForStats, nil
}

func GetComponentResolutionFromTemplate(template *LanguagePackTemplate) ComponentResolutionFunc {
	if template.ComponentResolution != nil {
		return template.ComponentResolution
	}
	for _, query := range template.QueriesForStats {
		if strings.Contains(query, "modularity__component__declarations") {
			return DeclarationBasedComponentResolution
		}
	}
	return DirectoryBasedComponentResolution
}

// ClaimsFile reports whether this language reads the path as source.
func (lp *LanguagePack) ClaimsFile(path string) bool {
	return lp.FileGlob.Match(path)
}

// AnalyzeFile analyzes a file and returns the results.
// Snippets (which are just tree-sitter capture groups) starting with an underscore are only used for stats, and are not recorded as snippets.
func (lp *LanguagePack) AnalyzeFile(f file.File) *file.Results {
	return lp.AnalyzeFileContent(f.Path(), f.Content())
}

func (lp *LanguagePack) transformSnippets(snippets []*file.Snippet) []*file.Snippet {
	if len(lp.SnippetTransformers) == 0 {
		return snippets
	}
	for i, snippet := range snippets {
		if transformer, has := lp.SnippetTransformers[snippet.Type]; has {
			snippets[i] = transformer(snippet)
		}
	}
	return snippets
}

func (lp *LanguagePack) AnalyzeFileContent(path string, content []byte) *file.Results {
	if !lp.FileGlob.Match(path) {
		return nil
	}
	return lp.AnalyzeContent(path, content)
}

// AnalyzeContent reads content as this pack's language whatever the path
// says, for a file that carries the language inside another: the script
// blocks of a .vue or .svelte component.
func (lp *LanguagePack) AnalyzeContent(path string, content []byte) *file.Results {
	start := time.Now()
	tree, err := parse(content, lp.Language)
	if err != nil {
		log.Warn().Err(err).Msgf("[treesitter] Skipping file %s", path)
		return nil
	}
	defer tree.Close()
	rawSnippetsForStats := runQueries(path, tree, content, lp.QueriesForStats)
	rawSnippetsForSnippets := runQueries(path, tree, content, lp.QueriesForSnippets)
	rawSnippetsForCounts := runQueries(path, tree, content, lp.QueriesForCounts)
	snippetsForStats := lp.transformSnippets(rawSnippetsForStats)
	snippetsForSnippets := lp.transformSnippets(rawSnippetsForSnippets)
	snippetsForCounts := lp.transformSnippets(rawSnippetsForCounts)
	allSnippets := append(snippetsForStats, snippetsForSnippets...)
	results := &file.Results{
		Snippets: allSnippets,
		Stats:    file.SnippetsToStats(append(append([]*file.Snippet{}, snippetsForStats...), snippetsForCounts...)),
	}
	if lp.complexity != nil {
		records, functions := lp.complexity.measure(tree.RootNode(), content)
		results.Stats = append(results.Stats, records...)
		results.Functions = functions
	}
	component := lp.ComponentResolution(results)
	results.Component = component
	for _, snippet := range results.Snippets {
		snippet.Component = component
	}
	log.Debug().Msgf("[treesitter] Analyzed %s with treesitter in %s", path, time.Since(start))
	return results
}

// parse reads the file once for all three query sets. The binding keeps
// parsers, trees and cursors in C memory with no finalizer, so each one is
// closed here; left open, every file parsed stayed in memory for the scan.
func parse(content []byte, language *sitter.Language) (*sitter.Tree, error) {
	parser := sitter.NewParser()
	defer parser.Close()
	if err := parser.SetLanguage(language); err != nil {
		return nil, err
	}
	tree := parser.ParseCtx(context.Background(), content, nil)
	if tree == nil {
		return nil, errors.New("tree-sitter returned no tree")
	}
	return tree, nil
}

func runQueries(filePath string, tree *sitter.Tree, content []byte, queries []*sitter.Query) []*file.Snippet {
	var snippetsToReturn []*file.Snippet
	for _, qr := range queries {
		snippetsToReturn = append(snippetsToReturn, execQuery(filePath, qr, tree, content)...)
	}
	return snippetsToReturn
}

func execQuery(filePath string, query *sitter.Query, ctx *sitter.Tree, content []byte) []*file.Snippet {
	var snippets []*file.Snippet
	cursor := sitter.NewQueryCursor()
	defer cursor.Close()

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
