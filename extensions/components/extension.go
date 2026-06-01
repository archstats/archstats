package components

import (
	"embed"
	"github.com/archstats/archstats/core"
	"github.com/archstats/archstats/core/definitions"
)

func Extension() core.Extension {
	return &extension{}
}

type extension struct{}

//go:embed definitions/**
var componentDefs embed.FS

func (extension) Init(settings core.Analyzer) error {
	loadedDefs, err := definitions.LoadYamlFiles(componentDefs)
	if err != nil {
		return err
	}
	for _, def := range loadedDefs {
		settings.AddDefinition(def)
	}

	settings.RegisterView(&core.ViewFactory{
		Name:           "components",
		CreateViewFunc: MainView,
	})

	settings.RegisterView(&core.ViewFactory{
		Name:           "component_connections_direct",
		CreateViewFunc: ConnectionsView,
	})

	settings.RegisterView(&core.ViewFactory{
		Name:           "component_connections_indirect",
		CreateViewFunc: ConnectionsIndirectView,
	})

	settings.RegisterView(&core.ViewFactory{
		Name:           "component_connections_furthest",
		CreateViewFunc: ConnectionsFurthestView,
	})

	settings.RegisterView(&core.ViewFactory{
		Name:           "component_cycles_shortest",
		CreateViewFunc: ShortestCyclesView,
	})

	settings.RegisterView(&core.ViewFactory{
		Name:           "component_strongly_connected_groups",
		CreateViewFunc: StronglyConnectedView,
	})
	settings.RegisterView(&core.ViewFactory{
		Name:           "component_communities",
		CreateViewFunc: CommunitiesView,
	})
	return nil
}
