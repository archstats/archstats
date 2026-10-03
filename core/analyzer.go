package core

import (
	"encoding/json"
	"github.com/archstats/archstats/core/definitions"
	"github.com/archstats/archstats/core/stats"
	"github.com/archstats/archstats/core/walker"
	"github.com/rs/zerolog/log"
	"sort"
	"strconv"
	"strings"
)

type Analyzer interface {
	Analyze() (*Results, error)
	RootPath() string
	// IgnorePatterns are the configured root-level exclusions.
	IgnorePatterns() []string

	AddDefinition(definition *definitions.Definition)
	RegisterStatAccumulator(statType string, merger stats.StatAccumulatorFunction)
	RegisterView(viewFactory *ViewFactory)
	RegisterFileAnalyzer(analyzer FileAnalyzer)
	RegisterFileResultsEditor(editor FileResultsEditor)
	RegisterResultsEditor(editor ResultsEditor)
}

func New(config *Config) Analyzer {
	return &analyzer{rootPath: config.RootPath, extensions: config.Extensions, ignorePatterns: config.IgnorePatterns,
		views:       map[string]*ViewFactory{},
		definitions: map[string]*definitions.Definition{},
		accumulators: &stats.StatAccumulator{
			AccumulateFunctions: make(map[string]stats.StatAccumulatorFunction),
		}}
}

type analyzer struct {
	rootPath           string
	ignorePatterns     []string
	extensions         []Extension
	views              map[string]*ViewFactory
	accumulators       *stats.StatAccumulator
	fileAnalyzers      []FileAnalyzer
	fileResultsEditors []FileResultsEditor
	resultsEditors     []ResultsEditor
	definitions        map[string]*definitions.Definition
}

func (analyzer *analyzer) AddDefinition(definition *definitions.Definition) {
	analyzer.definitions[definition.Id] = definition
}

func (analyzer *analyzer) typeAssertion() Analyzer {
	return analyzer
}

func (analyzer *analyzer) RegisterView(factory *ViewFactory) {
	analyzer.views[factory.Name] = factory
}

func (analyzer *analyzer) RegisterFileAnalyzer(fileAnalyzer FileAnalyzer) {
	analyzer.fileAnalyzers = append(analyzer.fileAnalyzers, fileAnalyzer)
}

func (analyzer *analyzer) RegisterFileResultsEditor(editor FileResultsEditor) {
	analyzer.fileResultsEditors = append(analyzer.fileResultsEditors, editor)
}

func (analyzer *analyzer) RegisterResultsEditor(editor ResultsEditor) {
	analyzer.resultsEditors = append(analyzer.resultsEditors, editor)
}

func (analyzer *analyzer) RootPath() string {
	return analyzer.rootPath
}

func (analyzer *analyzer) IgnorePatterns() []string {
	return analyzer.ignorePatterns
}

func (analyzer *analyzer) RegisterStatAccumulator(statType string, merger stats.StatAccumulatorFunction) {
	analyzer.accumulators.AccumulateFunctions[statType] = merger
}

// Analyze analyzes the given root directory and returns the results.
func (analyzer *analyzer) Analyze() (*Results, error) {

	// Initialize extensions
	for _, extension := range analyzer.extensions {
		err := extension.Init(analyzer)
		if err != nil {
			return nil, err
		}
	}

	// Get Snippets and Stats from the files
	fileResults, report, err := getAllFileResults(analyzer.rootPath, analyzer.fileAnalyzers, walker.Options{
		IgnorePatterns: analyzer.ignorePatterns,
		Claims:         claimsOf(analyzer.fileAnalyzers),
	})
	if err != nil {
		return nil, err
	}
	log.Debug().Msgf("Finished collecting individual file results")

	// Edit file results
	// Used to set the component and directory of a snippet
	for _, editor := range analyzer.fileResultsEditors {
		editor.EditFileResults(fileResults)
	}

	log.Debug().Msgf("Finished editing file results")

	// Aggregate Snippets and Stats into Results
	results := aggregateSnippetsAndStatsIntoResults(analyzer, fileResults)
	// What the walker left out, so a snapshot can say why a folder is absent.
	if patterns := normalizedPatterns(analyzer.ignorePatterns); patterns != "" {
		results.SetSnapshotInfo("ignore_globs", patterns)
	}
	if report != nil {
		results.SkippedFiles = report.Skipped
		ignored := report.Ignored
		results.SetSnapshotInfo("walker_ignored_files", strconv.Itoa(ignored.Files))
		results.SetSnapshotInfo("walker_ignored_dirs", strconv.Itoa(ignored.Dirs))
		if top, err := json.Marshal(ignored.Top); err == nil {
			results.SetSnapshotInfo("walker_ignored_top", string(top))
		}
	}
	log.Debug().Msgf("Finished aggregating snippets and stats into results")

	// Edit results after they've been aggregated

	for _, editor := range analyzer.resultsEditors {
		editor.EditResults(results)
	}
	log.Debug().Msgf("Finished editing results")

	return results, nil
}

// claimsOf reports whether any language pack among the analyzers reads a
// path as source.
func claimsOf(analyzers []FileAnalyzer) func(path string) bool {
	var claimers []SourceClaimer
	for _, a := range analyzers {
		if c, ok := a.(SourceClaimer); ok {
			claimers = append(claimers, c)
		}
	}
	if len(claimers) == 0 {
		return nil
	}
	return func(path string) bool {
		for _, c := range claimers {
			if c.ClaimsFile(path) {
				return true
			}
		}
		return false
	}
}

// normalizedPatterns is the patterns as a snapshot records them: trimmed,
// comments and blanks dropped, sorted, one per line. Two scans with the same
// exclusions record the same text, whatever order they were given in.
func normalizedPatterns(patterns []string) string {
	var out []string
	for _, p := range patterns {
		if t := strings.TrimSpace(p); t != "" && !strings.HasPrefix(t, "#") {
			out = append(out, t)
		}
	}
	sort.Strings(out)
	return strings.Join(out, "\n")
}
