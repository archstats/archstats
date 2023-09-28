package common

import (
	"github.com/RyanSusana/archstats/core"
	"github.com/RyanSusana/archstats/extensions/basic"
	"github.com/RyanSusana/archstats/extensions/cycles"
	"github.com/RyanSusana/archstats/extensions/indentations"
	"github.com/RyanSusana/archstats/extensions/regex"
	"github.com/RyanSusana/archstats/extensions/required"
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
	return []core.Extension{required.Extension(), basic.Extension()}
}

func optionalExtension(in string) (core.Extension, error) {
	switch in {
	case "cycles":
		return cycles.Extension(), nil
	case "indentations":
		return indentations.FourTabs(), nil
	default:
		return regex.BuiltInRegexExtension(in)
	}
}
