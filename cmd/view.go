package cmd

import (
	"encoding/json"
	"fmt"
	"github.com/RyanSusana/archstats/views"
	"github.com/ryanuber/columnize"
	"github.com/samber/lo"
	"github.com/spf13/cobra"
	"golang.org/x/exp/slices"
	"strings"
)

var viewCmd = &cobra.Command{
	Use:   "view",
	Short: "View data",
	Long:  `View data`,
	Run: func(cmd *cobra.Command, args []string) {
		results, _ := getResults(cmd)
		view, _ := cmd.Flags().GetString("view")
		sortedBy, _ := cmd.Flags().GetString("sorted-by")

		resultsFromCommand, _ := views.RenderView(view, results)

		views.SortRows(sortedBy, resultsFromCommand)
		printRows(resultsFromCommand, cmd)
	},
}

func init() {
	viewCmd.Flags().StringP("view", "v", "", "View to include")
	viewCmd.Flags().StringP("column", "c", "", "When this option is present, it will only show columns in the comma-separated list of columns.")
	viewCmd.Flags().Bool("no-header", false, "No header (only applicable for csv, tsv, table)")
	viewCmd.Flags().String("sorted-by", "", "Sorted by column name. For number based columns, this is in descending order.")
	viewCmd.Flags().StringP("output-format", "o", "table", "Output format")
	viewCmd.MarkFlagRequired("view")
}

type rowData map[string]interface{}

func printRows(resultsFromCommand *views.View, cmd *cobra.Command) string {

	columnsInput, err := cmd.Flags().GetStringSlice("column")
	output, err := cmd.Flags().GetString("output-format")
	noHeader, err := cmd.Flags().GetBool("no-header")

	columnsInput = lo.Map(columnsInput, func(columnName string, idx int) string {
		return strings.ToLower(strings.TrimSpace(columnName))
	})
	if err != nil {

	}

	columnsToPrint := resultsFromCommand.Columns

	if len(columnsInput) > 0 {
		columnsToPrint = lo.Filter(columnsToPrint, func(column *views.Column, idx int) bool {
			return slices.Contains(columnsInput, column.Name)
		})
	}

	switch output {
	case "csv":
		return strings.Join(getRows(columnsToPrint, resultsFromCommand.Rows, true, ","), "\n")
	case "tsv":
		return strings.Join(getRows(columnsToPrint, resultsFromCommand.Rows, !noHeader, "\t"), "\n")
	case "json":
		return string(getJson(columnsToPrint, resultsFromCommand.Rows))
	case "ndjson":
		var stringBuilder strings.Builder
		for _, dir := range resultsFromCommand.Rows {
			theJson, _ := json.Marshal(measurableToMap(dir, columnsToPrint))

			stringBuilder.WriteString(string(theJson))
			stringBuilder.WriteString("\n")
		}
		return stringBuilder.String()
	default:
		return columnize.SimpleFormat(getRows(columnsToPrint, resultsFromCommand.Rows, !noHeader, "|"))
	}
}

func getJson(columnsToPrint []*views.Column, rows []*views.Row) []byte {
	toPrint := rowsToMaps(columnsToPrint, rows)
	theJson, _ := json.Marshal(toPrint)
	return theJson
}

func rowsToMaps(columnsToPrint []*views.Column, rows []*views.Row) []rowData {
	var toPrint []rowData
	for _, row := range rows {
		toPrint = append(toPrint, measurableToMap(row, columnsToPrint))
	}
	return toPrint
}

func measurableToMap(measurable *views.Row, columns []*views.Column) map[string]interface{} {
	toReturn := map[string]interface{}{}
	for _, column := range columns {
		toReturn[column.Name] = measurable.Data[column.Name]
	}
	return toReturn
}

func getRows(columnsToPrint []*views.Column, resultsFromCommand []*views.Row, shouldPrintHeader bool, delimiter string) []string {
	var rows []string
	if shouldPrintHeader {
		rows = append(rows, getHeader(delimiter, columnsToPrint))
	}
	for _, dir := range resultsFromCommand {
		rows = append(rows, rowToString(columnsToPrint, delimiter, dir))
	}
	return rows
}

func getHeader(delimiter string, columnsToPrint []*views.Column) string {
	columnNames := lo.Map(columnsToPrint, func(column *views.Column, idx int) string {
		return column.Name
	})
	return strings.ToUpper(strings.Join(columnNames, delimiter))
}

func rowToString(columnsToPrint []*views.Column, delimiter string, row *views.Row) string {
	toReturn := make([]string, 0, len(columnsToPrint))
	columns := row.Data

	for _, columnToPrint := range columnsToPrint {
		theStat, hasStat := columns[columnToPrint.Name]
		if hasStat {
			toReturn = append(toReturn, fmt.Sprintf("%v", theStat))
		} else {
			toReturn = append(toReturn, "-")
		}
	}
	return strings.Join(toReturn, delimiter)
}
