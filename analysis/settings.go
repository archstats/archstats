package analysis

func New(rootPath string, extensions []Extension) *analyzer {
	return &analyzer{rootPath: rootPath, extensions: extensions,
		views: map[string]*ViewFactory{},
		accumulator: &accumulator{
			AccumulateFunctions: make(map[string]StatAccumulateFunction),
		}}
}

type Analyzer interface {
	RootPath() string
	RegisterStatAccumulator(statType string, merger StatAccumulateFunction)
	RegisterView(viewFactory *ViewFactory)
	RegisterFileAnalyzer(analyzer FileAnalyzer)
	RegisterFileResultsEditor(editor FileResultsEditor)
	RegisterResultsEditor(editor ResultsEditor)
}
type analyzer struct {
	rootPath           string
	extensions         []Extension
	views              map[string]*ViewFactory
	accumulator        *accumulator
	fileAnalyzers      []FileAnalyzer
	fileResultsEditors []FileResultsEditor
	resultsEditors     []ResultsEditor
}

func (a *analyzer) typeAssertion() Analyzer {
	return a
}

func (a *analyzer) RegisterView(factory *ViewFactory) {
	a.views[factory.Name] = factory
}

func (a *analyzer) RegisterFileAnalyzer(analyzer FileAnalyzer) {
	a.fileAnalyzers = append(a.fileAnalyzers, analyzer)
}

func (a *analyzer) RegisterFileResultsEditor(editor FileResultsEditor) {
	a.fileResultsEditors = append(a.fileResultsEditors, editor)
}

func (a *analyzer) RegisterResultsEditor(editor ResultsEditor) {
	a.resultsEditors = append(a.resultsEditors, editor)
}

func (a *analyzer) RootPath() string {
	return a.rootPath
}

func (a *analyzer) RegisterStatAccumulator(statType string, merger StatAccumulateFunction) {
	a.accumulator.AccumulateFunctions[statType] = merger
}

type Extension interface {
	Init(settings Analyzer) error
}

type FileResultsEditor interface {
	EditFileResults(all []*FileResults)
}

type ResultsEditor interface {
	EditResults(results *Results)
}
