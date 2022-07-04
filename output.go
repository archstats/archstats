package main

import (
	"encoding/json"
	"fmt"
	"github.com/ryanuber/columnize"
	"sort"
	"strings"
)

func printRows(statsToPrint []string, resultsFromCommand []*Row, genOpts *GeneralOptions) {
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

func printNdjson(stats []string, command []*Row) {
	for _, dir := range command {
		theJson, _ := json.Marshal(measurableToMap(dir, stats))

		fmt.Println(string(theJson))
	}
}
func printJson(stats []string, command []*Row) {
	var toPrint []map[string]interface{}
	for _, dir := range command {

		toPrint = append(toPrint, measurableToMap(dir, stats))
	}
	theJson, _ := json.Marshal(toPrint)
	fmt.Println(string(theJson))
}
func measurableToMap(measurable *Row, stats []string) map[string]interface{} {
	toReturn := map[string]interface{}{}

	toReturn["name"] = measurable.Name
	for _, stat := range stats {
		toReturn[stat] = measurable.Data[stat]
	}

	return toReturn
}

func getRows(statsToPrint []string, resultsFromCommand []*Row, shouldPrintHeader bool, delimiter string) []string {
	sort.Strings(statsToPrint)

	var rows []string

	if shouldPrintHeader {
		rows = append(rows, getHeader(delimiter, statsToPrint))
	}
	for _, dir := range resultsFromCommand {
		rows = append(rows, rowToString(statsToPrint, delimiter, dir))
	}
	return rows
}

func getHeader(delimiter string, statsToPrint []string) string {
	return strings.ToUpper(fmt.Sprintf("name%s%s", delimiter, strings.Join(statsToPrint, delimiter)))
}
func rowToString(statsToPrint []string, delimiter string, row *Row) string {
	buf := strings.Builder{}
	stats := row.Data

	buf.WriteString(fmt.Sprint(row.Name))

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
