package main

import (
	"errors"
	"fmt"
	"github.com/RyanSusana/archstats/snippets"
	"github.com/RyanSusana/archstats/views"
	"github.com/RyanSusana/archstats/walker"
	"github.com/jessevdk/go-flags"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"runtime/pprof"
	"sync"
)

type GeneralOptions struct {
	Args struct {
		RootDir string `description:"Root directory of the project" required:"true" positional-arg-name:"<project-directory>"`
	} `positional-args:"true" required:"true"`

	View string `short:"v" long:"view" default:"directories-recursive" description:"Type of view to show" required:"true"`

	Snippets []string `short:"s" long:"snippet" description:"Regular Expression to match snippet types. Snippet types are named by using regex named groups(?P<typeName>). For example, if you want to match a JavaScript function, you can use the regex 'function (?P<function>.*)'"`

	Extensions []string `short:"e" long:"extensions"  description:"This option adds support for additional extensions. The value of this option is a comma separated list of extensions. The supported extensions are: php"`

	Columns []string `short:"c" long:"column" description:"When this option is present, it will only show columns in the comma-separated list of columns."`

	NoHeader bool `long:"no-header" description:"No header (only applicable for csv, tsv, table)"`

	SortedBy string `long:"sorted-by"  description:"Sorted by column name. For number based columns, this is in descending order."`

	OutputFormat string `short:"o" long:"output-format" choice:"table" choice:"ndjson" choice:"json" choice:"csv" choice:"tsv" description:"Output format"`

	Profile struct {
		Cpu string `long:"cpu" description:"File to write CPU profile to"`
		Mem string `long:"mem" description:"File to write memory profile to"`
	} `group:"Profiling" hidden:"true" namespace:"profile"`
}

func main() {
	exitCode := 0
	defer func() { os.Exit(exitCode) }()

	generalOptions := &GeneralOptions{}
	_, err := flags.NewParser(generalOptions, flags.Default|flags.IgnoreUnknown).Parse()

	if err != nil {
		exitCode = printError(err)
		return
	}

	// Enable cpu profiling if requested.
	if generalOptions.Profile.Cpu != "" {
		f, err := os.Create(generalOptions.Profile.Cpu)
		if err != nil {
			log.Fatal("could not create CPU profile: ", err)
		}
		defer f.Close() // TODO handle error
		if err := pprof.StartCPUProfile(f); err != nil {
			log.Fatal("could not start CPU profile: ", err)
		}
		defer pprof.StopCPUProfile()
	}

	err = runArchStats(generalOptions)
	if err != nil {
		exitCode = printError(err)
	}

	// Enable memory profiling if requested.
	if generalOptions.Profile.Mem != "" {
		f, err := os.Create(generalOptions.Profile.Mem)
		if err != nil {
			log.Fatal("could not create memory profile: ", err)
		}
		defer f.Close() // TODO handle error
		runtime.GC()
		if err := pprof.WriteHeapProfile(f); err != nil {
			log.Fatal("could not write memory profile: ", err)
		}
	}
}

func printError(err error) int {
	fmt.Printf("Error: %s", err)
	return 1
}
func runArchStats(generalOptions *GeneralOptions) error {
	generalOptions.Args.RootDir, _ = filepath.Abs(generalOptions.Args.RootDir)
	var extensions []snippets.SnippetProvider
	for _, extension := range generalOptions.Extensions {
		extensions = append(extensions, getExtensions(extension)...)
	}

	extensions = append(extensions,
		&snippets.RegexBasedSnippetsProvider{
			Patterns: parseRegexes(generalOptions.Snippets),
		},
	)
	settings := snippets.AnalysisSettings{SnippetProviders: extensions}

	allResults, err := Analyze(generalOptions.Args.RootDir, settings)
	if err != nil {
		return err
	}
	resultsFromCommand, err := views.GetRowsFromResults(generalOptions.View, allResults)
	if err != nil {
		return err
	}
	sortRows(generalOptions.SortedBy, resultsFromCommand)

	printRows(resultsFromCommand, generalOptions)
	return nil
}
func Analyze(rootPath string, settings snippets.AnalysisSettings) (*snippets.Results, error) {

	var allSnippets []*snippets.Snippet
	lock := sync.Mutex{}

	walker.WalkDirectoryConcurrently(rootPath, func(file walker.OpenedFile) {
		var foundSnippets []*snippets.Snippet
		for _, provider := range settings.SnippetProviders {
			foundSnippets = append(foundSnippets, provider.GetSnippetsFromFile(file)...)
		}
		lock.Lock()
		allSnippets = append(allSnippets, foundSnippets...)
		lock.Unlock()
	})
	if len(allSnippets) == 0 {
		return nil, errors.New("could not find any snippets")
	}
	return snippets.CalculateResults(allSnippets), nil
}

func parseRegexes(input []string) []*regexp.Regexp {
	var toReturn []*regexp.Regexp
	for _, s := range input {
		toReturn = append(toReturn, regexp.MustCompile(s))
	}
	return toReturn
}
