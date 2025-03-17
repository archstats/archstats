package config

import (
	"github.com/archstats/archstats/core"
	"github.com/spf13/cobra"
)

func CreateEmptyCLIExtension(name string, extension core.Extension) *CLIConfiguredExtension {
	return &CLIConfiguredExtension{
		Name:        name,
		Description: "",
		Arguments:   nil,
		Initializer: func(config *cobra.Command) (core.Extension, error) {
			return extension, nil
		},
	}
}
