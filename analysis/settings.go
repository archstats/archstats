package analysis

func NewSettings(rootPath string, extensions []Extension) *settings {
	return &settings{rootPath: rootPath, extensions: extensions, accumulator: &accumulator{
		AccumulateFunctions: make(map[string]StatAccumulateFunction),
	}}
}

type Settings interface {
	RootPath() string
	SetStatAccumulator(statType string, merger StatAccumulateFunction)
}
type settings struct {
	rootPath    string
	extensions  []Extension
	accumulator *accumulator
}

func (s *settings) RootPath() string {
	return s.rootPath
}

func (s *settings) SetStatAccumulator(statType string, merger StatAccumulateFunction) {
	s.accumulator.AccumulateFunctions[statType] = merger
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
