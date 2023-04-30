package sqlite

import (
	"database/sql"
	"errors"
	"github.com/RyanSusana/archstats/analysis"
	"github.com/RyanSusana/archstats/cmd/common"
	_ "github.com/mattn/go-sqlite3"
	"github.com/samber/lo"
	"github.com/spf13/cobra"
	"os"
	"strings"
	"time"
)

const (
	//FlagSqliteDb = "db"
	FlagReportId = "report-id"
)

var Command = &cobra.Command{
	Use:   "sqlite",
	Short: "Export data to an SQLite database",
	Long:  `Export data to an SQLite database`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {

		reportId, _ := cmd.Flags().GetString(FlagReportId)
		results, _ := common.Analyze(cmd)
		var reportDate time.Time

		reportDate = time.Now()

		viewsToShow := lo.Map(results.GetAllViewFactories(), func(vf *analysis.ViewFactory, index int) string {
			return vf.Name
		})

		allViews := make(map[string]*analysis.View)

		for _, viewName := range viewsToShow {
			view, err := results.RenderView(viewName)
			if err != nil {
				return err
			}
			allViews[viewName] = view
		}

		dbPath := args[0]
		if dbPath == "" {
			return errors.New("sqlite-db is required")
		}

		viewSlice := lo.MapToSlice(allViews, func(viewName string, view *analysis.View) *analysis.View {
			return view
		})
		err := SaveToDB(&SqlOptions{
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

func init() {
	Command.Flags().String(FlagReportId, "", "The report id")
}

type SqlOptions struct {
	DatabaseName string

	ReportId string
	ScanTime time.Time
}

func SaveToDB(options *SqlOptions, views []*analysis.View) error {
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

func insertRowsForAllViews(options *SqlOptions, views []*analysis.View, db *sql.DB) error {
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

func deleteExistingReportFromAllTables(reportId string, views []*analysis.View, db *sql.DB) error {
	for _, view := range views {
		_, err := db.Exec("DELETE FROM `"+view.Name+"` WHERE report_id = ?", reportId)
		if err != nil {
			return err
		}
	}
	return nil
}
func addMissingColumnsForViews(db *sql.DB, views []*analysis.View) error {
	for _, view := range views {
		err := addMissingColumnsForView(db, view)
		if err != nil {
			return err
		}
	}
	return nil
}
func addMissingColumnsForView(db *sql.DB, view *analysis.View) error {
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
func insertRowsForView(db *sql.DB, name string, columns []*analysis.Column, rows []*analysis.Row, options *SqlOptions) error {
	extraColumns := []*analysis.Column{analysis.StringColumn("report_id"), analysis.DateColumn("timestamp")}
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
				valueArgs = append(valueArgs, row.Data[column.Name])
			}
		}
	}

	columnNames := lo.Map(allColumns, func(item *analysis.Column, index int) string {
		return "`" + item.Name + "`"
	})
	theSql := "INSERT INTO " + name + " (" + strings.Join(columnNames, ",") + ") VALUES " + strings.Join(valueStrings, ",")

	_, err := db.Exec(theSql, valueArgs...)
	if err != nil {
		return err
	}
	return nil
}

func ensureAllTablesExist(views []*analysis.View, db *sql.DB) error {
	for _, view := range views {
		err := ensureTableExists(db, view)

		if err != nil {
			return err
		}
	}
	return nil
}
func ensureTableExists(db *sql.DB, view *analysis.View) error {

	ddl := tableDDL(view)
	_, err := db.Exec(ddl)
	if err != nil {
		return err
	}
	return nil
}

func tableDDL(view *analysis.View) string {
	return "CREATE TABLE IF NOT EXISTS `" + view.Name + "` (" + columnsDDL(view) + ", report_id TEXT, timestamp DATE)"
}

func columnsDDL(view *analysis.View) string {
	columnDDLStrings := lo.Map(view.Columns, func(column *analysis.Column, index int) string {
		return columnDDL(column)
	})

	return strings.Join(columnDDLStrings, ",")
}

func columnDDL(column *analysis.Column) string {
	return "`" + column.Name + "` " + columnTypeDDL(column)
}

func columnTypeDDL(column *analysis.Column) string {
	switch column.Type {
	case analysis.String:
		return "TEXT"
	case analysis.Float:
		return "REAL"
	case analysis.Date:
		return "DATE"
	default:
		return "INTEGER"
	}
}
