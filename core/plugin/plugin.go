// Package plugin describes an extension the way a front end configures it:
// its name, the arguments it takes and their defaults, when to switch it on,
// and how to build it from settings.
//
// It used to live under cmd/config and hand every extension a
// *cobra.Command, so the desktop app built a throwaway command full of flags
// just to start a scan. Settings is the smaller thing an extension needs: the
// CLI passes its flag set, which already is one, and anything else passes
// Values.
package plugin

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/archstats/archstats/core"
)

type ArgumentType int

const (
	String ArgumentType = iota
	Int
	Bool
	StringSlice
)

// Argument is one setting an extension reads. Its name is its key in
// Arguments, and the CLI flag of the same name.
type Argument struct {
	Description string
	Required    bool
	// Default is a string, int, bool or []string, as Type says.
	Default interface{}
	Type    ArgumentType
}

type Arguments map[string]*Argument

// Settings is where an extension reads its arguments. *pflag.FlagSet, what
// cobra's Flags() returns, is one; Values is another.
type Settings interface {
	GetString(name string) (string, error)
	GetInt(name string) (int, error)
	GetBool(name string) (bool, error)
	GetStringSlice(name string) ([]string, error)
}

// Descriptor is an extension as something to configure and switch on.
type Descriptor struct {
	Name        string
	Description string
	Arguments   Arguments

	// Initializer builds a fresh extension for one scan. Extensions keep
	// per-scan state, so an instance is never shared between scans.
	Initializer func(settings Settings) (core.Extension, error)

	// DiscoveryTrigger says whether a workspace needs this extension. Nil
	// means it is only ever switched on by name.
	DiscoveryTrigger func(ctx *DiscoveryContext) bool
}

// Fixed describes an extension that takes no arguments.
func Fixed(name string, extension core.Extension) *Descriptor {
	return &Descriptor{
		Name: name,
		Initializer: func(Settings) (core.Extension, error) {
			return extension, nil
		},
	}
}

// DiscoveryContext is what a DiscoveryTrigger looks at: the workspace root
// and every file the walker would read.
type DiscoveryContext struct {
	RootDir string
	Files   []string
}

func (d *DiscoveryContext) HasFileExtension(ext string) bool {
	if !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}
	extLower := strings.ToLower(ext)
	for _, f := range d.Files {
		if strings.HasSuffix(strings.ToLower(f), extLower) {
			return true
		}
	}
	return false
}

func (d *DiscoveryContext) HasPath(path string) bool {
	_, err := os.Stat(filepath.Join(d.RootDir, path))
	return err == nil
}
