package cycles

import (
	"github.com/archstats/archstats/core"
)

func Extension() core.Extension {
	return &extension{}
}

type extension struct {
}

func (v *extension) Init(settings core.Analyzer) error {
	settings.RegisterView(&core.ViewFactory{
		Name:           "all_component_cycles",
		CreateViewFunc: allComponentCyclesView,
	})

	settings.RegisterView(&core.ViewFactory{
		Name:           "largest_component_cycle",
		CreateViewFunc: largestComponentCycleView,
	})
	return nil
}
