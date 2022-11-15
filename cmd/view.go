package cmd

import (
	"encoding/json"
	"fmt"
	"github.com/RyanSusana/archstats/analysis"
	"github.com/RyanSusana/archstats/extensions/views/basic"
	"github.com/ryanuber/columnize"
	"github.com/samber/lo"
	"github.com/spf13/cobra"
	"golang.org/x/exp/slices"
	"io"
	"sort"
	"strings"
)

var viewCmd = &cobra.Command{
	Use:          "view <view>",
	Short:        "View data",
	Long:         `View data`,
	Args:         cobra.ExactArgs(1),
	SilenceUsage: true,
	PreRun: func(cmd *cobra.Command, args []string) {
		cmd.AddCommand(exportCmd)
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		results, err := getResults(cmd)
		if err != nil {
			return err
		}

		view := args[0]
		availableViews := lo.Map(results.GetAllViewFactories(), func(vf *analysis.ViewFactory, index int) string {
			return vf.Name
		})
		if !slices.Contains(availableViews, view) {
			viewStrings := lo.Map(results.GetAllViewFactories(), func(vf *analysis.ViewFactory, index int) string {
				return fmt.Sprintf("  - %s: %s", vf.Name, vf.Description)
			})
			sort.Strings(viewStrings)
			availableViewsString := strings.Join(viewStrings, "\n")
			return fmt.Errorf("no view named '%s'. Available views:\n%s", view, availableViewsString)
		}
		sortedBy, err := cmd.Flags().GetString("sorted-by")
		if err != nil {
			return err
		}

		resultsFromCommand, err := results.RenderView(view)
		if err != nil {
			return err
		}

		basic.SortRows(sortedBy, resultsFromCommand)
		str := outputString(resultsFromCommand, cmd)

		_, err = io.WriteString(cmd.OutOrStdout(), str)

		return err
	},
}

func createCommand(factory *analysis.ViewFactory) *cobra.Command {
	return &cobra.Command{
		Use:   factory.Name,
		Short: factory.Description,
		Long:  factory.Description,
	}
}

func init() {
	viewCmd.Flags().StringP("column", "c", "", "When this option is present, it will only show columns in the comma-separated list of columns.")
	viewCmd.Flags().Bool("no-header", false, "No header (only applicable for csv, tsv, table)")
	viewCmd.Flags().String("sorted-by", "", "Sort by <column>. For number based columns, this is in descending order.")
	viewCmd.Flags().StringP("output-format", "o", "table", "Output format")
}

type rowData map[string]interface{}

func outputString(resultsFromCommand *analysis.View, cmd *cobra.Command) string {

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
		columnsToPrint = lo.Filter(columnsToPrint, func(column *analysis.Column, idx int) bool {
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

func getJson(columnsToPrint []*analysis.Column, rows []*analysis.Row) []byte {
	toPrint := rowsToMaps(columnsToPrint, rows)
	theJson, _ := json.Marshal(toPrint)
	return theJson
}

func rowsToMaps(columnsToPrint []*analysis.Column, rows []*analysis.Row) []rowData {
	var toPrint []rowData
	for _, row := range rows {
		toPrint = append(toPrint, measurableToMap(row, columnsToPrint))
	}
	return toPrint
}

func measurableToMap(measurable *analysis.Row, columns []*analysis.Column) map[string]interface{} {
	toReturn := map[string]interface{}{}
	for _, column := range columns {
		toReturn[column.Name] = measurable.Data[column.Name]
	}
	return toReturn
}

func getRows(columnsToPrint []*analysis.Column, resultsFromCommand []*analysis.Row, shouldPrintHeader bool, delimiter string) []string {
	var rows []string
	if shouldPrintHeader {
		rows = append(rows, getHeader(delimiter, columnsToPrint))
	}
	for _, dir := range resultsFromCommand {
		rows = append(rows, rowToString(columnsToPrint, delimiter, dir))
	}
	return rows
}

func getHeader(delimiter string, columnsToPrint []*analysis.Column) string {
	columnNames := lo.Map(columnsToPrint, func(column *analysis.Column, idx int) string {
		return column.Name
	})
	return strings.ToUpper(strings.Join(columnNames, delimiter))
}

func rowToString(columnsToPrint []*analysis.Column, delimiter string, row *analysis.Row) string {
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
