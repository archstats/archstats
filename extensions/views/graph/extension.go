package graph

import "github.com/RyanSusana/archstats/analysis"

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
		"all_component_cycles":               ComponentCyclesView,
		"largest_component_cycle":            LargestComponentCycleView,
		"strongly_connected_components_view": StronglyConnectedComponentGroupsView,
	}
}
