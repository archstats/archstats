package declbased

import (
	"fmt"
	"github.com/archstats/archstats/core"
	"github.com/archstats/archstats/core/plugin"
)

const (
	ComponentStrategy = "component-strategy"
)

func CLIExtension() *plugin.Descriptor {
	return &plugin.Descriptor{
		Name:        "declbased",
		Description: "Component linker based on code declarations or directory layouts",
		Arguments: plugin.Arguments{
			ComponentStrategy: {
				// A language pack already decides for itself how its files
				// resolve: those with a declarations query (Java, C#, Kotlin)
				// read the declaration, and those without one (Python,
				// JavaScript, TypeScript) use the directory, because in those
				// languages the directory IS the module. Defaulting to
				// "declared" overrode that and filed every file of a Python,
				// JS or TS project under a single "Unknown". "fallback" is
				// the same rule the packs state: measured on real projects it
				// is identical to "declared" wherever declarations exist.
				Default:     "fallback",
				Description: "Component resolution strategy: declared (only declarations, fallback to Unknown), directory (every directory is a component), or fallback (declarations if present, fallback to directory)",
				Required:    false,
				Type:        plugin.String,
			},
		},
		Initializer: Init,
	}
}

func Init(settings plugin.Settings) (core.Extension, error) {
	strategy, err := settings.GetString(ComponentStrategy)
	if err != nil {
		return nil, err
	}
	if strategy != "declared" && strategy != "directory" && strategy != "fallback" {
		return nil, fmt.Errorf("invalid component-strategy: '%s'. Must be one of: 'declared', 'directory', 'fallback'", strategy)
	}
	return &requiredExtensions{Strategy: strategy}, nil
}

func Extension() core.Extension {
	return &requiredExtensions{Strategy: "declared"}
}

type requiredExtensions struct {
	Strategy string
}

func (r *requiredExtensions) Init(settings core.Analyzer) error {
	// What the project calls its own packages, read from its manifests rather
	// than guessed at from directory names. The root is kept rather than the
	// map: the manifests are read from the files the walker found, so
	// .gitignore and .archstatsignore are honoured (ADR 0012).
	settings.RegisterFileResultsEditor(&componentLinker{
		Strategy: r.Strategy,
		root:     settings.RootPath(),
	})
	return nil
}
