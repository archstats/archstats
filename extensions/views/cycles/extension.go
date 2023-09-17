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
		Description:    "All component cycles. This can be VERY expensive to calculate and store",
		CreateViewFunc: allComponentCyclesView,
	})

	settings.RegisterView(&analysis.ViewFactory{
		Name:           "largest_component_cycle",
		Description:    "The largest component cycle. This can be VERY expensive to calculate.",
		CreateViewFunc: largestComponentCycleView,
	})
	return nil
}
