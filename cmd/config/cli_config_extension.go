package config

import (
	"github.com/archstats/archstats/core"
	"github.com/spf13/cobra"
)

type ArgumentType int

const (
	String ArgumentType = iota
	Int
	Bool
	StringSlice
)

type ArgumentConfig struct {
	// Description of the argument
	Description string
	// Required indicates if the argument is required
	Required bool
	// Default value of the argument
	Default interface{}
	// Type of the argument
	Type ArgumentType
}
type Arguments map[string]*ArgumentConfig

type CLIConfiguredExtension struct {
	// Name of the extension
	Name string
	// Description of the extension
	Description string
	// Arguments of the extension
	Arguments Arguments

	Initializer func(config *cobra.Command) (core.Extension, error)

	DiscoveryTrigger func(ctx *DiscoveryContext) bool
}
