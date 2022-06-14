package main

import (
	"analyzer/archstats"
	"fmt"
	"github.com/jessevdk/go-flags"
	"github.com/ryanuber/columnize"
	"regexp"
	"sort"
	"strings"
)

func main() {
	genOpts := &GeneralOptions{}
	args, err := flags.Parse(genOpts)
	if err != nil {
		return
	}
	command := args[0]
	rootPath := args[1]

	extensions := getLanguageExtensions(genOpts.Language)
	extensions = append(extensions,
		&archstats.FileSizeStatGenerator{},
		&archstats.RegexBasedStats{
			Patterns: parseRegexes(genOpts.RegexStats),
		})
	settings := archstats.AnalysisSettings{Extensions: extensions}
	allResults := archstats.Analyze(rootPath, settings)
	resultsFromCommand := getMeasurables(command, allResults)

	sort.Slice(resultsFromCommand, func(i, j int) bool {
		return strings.Compare(resultsFromCommand[i].Identity(), resultsFromCommand[j].Identity()) == 0
	})
	if len(resultsFromCommand) == 0 {
		return
	}

	statsToPrint := getStats(&resultsFromCommand)
	sort.Strings(statsToPrint)
	delimiter := "|"

	var rows []string

	if !genOpts.NoHeader {
		rows = append(rows, printHeader(delimiter, statsToPrint))
	}
	for _, dir := range resultsFromCommand {
		rows = append(rows, rowToString(statsToPrint, delimiter, dir))
	}
	fmt.Println(columnize.SimpleFormat(rows))
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

// arch-stats directories|files|components|summary --regex-stat|s=routes=Route::.* --language|l=custom/php/java/c# --no-header-line=true --min-depth=0 --max-depth=1000000
type GeneralOptions struct {
	RegexStats []string `short:"s" long:"regex-stat" description:"Regex stat"`

	Language string `short:"l" long:"language" description:"Programming language"`

	NoHeader bool `long:"no-header" description:"No header"`
}

func printHeader(delimiter string, statsToPrint []string) string {
	return strings.ToUpper(fmt.Sprintf("name%s%s", delimiter, strings.Join(statsToPrint, delimiter)))
}

func rowToString(statsToPrint []string, delimiter string, dir archstats.Measurable) string {
	buf := strings.Builder{}
	stats := dir.Stats()

	buf.WriteString(fmt.Sprint(dir.Identity()))

	for _, statToPrint := range statsToPrint {
		theStat, hasStat := stats[statToPrint]
		buf.WriteString(fmt.Sprintf(delimiter))
		if hasStat {
			buf.WriteString(fmt.Sprintf("%d", theStat))
		} else {
			buf.WriteString(fmt.Sprint("N/A"))
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
