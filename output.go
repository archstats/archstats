package main

import (
	"encoding/json"
	"fmt"
	"github.com/ryanuber/columnize"
	"strings"
)

func printRows(resultsFromCommand *View, genOpts *GeneralOptions) {
	columnsToPrint := resultsFromCommand.OrderedColumns
	switch genOpts.OutputFormat {
	case "csv":
		fmt.Println(getRows(columnsToPrint, resultsFromCommand.rows, !genOpts.NoHeader, ","))
	case "tsv":
		fmt.Println(getRows(columnsToPrint, resultsFromCommand.rows, !genOpts.NoHeader, "\t"))
	case "json":
		printJson(columnsToPrint, resultsFromCommand.rows)
	case "ndjson":
		printNdjson(columnsToPrint, resultsFromCommand.rows)
	default:
		fmt.Println(columnize.SimpleFormat(getRows(columnsToPrint, resultsFromCommand.rows, !genOpts.NoHeader, "|")))
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

	for _, stat := range stats {
		toReturn[stat] = measurable.Data[stat]
	}

	return toReturn
}

func getRows(columnsToPrint []string, resultsFromCommand []*Row, shouldPrintHeader bool, delimiter string) []string {

	var rows []string

	if shouldPrintHeader {
		rows = append(rows, getHeader(delimiter, columnsToPrint))
	}
	for _, dir := range resultsFromCommand {
		rows = append(rows, rowToString(columnsToPrint, delimiter, dir))
	}
	return rows
}

func getHeader(delimiter string, columnsToPrint []string) string {
	return strings.ToUpper(strings.Join(columnsToPrint, delimiter))
}
func rowToString(columnsToPrint []string, delimiter string, row *Row) string {
	toReturn := make([]string, 0, len(columnsToPrint))
	columns := row.Data

	for _, columnToPrint := range columnsToPrint {
		theStat, hasStat := columns[columnToPrint]
		if hasStat {
			toReturn = append(toReturn, fmt.Sprintf("%v", theStat))
		} else {
			toReturn = append(toReturn, "-")
		}
	}
	return strings.Join(toReturn, delimiter)
}
