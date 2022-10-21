package analysis

func NewSettings(rootPath string, extensions []Extension) *settings {
	return &settings{rootPath: rootPath, extensions: extensions, merger: &merger{}}
}

type Settings interface {
	RootPath() string
	SetStatAccumulator(statType string, merger StatMergeFunction)
}
type settings struct {
	rootPath   string
	extensions []Extension
	merger     *merger
}

func (s *settings) RootPath() string {
	return s.rootPath
}

func (s *settings) SetStatAccumulator(statType string, merger StatMergeFunction) {
	s.merger.MergeFunctions[statType] = merger
}

type Extension interface{}

type Initializable interface {
	Init(settings Settings)
}

type FileResultsEditor interface {
	EditFileResults(all []*FileResults)
}

type ResultsEditor interface {
	EditResults(results *Results)
}
