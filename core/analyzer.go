package core

import (
	"encoding/json"
	"sort"
	"strconv"
	"strings"

	"github.com/archstats/archstats/core/component"
	"github.com/archstats/archstats/core/definitions"
	"github.com/archstats/archstats/core/file"
	"github.com/archstats/archstats/core/module"
	"github.com/archstats/archstats/core/stats"
	"github.com/archstats/archstats/core/walker"
	"github.com/rs/zerolog/log"
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
	// RegisterModuleReader adds a reader for one ecosystem's manifests. A
	// language pack registers the readers for its own build tools; two packs
	// registering the same Kind (Java and Kotlin both read Gradle) register
	// it once.
	RegisterModuleReader(reader module.Reader)
	// RegisterConventions adds an ecosystem's or a domain's conventions for
	// what a file is: a test, generated, someone else's, a pipeline. Two
	// registrations of one Name register it once.
	RegisterConventions(conventions file.Conventions)
	// Files is the classifier holding every registered convention. Complete
	// once every extension's Init has run, so ask for it in AnalyzeFile or
	// later.
	Files() *file.Classifier
	// RegisterImportResolver adds a language's reading of its own imports,
	// for the component linker. Two registrations of one Name register it
	// once.
	RegisterImportResolver(resolver component.ImportResolver)
	// ImportResolvers is every registered resolver, in registration order.
	// Complete once every extension's Init has run.
	ImportResolvers() []component.ImportResolver
	// Modules is the module map, read from the manifests among the files the
	// walker found. Nil until the walk is done: a file results editor may
	// read it, an Init may not.
	Modules() *module.Map
}

func New(config *Config) Analyzer {
	return &analyzer{rootPath: config.RootPath, extensions: config.Extensions, ignorePatterns: config.IgnorePatterns,
		views:       map[string]*ViewFactory{},
		files:       file.NewClassifier(),
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
	moduleReaders      []module.Reader
	modules            *module.Map
	files              *file.Classifier
	importResolvers    []component.ImportResolver
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

func (analyzer *analyzer) RegisterModuleReader(reader module.Reader) {
	for _, r := range analyzer.moduleReaders {
		if r.Kind() == reader.Kind() {
			return
		}
	}
	analyzer.moduleReaders = append(analyzer.moduleReaders, reader)
}

func (analyzer *analyzer) RegisterConventions(conventions file.Conventions) {
	analyzer.files.Add(conventions)
}

func (analyzer *analyzer) Files() *file.Classifier {
	return analyzer.files
}

func (analyzer *analyzer) RegisterImportResolver(resolver component.ImportResolver) {
	for _, r := range analyzer.importResolvers {
		if r.Name() == resolver.Name() {
			return
		}
	}
	analyzer.importResolvers = append(analyzer.importResolvers, resolver)
}

func (analyzer *analyzer) ImportResolvers() []component.ImportResolver {
	return analyzer.importResolvers
}

func (analyzer *analyzer) Modules() *module.Map {
	return analyzer.modules
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
	fileResults, report, err := getAllFileResults(analyzer.rootPath, analyzer.fileAnalyzers, analyzer.files, walker.Options{
		IgnorePatterns: analyzer.ignorePatterns,
		Claims:         claimsOf(analyzer.fileAnalyzers),
	})
	if err != nil {
		return nil, err
	}
	log.Debug().Msgf("Finished collecting individual file results")

	// Modules are read from the manifests among the files the walker found,
	// so .gitignore and .archstatsignore are honoured for free (ADR 0012),
	// and before the editors run, so a linker can ask which module owns a
	// file.
	names := make([]string, len(fileResults))
	for i, fr := range fileResults {
		names[i] = fr.Name
	}
	analyzer.modules = module.ReadFrom(analyzer.rootPath, names, analyzer.moduleReaders)

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
