package common

import (
	"github.com/RyanSusana/archstats/analysis"
	"github.com/RyanSusana/archstats/extensions/analyzers/indentations"
	"github.com/RyanSusana/archstats/extensions/analyzers/regex"
	"github.com/RyanSusana/archstats/extensions/analyzers/required"
	"github.com/RyanSusana/archstats/extensions/views/basic"
	"github.com/RyanSusana/archstats/extensions/views/cycles"
	"github.com/samber/lo"
	"github.com/spf13/cobra"
	"path/filepath"
	"regexp"
)

const (
	FlagWorkingDirectory = "working-dir"
	FlagExtension        = "extension"
	FlagSnippet          = "snippet"
)

func Analyze(command *cobra.Command) (*analysis.Results, error) {

	rootDir, _ := command.Flags().GetString(FlagWorkingDirectory)
	rootDir, _ = filepath.Abs(rootDir)

	extensionStrings, _ := command.Flags().GetStringSlice(FlagExtension)

	snippetStrings, _ := command.Flags().GetStringSlice(FlagSnippet)

	var allExtensions = defaultExtensions()
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

	allResults, err := analysis.New(&analysis.Config{
		RootPath:   rootDir,
		Extensions: allExtensions,
	}).Analyze()

	return allResults, err
}

func defaultExtensions() []analysis.Extension {
	return []analysis.Extension{required.Extension(), basic.Extension()}
}

func optionalExtension(in string) (analysis.Extension, error) {
	switch in {
	case "cycles":
		return cycles.Extension(), nil
	case "indentations":
		return indentations.Extension(), nil
	default:
		return regex.BuiltInRegexExtension(in)
	}
}