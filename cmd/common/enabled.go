package common

import (
	"github.com/archstats/archstats/cmd/config"
	"github.com/archstats/archstats/engine"
	"github.com/spf13/cobra"
)

// Optional and AlwaysEnabled are engine's; the names stay for callers.
func Optional() []*config.CLIConfiguredExtension      { return engine.Optional() }
func AlwaysEnabled() []*config.CLIConfiguredExtension { return engine.AlwaysEnabled() }

// GetEnabledExtensions is engine.Select with its arguments read from flags.
func GetEnabledExtensions(cmd *cobra.Command) ([]*config.CLIConfiguredExtension, error) {
	flags := GetCommonFlags(cmd)
	enabled, _, err := engine.Select(flags.WorkingDirectory, flags.Ignore, flags.Extensions)
	return enabled, err
}
