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
		Name:           "component_cycles_elementary",
		CreateViewFunc: allComponentCyclesView,
	})

	settings.RegisterView(&core.ViewFactory{
		Name:           "component_cycles_largest",
		CreateViewFunc: largestComponentCycleView,
	})
	return nil
}
