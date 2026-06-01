package declbased

import (
	"fmt"
	"github.com/archstats/archstats/cmd/config"
	"github.com/archstats/archstats/core"
	"github.com/spf13/cobra"
)

const (
	ComponentStrategy = "component-strategy"
)

func CLIExtension() *config.CLIConfiguredExtension {
	return &config.CLIConfiguredExtension{
		Name:        "declbased",
		Description: "Component linker based on code declarations or directory layouts",
		Arguments: config.Arguments{
			ComponentStrategy: {
				Default:     "declared",
				Description: "Component resolution strategy: declared (only declarations, fallback to Unknown), directory (every directory is a component), or fallback (declarations if present, fallback to directory)",
				Required:    false,
				Type:        config.String,
			},
		},
		Initializer: Init,
	}
}

func Init(command *cobra.Command) (core.Extension, error) {
	strategy, err := command.Flags().GetString(ComponentStrategy)
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
	settings.RegisterFileResultsEditor(&componentLinker{Strategy: r.Strategy})
	return nil
}
