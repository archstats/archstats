package main

import (
	"encoding/json"
	"fmt"
	"github.com/RyanSusana/archstats/views"
	"github.com/ryanuber/columnize"
	"golang.org/x/exp/slices"
	"strings"
)

type rowData map[string]interface{}

func printAllViews(allViews map[string]*views.View) string {

	theViews := make(map[string][]rowData)
	for viewName, view := range allViews {
		theViews[viewName] = rowsToMaps(view.OrderedColumns, view.Rows)
	}
	theJson, _ := json.Marshal(theViews)

	return string(theJson)
}
func printRows(resultsFromCommand *views.View, genOpts *GeneralOptions) string {
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
		return strings.Join(getRows(availableColumns, resultsFromCommand.Rows, true, ","), "\n")
	case "tsv":
		return strings.Join(getRows(availableColumns, resultsFromCommand.Rows, !genOpts.NoHeader, "\t"), "\n")
	case "json":
		return string(getJson(availableColumns, resultsFromCommand.Rows))
	case "ndjson":
		var stringBuilder strings.Builder
		for _, dir := range resultsFromCommand.Rows {
			theJson, _ := json.Marshal(measurableToMap(dir, availableColumns))

			stringBuilder.WriteString(string(theJson))
			stringBuilder.WriteString("\n")
		}
		return stringBuilder.String()
	default:
		return columnize.SimpleFormat(getRows(availableColumns, resultsFromCommand.Rows, !genOpts.NoHeader, "|"))
	}
}

func getJson(columnsToPrint []string, rows []*views.Row) []byte {
	toPrint := rowsToMaps(columnsToPrint, rows)
	theJson, _ := json.Marshal(toPrint)
	return theJson
}

func rowsToMaps(columnsToPrint []string, rows []*views.Row) []rowData {
	var toPrint []rowData
	for _, row := range rows {
		toPrint = append(toPrint, measurableToMap(row, columnsToPrint))
	}
	return toPrint
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
