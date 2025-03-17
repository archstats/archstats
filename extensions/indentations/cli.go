package indentations

import (
	"fmt"
	"github.com/archstats/archstats/cmd/config"
	"github.com/archstats/archstats/core"
	"github.com/spf13/cobra"
)

const (
	IndentationSize = "indentation-size"
)

func CLIExtension() *config.CLIConfiguredExtension {
	return &config.CLIConfiguredExtension{
		Name:        "indentations",
		Description: "Indentations extension",
		Arguments: config.Arguments{
			IndentationSize: {
				Default:     4,
				Description: "Indentation size, 4 or 2.",
				Required:    false,
				Type:        config.Int,
			},
		},
		Initializer: Init,
	}
}

func Init(command *cobra.Command) (core.Extension, error) {
	indentationSize, err := command.Flags().GetInt(IndentationSize)
	if err != nil {
		return nil, err
	}
	if indentationSize != 4 && indentationSize != 2 {
		return nil, fmt.Errorf("indentation size must be '4' or '2'")
	}
	return &Extension{SpacesInTab: indentationSize}, nil
}
