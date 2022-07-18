package main

import (
	"analyzer/snippets"
	"github.com/jessevdk/go-flags"
	"log"
	"os"
	"regexp"
	"runtime"
	"runtime/pprof"
)

type GeneralOptions struct {
	RegexStats []string `short:"s" long:"regex-stat" description:"Regex stat"`

	Language string `short:"l" long:"language" description:"Programming language"`

	NoHeader bool `long:"no-header" description:"No header (only applicable csv, tsv, table)"`

	SortedBy string `long:"sorted-by" description:"Sorted by (default: name). For number based columns, this is in descending order."`

	OutputFormat string `short:"o" long:"output-format" description:"Output format: table, ndjson, json, csv (default: table)"`

	CpuProfile string `long:"cpu-profile" description:"Write cpu profile to file"`
	MemProfile string `long:"mem-profile" description:"Write memory profile to file"`
}

func main() {
	generalOptions := &GeneralOptions{}
	args, err := flags.Parse(generalOptions)

	if err != nil {
		return
	}

	// Enable cpu profiling if requested.
	if generalOptions.CpuProfile != "" {
		f, err := os.Create(generalOptions.CpuProfile)
		if err != nil {
			log.Fatal("could not create CPU profile: ", err)
		}
		defer f.Close() // TODO handle error
		if err := pprof.StartCPUProfile(f); err != nil {
			log.Fatal("could not start CPU profile: ", err)
		}
		defer pprof.StopCPUProfile()
	}

	runArchStats(args, generalOptions)

	// Enable memory profiling if requested.
	if generalOptions.MemProfile != "" {
		f, err := os.Create(generalOptions.MemProfile)
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

func runArchStats(args []string, generalOptions *GeneralOptions) {
	command := args[0]
	rootPath := args[1]

	extensions := getLanguageExtensions(generalOptions.Language)

	extensions = append(extensions,
		&snippets.RegexBasedSnippetsProvider{
			Patterns: parseRegexes(generalOptions.RegexStats),
		},
	)
	settings := snippets.AnalysisSettings{SnippetProvider: extensions}
	allResults, _ := snippets.Analyze(rootPath, settings)
	resultsFromCommand, _ := getRowsFromResults(command, allResults)

	sortRows(generalOptions.SortedBy, resultsFromCommand)
	statsToPrint := getDistinctStatsFromRows(resultsFromCommand)

	printRows(statsToPrint, resultsFromCommand, generalOptions)
}

func parseRegexes(input []string) []*regexp.Regexp {
	var toReturn []*regexp.Regexp
	for _, s := range input {
		toReturn = append(toReturn, regexp.MustCompile(s))
	}
	return toReturn
}
