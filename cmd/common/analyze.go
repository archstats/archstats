package common

import (
	"github.com/archstats/archstats/core"
	"github.com/archstats/archstats/extensions/basic"
	"github.com/archstats/archstats/extensions/cycles"
	"github.com/archstats/archstats/extensions/git"
	"github.com/archstats/archstats/extensions/indentations"
	"github.com/archstats/archstats/extensions/lines"
	"github.com/archstats/archstats/extensions/regex"
	"github.com/archstats/archstats/extensions/required"
	"github.com/samber/lo"
	"github.com/spf13/cobra"
	"path/filepath"
	"regexp"
)

const (
	FlagWorkingDirectory = "working-dir"
	FlagExtension        = "extension"
	FlagSnippet          = "snippet"
	FlagSet              = "set"
)

func Analyze(command *cobra.Command) (*core.Results, error) {
	rootDir, _ := command.Flags().GetString(FlagWorkingDirectory)
	rootDir, _ = filepath.Abs(rootDir)

	extensionStrings, _ := command.Flags().GetStringSlice(FlagExtension)

	snippetStrings, _ := command.Flags().GetStringSlice(FlagSnippet)

	var allExtensions = defaultExtensions()
	var extraExtensions = command.Context().Value("extraExtensions").([]core.Extension)
	allExtensions = append(allExtensions, extraExtensions...)
	for _, extension := range extensionStrings {
		provider, err := optionalExtension(extension)
		if err != nil {
			return nil, err
		}
		allExtensions = append(allExtensions, provider)
	}

	allExtensions = append(allExtensions,
		&regex.Extension{
			Patterns: lo.Map(snippetStrings, func(s string, idx int) *regexp.Regexp {
				return regexp.MustCompile(s)
			}),
		},
	)

	allResults, err := core.New(&core.Config{
		RootPath:   rootDir,
		Extensions: allExtensions,
	}).Analyze()

	return allResults, err
}

func defaultExtensions() []core.Extension {
	return []core.Extension{required.Extension(), basic.Extension(), &lines.Extension{}}
}

func optionalExtension(in string) (core.Extension, error) {
	switch in {
	// Lines is already included by default, this is for backwards compatibility
	case "lines":
		return &emptyExtension{}, nil
	case "git":
		return git.Extension(), nil
	case "cycles":
		return cycles.Extension(), nil
	case "indentations":
		return indentations.FourTabs(), nil
	case "indentations-2":
		return indentations.TwoTabs(), nil
	default:
		return regex.BuiltInRegexExtension(in)
	}
}

type emptyExtension struct {
}

func (e *emptyExtension) Init(settings core.Analyzer) error { return nil }
