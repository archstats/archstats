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
	settings.RegisterView(&analysis.ViewFactory{
		Name:           "all_component_cycles",
		CreateViewFunc: allComponentCyclesView,
	})

	settings.RegisterView(&analysis.ViewFactory{
		Name:           "largest_component_cycle",
		CreateViewFunc: largestComponentCycleView,
	})
	return nil
}
