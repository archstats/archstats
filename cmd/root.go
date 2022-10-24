package cmd

import (
	"fmt"
	"github.com/RyanSusana/archstats/analysis"
	"github.com/RyanSusana/archstats/extensions/analyzers/indentation"
	"github.com/RyanSusana/archstats/extensions/analyzers/regex"
	"github.com/RyanSusana/archstats/extensions/analyzers/required"
	"github.com/RyanSusana/archstats/extensions/views/basic"
	"github.com/samber/lo"
	"github.com/spf13/cobra"
	"os"
	"path/filepath"
	"regexp"
)

var (
	rootCmd = &cobra.Command{
		Use:   "archstats",
		Short: "archstats is a command line tool for generating software architectural insights",
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			//TODO

			//// Enable cpu profiling if requested.
			//if generalOptions.Profile.Cpu != "" {
			//	f, err := os.Create(generalOptions.Profile.Cpu)
			//	if err != nil {
			//		return "", err
			//	}
			//	defer f.Close() // TODO handle error
			//	if err := pprof.StartCPUProfile(f); err != nil {
			//		return "", err
			//	}
			//	defer pprof.StopCPUProfile()
			//}
			//
			//output, err := runArchStats(generalOptions)
			//
			//// Enable memory profiling if requested.
			//if generalOptions.Profile.Mem != "" {
			//	f, err := os.Create(generalOptions.Profile.Mem)
			//	if err != nil {
			//		return "", err
			//	}
			//	defer f.Close() // TODO handle error
			//	runtime.GC()
			//	if err := pprof.WriteHeapProfile(f); err != nil {
			//		return "", err
			//	}
			//}
		},
	}
)

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringSliceP(FlagExtension, "e", nil, "Archstat extension(s) to use")
	rootCmd.PersistentFlags().StringSliceP(FlagSnippet, "s", nil, "Regular Expression to match snippet types. FlagSnippet types are named by using regex named groups(?P<typeName>). For example, if you want to match a JavaScript function, you can use the regex 'function (?P<function>.*)'")
	rootCmd.PersistentFlags().StringP(FlagWorkingDirectory, "f", "", "Input directory")

	rootCmd.AddCommand(viewCmd)
	rootCmd.AddCommand(exportCmd)
}

const (
	FlagWorkingDirectory = "working-dir"
	FlagExtension        = "extension"
	FlagSnippet          = "snippet"
)

func getResults(command *cobra.Command) (*analysis.Results, error) {

	rootDir, _ := command.Flags().GetString(FlagWorkingDirectory)
	rootDir, _ = filepath.Abs(rootDir)

	extensionStrings, _ := command.Flags().GetStringSlice(FlagExtension)

	snippetStrings, _ := command.Flags().GetStringSlice(FlagSnippet)

	var allExtensions = DefaultExtensions()
	for _, extension := range extensionStrings {
		provider, err := OptionalExtension(extension)
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

	settings := analysis.New(rootDir, allExtensions)

	allResults, err := analysis.Analyze(settings)
	return allResults, err
}

func DefaultExtensions() []analysis.Extension {
	return []analysis.Extension{required.Extension(), basic.Extension()}
}

func OptionalExtension(in string) (analysis.Extension, error) {
	switch in {
	case "indentation":
		return &indentation.Analyzer{}, nil
	default:
		return regex.BuiltInRegexExtension(in)
	}
}
