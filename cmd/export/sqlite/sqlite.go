package sqlite

import (
	"database/sql"
	"errors"
	"fmt"
	"github.com/archstats/archstats/cmd/common"
	"github.com/archstats/archstats/core"
	"github.com/archstats/archstats/core/file"
	_ "github.com/mattn/go-sqlite3"
	"github.com/rs/zerolog/log"
	"github.com/samber/lo"
	"github.com/spf13/cobra"
	"math"
	"os"
	"strings"
	"time"
)

const (
	FlagViews        = "views"
	FlagExcludeViews = "exclude-views"
	FlagReportId     = "report-id"
)

func Cmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sqlite",
		Short: "Export data to an SQLite database",
		Long:  `Export data to an SQLite database`,
		Args:  cobra.ExactArgs(1),

		RunE: func(cmd *cobra.Command, args []string) error {
			var err error
			commonFlags := common.GetCommonFlags(cmd)
			reportId, err := cmd.Flags().GetString(FlagReportId)
			if err != nil {
				return err
			}
			log.Info().Msgf("Analyzing %s with extension(s): %s", commonFlags.WorkingDirectory, strings.Join(commonFlags.Extensions, ", "))

			results, err := common.Analyze(cmd)
			if err != nil {
				return err
			}
			var reportDate time.Time

			reportDate = time.Now()

			possibleViews := lo.Map(results.GetViewFactories(), func(vf *core.ViewFactory, index int) string {
				return vf.Name
			})
			viewsRequested, err := cmd.Flags().GetStringSlice(FlagViews)
			viewsExcluded, err := cmd.Flags().GetStringSlice(FlagExcludeViews)
			if err != nil {
				return err
			}

			viewsToShow, err := getViewsToShow(viewsRequested, viewsExcluded, possibleViews)

			log.Info().Msgf("Exporting views: %s", strings.Join(viewsToShow, ", "))
			if err != nil {
				return err
			}

			allViews := make(map[string]*core.View)

			for _, viewName := range viewsToShow {
				view, err := results.RenderView(viewName)
				if err != nil {
					return err
				}
				allViews[viewName] = view
			}

			dbPath := args[0]

			log.Info().Msgf("Exporting %d views to %s", len(viewsToShow), dbPath)
			viewSlice := lo.MapToSlice(allViews, func(viewName string, view *core.View) *core.View {
				return view
			})
			err = SaveToDB(&SqlOptions{
				DatabaseName: dbPath,
				ReportId:     reportId,
				ScanTime:     reportDate,
			}, viewSlice)

			if err != nil {
				log.Error().Err(err).Msg("Error exporting to SQLite")
				log.Debug().Msgf("Error exporting to SQLite: %s", err)
				return err
			}
			log.Info().Msgf("Exported %d views to %s", len(viewsToShow), dbPath)
			return nil
		},
	}
	cmd.Flags().String(FlagReportId, "", "The report id")
	cmd.Flags().StringSlice(FlagViews, []string{}, "The views to export")
	cmd.Flags().StringSlice(FlagExcludeViews, []string{}, "The views to exclude from export")

	return cmd
}

func getViewsToShow(requested, excluded, possible []string) ([]string, error) {

	requested = lo.Filter(requested, func(item string, index int) bool {
		return item != ""
	})
	excluded = lo.Filter(excluded, func(item string, index int) bool {
		return item != ""
	})
	possible = lo.Filter(possible, func(item string, index int) bool {
		return item != ""
	})
	var toShow []string
	if len(requested) > 0 {
		leftover := lo.Without(requested, possible...)

		if len(leftover) > 0 {
			return nil, fmt.Errorf("unknown view(s): %s.\npossible views: %s", strings.Join(leftover, ", "), strings.Join(possible, ", "))
		}
		toShow = requested
	} else {
		toShow = possible
	}

	if len(excluded) == 0 {
		return toShow, nil
	}

	leftover := lo.Without(excluded, possible...)

	if len(leftover) > 0 {
		return nil, fmt.Errorf("can't exclude unknown view(s): %s\n possible views: %s", strings.Join(leftover, ","), strings.Join(toShow, ","))
	}

	return lo.Without(toShow, excluded...), nil
}

type SqlOptions struct {
	DatabaseName string

	ReportId string
	ScanTime time.Time
}

func SaveToDB(options *SqlOptions, views []*core.View) error {
	// check DB exists. If not, create it. If so, check tables exist.

	var db *sql.DB
	var err error
	// check if file exist
	if _, err = os.Stat(options.DatabaseName); errors.Is(err, os.ErrNotExist) {
		// create db
		db, err = createDb(options)
		if err != nil {
			return err
		}
	} else {
		// open db
		db, err = sql.Open("sqlite3", options.DatabaseName)
		if err != nil {
			return err
		}
	}

	err = ensureAllTablesExist(views, db)
	if err != nil {
		return err
	}

	// delete all data from tables
	err = deleteExistingReportFromAllTables(options.ReportId, views, db)
	if err != nil {
		return err
	}

	err = addMissingColumnsForViews(db, views)
	if err != nil {
		return err
	}

	err = insertRowsForAllViews(options, views, db)

	return err
}

// MaxParameters SQLITE_LIMIT_VARIABLE_NUMBER, see https://www.sqlite.org/limits.html#max_variable_number
const MaxParameters = 32766
const MaxChunkSize = 500

