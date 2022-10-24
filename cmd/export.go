package cmd

import (
	"errors"
	"github.com/RyanSusana/archstats/analysis"
	"github.com/RyanSusana/archstats/export"
	"github.com/araddon/dateparse"
	"github.com/samber/lo"
	"github.com/spf13/cobra"
	"time"
)

const (
	FlagView               = "view"
	FlagAllViews           = "all-views"
	FlagSqliteDb           = "sqlite-db"
	FlagReportId           = "report-id"
	FlagReportDate         = "report-timestamp"
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
		reportDateString, _ := cmd.Flags().GetString(FlagReportDate)
		results, _ := getResults(cmd)
		//showAllViews, _ := cmd.Flags().GetBool(FlagAllViews)
		var reportDate time.Time
		if err != nil {
			return err
		}

		viewsToShow = results.GetAllViews()
		if reportDateString == "" {
			reportDate = time.Now()
		} else {
			reportDate, err = dateparse.ParseAny(reportDateString)
			if err != nil {
				return err
			}
		}

		allViews := make(map[string]*analysis.View)

		for _, viewName := range viewsToShow {
			view, err := results.RenderView(viewName)
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

			viewSlice := lo.MapToSlice(allViews, func(viewName string, view *analysis.View) *analysis.View {
				return view
			})
			err := export.SaveToDB(&export.SqlOptions{
				DatabaseName: dbPath,
				ReportId:     reportId,
				ScanTime:     reportDate,
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
	exportCmd.Flags().Bool(FlagAllViews, false, "The view(s) to export")
	exportCmd.Flags().String(FlagReportId, "", "The report id")
	exportCmd.Flags().String(FlagReportDate, "", "The report date")
	exportCmd.Flags().String(FlagSqliteDb, "", "Database to export to")
}
