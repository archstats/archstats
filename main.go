package main

import (
	"analyzer/archstats"
	"encoding/json"
	"fmt"
	"github.com/jessevdk/go-flags"
	"github.com/ryanuber/columnize"
	"regexp"
	"sort"
	"strings"
)

func main() {
	generalOptions := &GeneralOptions{}
	args, err := flags.Parse(generalOptions)
	if err != nil {
		return
	}
	command := args[0]
	rootPath := args[1]

	extensions := getLanguageExtensions(generalOptions.Language)
	extensions = append(extensions,
		&archstats.FileSizeStatGenerator{},
		&archstats.RegexBasedStats{
			Patterns: parseRegexes(generalOptions.RegexStats),
		})
	settings := archstats.AnalysisSettings{Extensions: extensions}
	allResults := archstats.Analyze(rootPath, settings)
	resultsFromCommand := getMeasurables(command, allResults)

	if generalOptions.SortedBy == "" {
		sort.Slice(resultsFromCommand, func(i, j int) bool {
			return strings.Compare(resultsFromCommand[i].Name(), resultsFromCommand[j].Name()) == 0
		})
	} else {
		sort.Slice(resultsFromCommand, func(i, j int) bool {
			return resultsFromCommand[i].Stats()[generalOptions.SortedBy] > resultsFromCommand[j].Stats()[generalOptions.SortedBy]
		})
	}
	statsToPrint := getStats(&resultsFromCommand)

	printRows(statsToPrint, resultsFromCommand, generalOptions)
}

func printNdjson(stats []string, command []archstats.Measurable) {
	for _, dir := range command {
		theJson, _ := json.Marshal(measurableToMap(dir, stats))

		fmt.Println(string(theJson))
	}
}
func printJson(stats []string, command []archstats.Measurable) {
	var toPrint []map[string]string
	for _, dir := range command {

		toPrint = append(toPrint, measurableToMap(dir, stats))
	}
	theJson, _ := json.Marshal(toPrint)
	fmt.Println(string(theJson))
}
func measurableToMap(measurable archstats.Measurable, stats []string) map[string]string {
	toReturn := map[string]string{}

	toReturn["name"] = measurable.Name()
	for _, stat := range stats {
		toReturn[stat] = fmt.Sprintf("%d", measurable.Stats()[stat])
	}

	return toReturn
}
func printRows(statsToPrint []string, resultsFromCommand []archstats.Measurable, genOpts *GeneralOptions) {
	switch genOpts.OutputFormat {
	case "csv":
		fmt.Println(getRows(statsToPrint, resultsFromCommand, !genOpts.NoHeader, ","))
	case "tsv":
		fmt.Println(getRows(statsToPrint, resultsFromCommand, !genOpts.NoHeader, "\t"))
	case "json":
		printJson(statsToPrint, resultsFromCommand)
	case "ndjson":
		printNdjson(statsToPrint, resultsFromCommand)
	default:
		fmt.Println(columnize.SimpleFormat(getRows(statsToPrint, resultsFromCommand, !genOpts.NoHeader, "|")))
	}
}

func getRows(statsToPrint []string, resultsFromCommand []archstats.Measurable, shouldPrintHeader bool, delimiter string) []string {
	sort.Strings(statsToPrint)

	var rows []string

	if shouldPrintHeader {
		rows = append(rows, printHeader(delimiter, statsToPrint))
	}
	for _, dir := range resultsFromCommand {
		rows = append(rows, rowToString(statsToPrint, delimiter, dir))
	}
	return rows
}

func getStats(all *[]archstats.Measurable) []string {
	allStats := map[string]bool{}
	for _, measurable := range *all {
		for s, _ := range measurable.Stats() {
			allStats[s] = true
		}
	}
	keys := make([]string, len(allStats))
	i := 0
	for k := range allStats {
		keys[i] = k
		i++
	}
	return keys
}
func getMeasurables(command string, results *archstats.AnalysisResults) []archstats.Measurable {
	var measurables []archstats.Measurable
	switch command {
	case "components":
		for _, component := range results.Components {
			measurables = append(measurables, component)
		}
	case "files":
		for _, file := range results.Files {
			measurables = append(measurables, file)
		}
	case "directories":
		for _, directory := range results.Directories {
			measurables = append(measurables, directory)
		}
	}

	return measurables
}

type GeneralOptions struct {
	RegexStats []string `short:"s" long:"regex-stat" description:"Regex stat"`

	Language string `short:"l" long:"language" description:"Programming language"`

	NoHeader bool `long:"no-header" description:"No header"`

	SortedBy string `long:"sorted-by" description:"Sorted by (default: name)"`

	OutputFormat string `short:"o" long:"output-format" description:"Output format: columns, json, csv (default: columns)"`
}

func printHeader(delimiter string, statsToPrint []string) string {
	return strings.ToUpper(fmt.Sprintf("name%s%s", delimiter, strings.Join(statsToPrint, delimiter)))
}

func rowToString(statsToPrint []string, delimiter string, dir archstats.Measurable) string {
	buf := strings.Builder{}
	stats := dir.Stats()

	buf.WriteString(fmt.Sprint(dir.Name()))

	for _, statToPrint := range statsToPrint {
		theStat, hasStat := stats[statToPrint]
		buf.WriteString(fmt.Sprintf(delimiter))
		if hasStat {
			buf.WriteString(fmt.Sprintf("%d", theStat))
		} else {
			buf.WriteString("0")
		}
	}
	return buf.String()
}
func parseRegexes(input []string) []archstats.RegexStatPattern {
	var toReturn []archstats.RegexStatPattern
	for _, s := range input {
		toReturn = append(toReturn, parseRegex(s))
	}
	return toReturn
}
func parseRegex(input string) archstats.RegexStatPattern {
	split := strings.Split(input, "=")
	return archstats.RegexStatPattern{
		Name:   split[0],
		Regexp: regexp.MustCompile(split[1]),
	}
}
func getLanguageExtensions(lang string) []archstats.Extension {
	if lang == "php" {
		return []archstats.Extension{
			archstats.RegexBasedComponents(archstats.RegexBasedComponentSettings{
				Definition: regexp.MustCompile("namespace (?P<component>.*);"),
				Import:     regexp.MustCompile("(use|import) (?P<component>.*)\\\\.*;"),
			}),
		}
	}
	return []archstats.Extension{}
}