func calculateOptimumChunkSize(rows, columns int) int {
	maxParams := MaxParameters
	maxChunkSize := MaxChunkSize

	// Calculate the maximum rows allowed based on parameter limit.
	maxRowsByParams := maxParams / columns

	// Choose the smaller of the two limits.
	optimumChunkSize := int(math.Min(float64(maxRowsByParams), float64(maxChunkSize)))

	// Ensure the chunk size is at least 1, if columns is 0, we should return 0, otherwise it will cause divide by zero error.
	if columns == 0 {
		return 0
	}

	return optimumChunkSize
}

func insertRowsForAllViews(options *SqlOptions, views []*core.View, db *sql.DB) error {
	for _, view := range views {
		columnCount := len(view.Columns) + 2
		rowCount := len(view.Rows)

		chunkValue := calculateOptimumChunkSize(rowCount, columnCount)

		viewName := view.Name

		log.Info().Msgf("Inserting %d rows for view %s in chunks of %d rows", len(view.Rows), viewName, chunkValue)
		chunks := lo.Chunk(view.Rows, chunkValue)
		for _, chunk := range chunks {
			err := insertRowsForView(db, viewName, view.Columns, chunk, options)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func deleteExistingReportFromAllTables(reportId string, views []*core.View, db *sql.DB) error {
	for _, view := range views {
		_, err := db.Exec("DELETE FROM `"+view.Name+"` WHERE report_id = ?", reportId)
		if err != nil {
			return err
		}
	}
	return nil
}
func addMissingColumnsForViews(db *sql.DB, views []*core.View) error {
	for _, view := range views {
		err := addMissingColumnsForView(db, view)
		if err != nil {
			return err
		}
	}
	return nil
}
func addMissingColumnsForView(db *sql.DB, view *core.View) error {
	// get existing columns
	rows, err := db.Query("select name from pragma_table_info(?)", view.Name)
	if err != nil {
		return err
	}
	defer rows.Close()

	existingColumns := make(map[string]bool)
	for rows.Next() {
		if err := rows.Err(); err != nil {

		}
		var theName string
		hasResult := rows.Scan(&theName)
		existingColumns[theName] = true
		if hasResult != nil {

		}
	}

	// column difference
	for _, column := range view.Columns {
		if !existingColumns[column.Name] {
			// add column
			_, err := db.Exec("ALTER TABLE " + view.Name + " ADD COLUMN " + columnDDL(column))
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func createDb(options *SqlOptions) (*sql.DB, error) {
	db, err := sql.Open("sqlite3", options.DatabaseName)
	if err != nil {
		return nil, err
	}
	return db, nil
}
func insertRowsForView(db *sql.DB, name string, columns []*core.Column, rows []*core.Row, options *SqlOptions) error {
	extraColumns := []*core.Column{core.StringColumn("report_id"), core.DateColumn("timestamp")}
	allColumns := append(columns, extraColumns...)

	valueStrings := make([]string, 0, len(rows))
	amountOfArgumentsPerRow := len(allColumns)
	valueArgs := make([]interface{}, 0, len(rows)*amountOfArgumentsPerRow)

	valueStringTemplate := "(" + strings.Repeat("?, ", amountOfArgumentsPerRow-1) + "?)"
	for _, row := range rows {
		valueStrings = append(valueStrings, valueStringTemplate)

		for _, column := range allColumns {
			switch column.Name {
			case "report_id":
				valueArgs = append(valueArgs, options.ReportId)
			case "timestamp":
				valueArgs = append(valueArgs, options.ScanTime)
			default:
				switch column.Type {

				case core.PositionInFile:
					position := row.Data[column.Name].(*file.Position)
					valueArgs = append(valueArgs, fmt.Sprintf("%d:%d", position.Line, position.CharInLine))
				default:
					valueArgs = append(valueArgs, row.Data[column.Name])
				}

			}
		}
	}

	columnNames := lo.Map(allColumns, func(item *core.Column, index int) string {
		return "`" + item.Name + "`"
	})
	theSql := "INSERT INTO " + name + " (" + strings.Join(columnNames, ",") + ") VALUES " + strings.Join(valueStrings, ",")

	_, err := db.Exec(theSql, valueArgs...)
	if err != nil {
		return err
	}
	return nil
}

func ensureAllTablesExist(views []*core.View, db *sql.DB) error {
	for _, view := range views {
		err := ensureTableExists(db, view)

		if err != nil {
			return err
		}
	}
	return nil
}
func ensureTableExists(db *sql.DB, view *core.View) error {

	ddl := tableDDL(view)
	_, err := db.Exec(ddl)
	if err != nil {
		return err
	}
	return nil
}

func tableDDL(view *core.View) string {
	return "CREATE TABLE IF NOT EXISTS `" + view.Name + "` (" + columnsDDL(view) + ", report_id TEXT, timestamp DATE)"
}

func columnsDDL(view *core.View) string {
	columnDDLStrings := lo.Map(view.Columns, func(column *core.Column, index int) string {
		return columnDDL(column)
	})

	return strings.Join(columnDDLStrings, ",")
}

func columnDDL(column *core.Column) string {
	return "`" + column.Name + "` " + columnTypeDDL(column)
}

func columnTypeDDL(column *core.Column) string {
	switch column.Type {
	case core.String:
		return "TEXT"
	case core.Float:
		return "REAL"
	case core.Date:
		return "DATE"
	case core.PositionInFile:
		return "TEXT"
	default:
		return "INTEGER"
	}
}
