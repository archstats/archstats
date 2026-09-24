package common

import (
	"context"
	"fmt"
	"github.com/archstats/archstats/core"
	"github.com/archstats/archstats/extensions/regex"
	"github.com/spf13/cobra"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const (
	FlagWorkingDirectory = "working-dir"
	FlagExtension        = "extension"
	FlagSnippet          = "snippet"
	FlagSet              = "set"
	FlagVerbose          = "verbose"
	FlagIgnore           = "ignore"
)

// contextKey is an unexported type used for context keys to avoid collisions.
type contextKey struct{}

// ExtraExtensionsKey is the context key for extra extensions passed programmatically.
var ExtraExtensionsKey = contextKey{}

// ContextWithExtraExtensions creates a context carrying extra extensions.
func ContextWithExtraExtensions(ctx context.Context, extensions []core.Extension) context.Context {
	return context.WithValue(ctx, ExtraExtensionsKey, extensions)
}

type CommonFlags struct {
	WorkingDirectory string
	Extensions       []string
	Snippets         []string
	// Ignore patterns on top of the tree's own ignore files (gitignore syntax).
	Ignore []string
}

func GetCommonFlags(command *cobra.Command) *CommonFlags {
	rootDir, _ := command.Flags().GetString(FlagWorkingDirectory)
	rootDir, _ = filepath.Abs(rootDir)

	extensionStrings, _ := command.Flags().GetStringSlice(FlagExtension)

	snippetStrings, _ := command.Flags().GetStringSlice(FlagSnippet)
	ignoreStrings, _ := command.Flags().GetStringSlice(FlagIgnore)

	return &CommonFlags{
		WorkingDirectory: rootDir,
		Extensions:       extensionStrings,
		Snippets:         snippetStrings,
		Ignore:           ignoreStrings,
	}
}

func Analyze(command *cobra.Command) (*core.Results, error) {
	commonFlags := GetCommonFlags(command)
	rootDir, _ := filepath.Abs(commonFlags.WorkingDirectory)

	enabledExtensions, err := GetEnabledExtensions(command)
	if err != nil {
		return nil, err
	}

	var archstatsExtensions []core.Extension
	for _, extension := range enabledExtensions {
		initializer, err := extension.Initializer(command)
		if err != nil {
			return nil, err
		}
		archstatsExtensions = append(archstatsExtensions, initializer)
	}
	if extra, ok := command.Context().Value(ExtraExtensionsKey).([]core.Extension); ok {
		archstatsExtensions = append(archstatsExtensions, extra...)
	}

	// --snippet was parsed and never used, so every pattern a user passed
	// was silently ignored while the help text promised the opposite.
	if len(commonFlags.Snippets) > 0 {
		snippets, err := SnippetExtension(commonFlags.Snippets)
		if err != nil {
			return nil, err
		}
		archstatsExtensions = append(archstatsExtensions, snippets)
	}

	allResults, err := core.New(&core.Config{
		RootPath:       rootDir,
		Extensions:     archstatsExtensions,
		IgnorePatterns: commonFlags.Ignore,
	}).Analyze()
	if err == nil && allResults != nil {
		names := make([]string, 0, len(enabledExtensions))
		for _, ext := range enabledExtensions {
			names = append(names, ext.Name)
		}
		sort.Strings(names)
		allResults.SetSnapshotInfo("extensions", strings.Join(names, ","))
	}
	return allResults, err
}

type emptyExtension struct {
}

func (e *emptyExtension) Init(settings core.Analyzer) error { return nil }

// SnippetExtension compiles user-supplied --snippet patterns into a regex
// extension. A pattern must compile and must name at least one group: the
// group's name is the snippet type, so a pattern without one could never
// record anything and is a mistake worth stopping on.
func SnippetExtension(patterns []string) (core.Extension, error) {
	compiled := make([]*regexp.Regexp, 0, len(patterns))
	for _, p := range patterns {
		re, err := regexp.Compile(p)
		if err != nil {
			return nil, fmt.Errorf("--snippet %q: %w", p, err)
		}
		named := false
		for _, name := range re.SubexpNames() {
			if name != "" {
				named = true
				break
			}
		}
		if !named {
			return nil, fmt.Errorf("--snippet %q: needs a named group, like (?P<function>...), whose name becomes the snippet type", p)
		}
		compiled = append(compiled, re)
	}
	return &regex.Extension{Patterns: compiled}, nil
}
