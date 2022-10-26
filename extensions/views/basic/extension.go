package basic

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
		"summary":                            summaryView,
		"components":                         componentView,
		"files":                              fileView,
		"directories":                        directoryView,
		"snippets":                           snippetsView,
		"component_connections_direct":       componentConnectionsView,
		"component_connections_indirect":     componentDistanceView,
		"strongly_connected_components_view": stronglyConnectedComponentGroupsView,
	}
}
