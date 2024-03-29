package sqlite

import (
	"database/sql"
	"errors"
	"fmt"
	"github.com/archstats/archstats/cmd/common"
	"github.com/archstats/archstats/core"
	"github.com/archstats/archstats/core/file"
	_ "github.com/mattn/go-sqlite3"
	"github.com/samber/lo"
	"github.com/spf13/cobra"
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

			reportId, err := cmd.Flags().GetString(FlagReportId)
			if err != nil {
				return err
			}
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

			viewSlice := lo.MapToSlice(allViews, func(viewName string, view *core.View) *core.View {
				return view
			})
			err = SaveToDB(&SqlOptions{
				DatabaseName: dbPath,
				ReportId:     reportId,
				ScanTime:     reportDate,
			}, viewSlice)
			if err != nil {
				return err
			}

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

func insertRowsForAllViews(options *SqlOptions, views []*core.View, db *sql.DB) error {
	for _, view := range views {
		chunks := lo.Chunk(view.Rows, 500)
		for _, chunk := range chunks {
			err := insertRowsForView(db, view.Name, view.Columns, chunk, options)
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
