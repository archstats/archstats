package common

import (
	"context"
	"github.com/archstats/archstats/core"
	"github.com/spf13/cobra"
	"path/filepath"
)

const (
	FlagWorkingDirectory = "working-dir"
	FlagExtension        = "extension"
	FlagSnippet          = "snippet"
	FlagSet              = "set"
	FlagVerbose          = "verbose"
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
}

func GetCommonFlags(command *cobra.Command) *CommonFlags {
	rootDir, _ := command.Flags().GetString(FlagWorkingDirectory)
	rootDir, _ = filepath.Abs(rootDir)

	extensionStrings, _ := command.Flags().GetStringSlice(FlagExtension)

	snippetStrings, _ := command.Flags().GetStringSlice(FlagSnippet)

	return &CommonFlags{
		WorkingDirectory: rootDir,
		Extensions:       extensionStrings,
		Snippets:         snippetStrings,
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

	allResults, err := core.New(&core.Config{
		RootPath:   rootDir,
		Extensions: archstatsExtensions,
	}).Analyze()

	return allResults, err
}

type emptyExtension struct {
}

func (e *emptyExtension) Init(settings core.Analyzer) error { return nil }

