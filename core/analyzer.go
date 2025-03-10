package core

import (
	"github.com/archstats/archstats/core/definitions"
	"github.com/rs/zerolog/log"
)

type Analyzer interface {
	Analyze() (*Results, error)
	RootPath() string

	AddDefinition(definition *definitions.Definition)
	RegisterStatAccumulator(statType string, merger StatAccumulatorFunction)
	RegisterView(viewFactory *ViewFactory)
	RegisterFileAnalyzer(analyzer FileAnalyzer)
	RegisterFileResultsEditor(editor FileResultsEditor)
	RegisterResultsEditor(editor ResultsEditor)
}

func New(config *Config) Analyzer {
	return &analyzer{rootPath: config.RootPath, extensions: config.Extensions,
		views:       map[string]*ViewFactory{},
		definitions: map[string]*definitions.Definition{},
		accumulators: &accumulatorIndex{
			AccumulateFunctions: make(map[string]StatAccumulatorFunction),
		}}
}

type analyzer struct {
	rootPath           string
	extensions         []Extension
	views              map[string]*ViewFactory
	accumulators       *accumulatorIndex
	fileAnalyzers      []FileAnalyzer
	fileResultsEditors []FileResultsEditor
	resultsEditors     []ResultsEditor
	definitions        map[string]*definitions.Definition
}

func (analyzer *analyzer) AddDefinition(definition *definitions.Definition) {
	analyzer.definitions[definition.Name] = definition
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

func (analyzer *analyzer) RegisterStatAccumulator(statType string, merger StatAccumulatorFunction) {
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
	fileResults := getAllFileResults(analyzer.rootPath, analyzer.fileAnalyzers)
	log.Debug().Msgf("Finished collecting individual file results")

	// Edit file results
	// Used to set the component and directory of a snippet
	for _, editor := range analyzer.fileResultsEditors {
		editor.EditFileResults(fileResults)
	}

	log.Debug().Msgf("Finished editing file results")

	// Aggregate Snippets and Stats into Results
	results := aggregateSnippetsAndStatsIntoResults(analyzer, fileResults)
	log.Debug().Msgf("Finished aggregating snippets and stats into results")

	// Edit results after they've been aggregated

	for _, editor := range analyzer.resultsEditors {
		editor.EditResults(results)
	}
	log.Debug().Msgf("Finished editing results")

	return results, nil
}
