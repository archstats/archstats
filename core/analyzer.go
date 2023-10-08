package core

import "github.com/archstats/archstats/core/definitions"

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
		accumulator: &accumulatorIndex{
			AccumulateFunctions: make(map[string]StatAccumulatorFunction),
		}}
}

type analyzer struct {
	rootPath           string
	extensions         []Extension
	views              map[string]*ViewFactory
	accumulator        *accumulatorIndex
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
	analyzer.accumulator.AccumulateFunctions[statType] = merger
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

	// Edit file results
	// Used to set the component and directory of a snippet
	for _, editor := range analyzer.fileResultsEditors {
		editor.EditFileResults(fileResults)
	}

	// Aggregate Snippets and Stats into Results
	results := aggregateSnippetsAndStatsIntoResults(analyzer, fileResults)

	// Edit results after they've been aggregated

	for _, editor := range analyzer.resultsEditors {
		editor.EditResults(results)
	}

	return results, nil
}
