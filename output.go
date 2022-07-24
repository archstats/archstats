package main

import (
	"archstats/views"
	"encoding/json"
	"fmt"
	"github.com/ryanuber/columnize"
	"golang.org/x/exp/slices"
	"strings"
)

func printRows(resultsFromCommand *views.View, genOpts *GeneralOptions) {
	availableColumns := resultsFromCommand.OrderedColumns

	if len(genOpts.Columns) > 0 {
		var columnsToPrint []string
		for _, columns := range genOpts.Columns {
			for _, untrimmedColumn := range strings.Split(columns, ",") {

				column := strings.ToLower(strings.Trim(untrimmedColumn, " "))
				if slices.Contains(availableColumns, column) {
					columnsToPrint = append(columnsToPrint, column)
				}
			}
		}
		availableColumns = columnsToPrint
	}
	switch genOpts.OutputFormat {
	case "csv":
		fmt.Println(getRows(availableColumns, resultsFromCommand.Rows, !genOpts.NoHeader, ","))
	case "tsv":
		fmt.Println(getRows(availableColumns, resultsFromCommand.Rows, !genOpts.NoHeader, "\t"))
	case "json":
		printJson(availableColumns, resultsFromCommand.Rows)
	case "ndjson":
		printNdjson(availableColumns, resultsFromCommand.Rows)
	default:
		fmt.Println(columnize.SimpleFormat(getRows(availableColumns, resultsFromCommand.Rows, !genOpts.NoHeader, "|")))
	}
}

func printNdjson(columnsToPrint []string, command []*views.Row) {
	for _, dir := range command {
		theJson, _ := json.Marshal(measurableToMap(dir, columnsToPrint))

		fmt.Println(string(theJson))
	}
}
func printJson(columnsToPrint []string, command []*views.Row) {
	var toPrint []map[string]interface{}
	for _, dir := range command {
		toPrint = append(toPrint, measurableToMap(dir, columnsToPrint))
	}
	theJson, _ := json.Marshal(toPrint)
	fmt.Println(string(theJson))
}
func measurableToMap(measurable *views.Row, stats []string) map[string]interface{} {
	toReturn := map[string]interface{}{}

	for _, stat := range stats {
		toReturn[stat] = measurable.Data[stat]
	}

	return toReturn
}

func getRows(columnsToPrint []string, resultsFromCommand []*views.Row, shouldPrintHeader bool, delimiter string) []string {

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
func rowToString(columnsToPrint []string, delimiter string, row *views.Row) string {
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
