package cmd

import (
	"errors"
	"github.com/RyanSusana/archstats/export"
	"github.com/RyanSusana/archstats/views"
	"github.com/samber/lo"
	"github.com/spf13/cobra"
	"time"
)

const (
	FlagView               = "view"
	FlagAllViews           = "all-views"
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
		showAllViews, _ := cmd.Flags().GetBool(FlagAllViews)
		if err != nil {
			return err
		}

		if showAllViews {
			viewsToShow = views.GetAvailableViews()
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
				ScanTime:     time.Now(),
			}, viewSlice)
			if err != nil {
				return err
			}
		}
		return nil
	},
}

func init() {
	exportCmd.Flags().StringSliceP(FlagView, "v", views.GetQuickViews(), "The view(s) to export")
	exportCmd.Flags().Bool(FlagAllViews, false, "The view(s) to export")

	exportCmd.Flags().StringP(FlagExportOutputFormat, "o", "sqlite", "The output format")
	exportCmd.Flags().String(FlagReportId, "", "The report id")
	exportCmd.Flags().String(FlagSqliteDb, "", "Database to export to")
}
