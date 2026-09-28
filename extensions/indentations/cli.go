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
				Default:     0,
				Description: "Spaces per indentation level. 0 reads it per file: Prettier config or .editorconfig, then the file's own indentation.",
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
	if indentationSize < 0 || indentationSize > 8 {
		return nil, fmt.Errorf("indentation size must be between 0 (detect) and 8")
	}
	return &Extension{SpacesInTab: indentationSize}, nil
}
