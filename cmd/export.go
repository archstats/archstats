package cmd

import (
	"encoding/json"
	"errors"
	"github.com/RyanSusana/archstats/export"
	"github.com/RyanSusana/archstats/views"
	"github.com/samber/lo"
	"github.com/spf13/cobra"
)

const (
	FlagView               = "view"
	FlagSqliteDb           = "sqlite-db"
	FlagReportId           = "report-id"
	FlagExportOutputFormat = "output-format"
)

var exportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export data to a destination",
	Long:  `Export data to a database`,
	RunE: func(cmd *cobra.Command, args []string) error {

		outputFormat, _ := cmd.Flags().GetString(FlagExportOutputFormat)
		viewsToShow, err := cmd.Flags().GetStringSlice(FlagView)
		reportId, _ := cmd.Flags().GetString(FlagReportId)
		allResults, _ := getResults(cmd)
		if err != nil {
			return err
		}

		availableViews := views.GetAvailableViews()

		if len(viewsToShow) == 0 {
			viewsToShow = availableViews
		}

		allViews := make(map[string]*views.View)

		for _, viewName := range viewsToShow {
			view, err := views.RenderView(viewName, allResults)
			if err != nil {
				return err
			}
			allViews[viewName] = view
		}

		switch outputFormat {
		default:
			dbPath, _ := cmd.Flags().GetString(FlagSqliteDb)
			if dbPath == "" {
				return errors.New("sqlite-db is required")
			}

			viewSlice := lo.MapToSlice(allViews, func(viewName string, view *views.View) *views.View {
				return view
			})
			err := export.SaveToDB(&export.SqlOptions{
				DatabaseName: dbPath,
				ReportId:     reportId,
			}, viewSlice)
			if err != nil {
				return err
			}
		}
		return nil
	},
}

func init() {
	exportCmd.Flags().StringSliceP(FlagView, "v", []string{}, "The view(s) to export")
	exportCmd.Flags().StringP(FlagExportOutputFormat, "o", "sqlite", "The output format")
	exportCmd.Flags().String(FlagReportId, "", "The report id")
	exportCmd.Flags().String(FlagSqliteDb, "", "Database to export to")
}

func printAllViews(allViews map[string]*views.View) string {

	theViews := make(map[string][]rowData)
	for viewName, view := range allViews {
		theViews[viewName] = rowsToMaps(view.Columns, view.Rows)
	}
	theJson, _ := json.Marshal(theViews)

	return string(theJson)
}
