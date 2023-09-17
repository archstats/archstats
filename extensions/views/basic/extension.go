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
	settings.RegisterView(&analysis.ViewFactory{
		Name:           "summary",
		Description:    "A summary of the codebase",
		CreateViewFunc: summaryView,
	})

	settings.RegisterView(&analysis.ViewFactory{
		Name:           "components",
		Description:    "All components in the codebase with stats per component",
		CreateViewFunc: componentView,
	})

	settings.RegisterView(&analysis.ViewFactory{
		Name:           "files",
		Description:    "All files in the codebase with stats per file",
		CreateViewFunc: fileView,
	})

	settings.RegisterView(&analysis.ViewFactory{
		Name:           "directories",
		Description:    "All directories in the codebase with stats per directory",
		CreateViewFunc: directoryView,
	})

	settings.RegisterView(&analysis.ViewFactory{
		Name:           "snippets",
		Description:    "All snippets in the codebase",
		CreateViewFunc: snippetsView,
	})

	settings.RegisterView(&analysis.ViewFactory{
		Name:           "component_connections_direct",
		Description:    "All connections between components. Use this to build a component graph.",
		CreateViewFunc: componentConnectionsView,
	})

	settings.RegisterView(&analysis.ViewFactory{
		Name:           "component_connections_indirect",
		Description:    "All indirect connections between components.",
		CreateViewFunc: componentConnectionsIndirectView,
	})

	settings.RegisterView(&analysis.ViewFactory{
		Name:           "component_connections_furthest",
		Description:    "The furthest component from any given component.",
		CreateViewFunc: componentConnectionsFurthestView,
	})

	settings.RegisterView(&analysis.ViewFactory{
		Name:           "strongly_connected_component_groups",
		Description:    "Strongly connected component groups",
		CreateViewFunc: stronglyConnectedComponentGroupsView,
	})

	return nil
}
