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
		Name:        "summary",
		Description: "A summary of the codebase",
		Create:      summaryView,
	})

	settings.RegisterView(&analysis.ViewFactory{
		Name:        "components",
		Description: "All components in the codebase with stats per component",
		Create:      componentView,
	})

	settings.RegisterView(&analysis.ViewFactory{
		Name:        "files",
		Description: "All files in the codebase with stats per file",
		Create:      fileView,
	})

	settings.RegisterView(&analysis.ViewFactory{
		Name:        "directories",
		Description: "All directories in the codebase with stats per directory",
		Create:      directoryView,
	})

	settings.RegisterView(&analysis.ViewFactory{
		Name:        "snippets",
		Description: "All snippets in the codebase",
		Create:      snippetsView,
	})

	settings.RegisterView(&analysis.ViewFactory{
		Name:        "component_connections_direct",
		Description: "All connections between components. Use this to build a component graph.",
		Create:      componentConnectionsView,
	})

	settings.RegisterView(&analysis.ViewFactory{
		Name:        "component_connections_indirect",
		Description: "All indirect connections between components.",
		Create:      componentConnectionsIndirectView,
	})

	settings.RegisterView(&analysis.ViewFactory{
		Name:        "component_connections_furthest",
		Description: "The furthest component from any given component.",
		Create:      componentConnectionsFurthestView,
	})

	settings.RegisterView(&analysis.ViewFactory{
		Name:        "strongly_connected_component_groups",
		Description: "Strongly connected component groups",
		Create:      stronglyConnectedComponentGroupsView,
	})

	settings.RegisterView(&analysis.ViewFactory{
		Name:        "all_component_cycles",
		Description: "All component cycles. This can be VERY expensive to calculate and store",
		Create:      componentCyclesView,
	})

	settings.RegisterView(&analysis.ViewFactory{
		Name:        "largest_component_cycle",
		Description: "The largest component cycle. This can be VERY expensive to calculate.",
		Create:      largestComponentCycleView,
	})
	return nil
}
