package basic

import (
	"embed"
	"github.com/RyanSusana/archstats/analysis"
	"github.com/RyanSusana/archstats/analysis/definitions"
)

func Extension() analysis.Extension {
	return &extension{}
}

type extension struct {
}

//go:embed definitions/**
var defs embed.FS

func (v *extension) Init(settings analysis.Analyzer) error {

	defs, err := definitions.LoadYamlFiles(defs)
	if err != nil {
		return err
	}

	for _, definition := range defs {
		settings.AddDefinition(definition)
	}

	settings.RegisterView(&analysis.ViewFactory{
		Name:           "definitions",
		CreateViewFunc: definitionsView,
	})

	settings.RegisterView(&analysis.ViewFactory{
		Name:           "summary",
		CreateViewFunc: summaryView,
	})

	settings.RegisterView(&analysis.ViewFactory{
		Name:           "components",
		CreateViewFunc: componentView,
	})

	settings.RegisterView(&analysis.ViewFactory{
		Name:           "files",
		CreateViewFunc: fileView,
	})

	settings.RegisterView(&analysis.ViewFactory{
		Name:           "directories",
		CreateViewFunc: directoryView,
	})

	settings.RegisterView(&analysis.ViewFactory{
		Name:           "snippets",
		CreateViewFunc: snippetsView,
	})

	settings.RegisterView(&analysis.ViewFactory{
		Name:           "component_connections_direct",
		CreateViewFunc: componentConnectionsView,
	})

	settings.RegisterView(&analysis.ViewFactory{
		Name:           "component_connections_indirect",
		CreateViewFunc: componentConnectionsIndirectView,
	})

	settings.RegisterView(&analysis.ViewFactory{
		Name:           "component_connections_furthest",
		CreateViewFunc: componentConnectionsFurthestView,
	})

	settings.RegisterView(&analysis.ViewFactory{
		Name:           "strongly_connected_component_groups",
		CreateViewFunc: stronglyConnectedComponentGroupsView,
	})

	return nil
}
