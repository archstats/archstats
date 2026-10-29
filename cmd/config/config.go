// Package config is what core/plugin was called when every extension was
// configured through a cobra command. The names stay so callers keep
// compiling; new code uses core/plugin.
package config

import (
	"github.com/archstats/archstats/core"
	"github.com/archstats/archstats/core/plugin"
)

type ArgumentType = plugin.ArgumentType

const (
	String      = plugin.String
	Int         = plugin.Int
	Bool        = plugin.Bool
	StringSlice = plugin.StringSlice
)

type ArgumentConfig = plugin.Argument
type Arguments = plugin.Arguments

// Deprecated: use plugin.Descriptor. Its Initializer takes plugin.Settings;
// pass cmd.Flags() where a *cobra.Command used to go.
type CLIConfiguredExtension = plugin.Descriptor

type DiscoveryContext = plugin.DiscoveryContext

// Deprecated: use plugin.Fixed.
func CreateEmptyCLIExtension(name string, extension core.Extension) *CLIConfiguredExtension {
	return plugin.Fixed(name, extension)
}
