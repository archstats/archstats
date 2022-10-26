package cycles

import (
	"github.com/RyanSusana/archstats/analysis"
)

func Extension() analysis.Extension {
	return &extension{}
}

type extension struct {
}

func (v *extension) Init(settings analysis.Analyzer) error {
	for s, function := range getViewFactories() {
		settings.RegisterView(s, function)
	}
	return nil
}

func getViewFactories() map[string]analysis.ViewFactoryFunction {
	return map[string]analysis.ViewFactoryFunction{
		"all_component_cycles":    componentCyclesView,
		"largest_component_cycle": largestComponentCycleView,
	}
}
