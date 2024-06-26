package basic

import (
	"embed"
	"github.com/archstats/archstats/core"
	"github.com/archstats/archstats/core/definitions"
)

func Extension() core.Extension {
	return &extension{}
}

type extension struct {
}

//go:embed definitions/**
var defs embed.FS

func (v *extension) Init(settings core.Analyzer) error {

	defs, err := definitions.LoadYamlFiles(defs)
	if err != nil {
		return err
	}

	for _, definition := range defs {
		settings.AddDefinition(definition)
	}

	settings.RegisterView(&core.ViewFactory{
		Name:           "definitions",
		CreateViewFunc: definitionsView,
	})

	settings.RegisterView(&core.ViewFactory{
		Name:           "summary",
		CreateViewFunc: summaryView,
	})

	settings.RegisterView(&core.ViewFactory{
		Name:           "files",
		CreateViewFunc: fileView,
	})

	settings.RegisterView(&core.ViewFactory{
		Name:           "directories",
		CreateViewFunc: directoryView,
	})

	settings.RegisterView(&core.ViewFactory{
		Name:           "snippets",
		CreateViewFunc: snippetsView,
	})

	return nil
}
