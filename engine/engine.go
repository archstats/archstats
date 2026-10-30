// Package engine runs a scan. It is what the CLI and the desktop app both
// call: which extensions a workspace needs, built from settings, run over the
// workspace by core.
package engine

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/archstats/archstats/core"
	"github.com/archstats/archstats/core/file"
	"github.com/archstats/archstats/core/plugin"
	"github.com/archstats/archstats/core/walker"
	"github.com/archstats/archstats/version"
	"github.com/rs/zerolog/log"
)

// Options says what to scan and how.
type Options struct {
	// Root is the workspace folder.
	Root string
	// Ignore holds paths to leave out, in .gitignore syntax, on top of the
	// tree's own ignore files.
	Ignore []string
	// Extensions names the optional extensions to run. Empty means the ones
	// the workspace's files call for.
	Extensions []string
	// Settings holds the extensions' arguments. Nil means every argument at
	// its default; plugin.Values fills in the ones a map leaves out.
	Settings plugin.Settings
	// Extra extensions run after the configured ones.
	Extra []core.Extension
}

// Select is the extensions a scan of root runs: every always-enabled one,
// then the optional ones named, or when none are named the ones the
// workspace's files call for. Discovered lists the optional ones that were
// switched on by discovery.
func Select(root string, ignore []string, names []string) (enabled []*plugin.Descriptor, discovered []string, err error) {
	enabled = AlwaysEnabled()
	optional := Optional()

	if len(names) > 0 {
		always := map[string]bool{}
		for _, d := range enabled {
			always[d.Name] = true
		}
		byName := map[string]*plugin.Descriptor{}
		for _, d := range optional {
			byName[d.Name] = d
		}
		for _, name := range names {
			if always[name] {
				continue
			}
			d, ok := byName[name]
			if !ok {
				return nil, nil, fmt.Errorf("unknown extension: %s", name)
			}
			enabled = append(enabled, d)
		}
		return enabled, nil, nil
	}

	root, err = filepath.Abs(root)
	if err != nil {
		return nil, nil, err
	}
	files, err := walker.GetAllFiles(root, walker.Options{IgnorePatterns: ignore})
	if err != nil {
		return nil, nil, err
	}
	paths := make([]string, 0, len(files))
	for _, f := range files {
		paths = append(paths, f.Path())
	}
	ctx := &plugin.DiscoveryContext{RootDir: root, Files: paths}
	for _, d := range optional {
		if d.DiscoveryTrigger != nil && d.DiscoveryTrigger(ctx) {
			enabled = append(enabled, d)
			discovered = append(discovered, d.Name)
		}
	}
	if len(discovered) > 0 {
		log.Debug().Msgf("Auto-discovered extensions: %v", discovered)
	}
	return enabled, discovered, nil
}

// Analyze scans a workspace.
func Analyze(opts Options) (*core.Results, error) {
	root, err := filepath.Abs(opts.Root)
	if err != nil {
		return nil, err
	}
	descriptors, _, err := Select(root, opts.Ignore, opts.Extensions)
	if err != nil {
		return nil, err
	}
	settings := opts.Settings
	if settings == nil {
		settings = plugin.Values(nil, descriptors)
	}

	extensions := make([]core.Extension, 0, len(descriptors)+len(opts.Extra))
	names := make([]string, 0, len(descriptors))
	for _, d := range descriptors {
		ext, err := d.Initializer(settings)
		if err != nil {
			return nil, fmt.Errorf("extension %s: %w", d.Name, err)
		}
		extensions = append(extensions, ext)
		names = append(names, d.Name)
	}
	extensions = append(extensions, opts.Extra...)

	results, err := core.New(&core.Config{
		RootPath:       root,
		Extensions:     extensions,
		IgnorePatterns: opts.Ignore,
	}).Analyze()
	if err != nil {
		return nil, err
	}
	sort.Strings(names)
	results.SetSnapshotInfo("extensions", strings.Join(names, ","))
	results.SetSnapshotInfo("engine_version", version.Version())
	return results, nil
}

// Classifier holds the file conventions of every extension in the
// catalogue, for a tool that judges files outside a scan. A scan holds only
// the conventions of the extensions it runs.
func Classifier() *file.Classifier {
	descriptors := append(AlwaysEnabled(), Optional()...)
	settings := plugin.Values(nil, descriptors)
	a := core.New(&core.Config{})
	for _, d := range descriptors {
		ext, err := d.Initializer(settings)
		if err != nil {
			continue
		}
		_ = ext.Init(a)
	}
	return a.Files()
}
