package indentations

import (
	"fmt"
	"github.com/archstats/archstats/core"
	"github.com/archstats/archstats/core/plugin"
)

const (
	IndentationSize = "indentation-size"
)

func CLIExtension() *plugin.Descriptor {
	return &plugin.Descriptor{
		Name:        "indentations",
		Description: "Indentations extension",
		Arguments: plugin.Arguments{
			IndentationSize: {
				Default:     0,
				Description: "Spaces per indentation level. 0 reads it per file: Prettier config or .editorconfig, then the file's own indentation.",
				Required:    false,
				Type:        plugin.Int,
			},
		},
		Initializer: Init,
	}
}

func Init(settings plugin.Settings) (core.Extension, error) {
	indentationSize, err := settings.GetInt(IndentationSize)
	if err != nil {
		return nil, err
	}
	if indentationSize < 0 || indentationSize > 8 {
		return nil, fmt.Errorf("indentation size must be between 0 (detect) and 8")
	}
	return &Extension{SpacesInTab: indentationSize}, nil
}
