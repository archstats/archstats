package export

import (
	"database/sql"
	"errors"
	"github.com/RyanSusana/archstats/views"
	_ "github.com/mattn/go-sqlite3"
	"github.com/samber/lo"
	"os"
	"strings"
	"time"
)

type SqlOptions struct {
	DatabaseName string

	ReportId string
	ScanTime time.Time
}

func SaveToDB(options *SqlOptions, views []*views.View) error {
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

func insertRowsForAllViews(options *SqlOptions, views []*views.View, db *sql.DB) error {
	for _, view := range views {
		err := insertRowsForView(db, view, options)
		if err != nil {
			return err
		}
	}
	return nil
}

func deleteExistingReportFromAllTables(reportId string, views []*views.View, db *sql.DB) error {
	for _, view := range views {
		_, err := db.Exec("DELETE FROM `"+view.Name+"` WHERE report_id = ?", reportId)
		if err != nil {
			return err
		}
	}
	return nil
}
func addMissingColumnsForViews(db *sql.DB, views []*views.View) error {
	for _, view := range views {
		err := addMissingColumnsForView(db, view)
		if err != nil {
			return err
		}
	}
	return nil
}
func addMissingColumnsForView(db *sql.DB, view *views.View) error {
	// get existing columns
	rows, err := db.Query("select name from pragma_table_info(?)", view.Name)
	if err != nil {
		return err
	}
	defer rows.Close()

	existingColumns := make(map[string]bool)
	for rows.Next() {
		var theName string
		rows.Scan(&theName)
		existingColumns[theName] = true
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
func insertRowsForView(db *sql.DB, view *views.View, options *SqlOptions) error {
	extraColumns := []*views.Column{views.StringColumn("report_id"), views.DateColumn("timestamp")}
	allColumns := append(view.Columns, extraColumns...)

	valueStrings := make([]string, 0, len(view.Rows))
	amountOfArgumentsPerRow := len(allColumns)
	valueArgs := make([]interface{}, 0, len(view.Rows)*amountOfArgumentsPerRow)

	valueStringTemplate := "(" + strings.Repeat("?, ", amountOfArgumentsPerRow-1) + "?)"
	for _, row := range view.Rows {
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

	columnNames := lo.Map(allColumns, func(item *views.Column, index int) string {
		return "`" + item.Name + "`"
	})
	theSql := "INSERT INTO " + view.Name + " (" + strings.Join(columnNames, ",") + ") VALUES " + strings.Join(valueStrings, ",")

	_, err := db.Exec(theSql, valueArgs...)
	if err != nil {
		return err
	}
	return nil
}

func ensureAllTablesExist(views []*views.View, db *sql.DB) error {
	for _, view := range views {
		err := ensureTableExists(db, view)

		if err != nil {
			return err
		}
	}
	return nil
}
func ensureTableExists(db *sql.DB, view *views.View) error {

	ddl := tableDDL(view)
	_, err := db.Exec(ddl)
	if err != nil {
		return err
	}
	return nil
}

func tableDDL(view *views.View) string {
	return "CREATE TABLE IF NOT EXISTS `" + view.Name + "` (" + columnsDDL(view) + ", report_id TEXT, timestamp DATE)"
}

func columnsDDL(view *views.View) string {
	columnDDLStrings := lo.Map(view.Columns, func(column *views.Column, index int) string {
		return columnDDL(column)
	})

	return strings.Join(columnDDLStrings, ",")
}

func columnDDL(column *views.Column) string {
	return "`" + column.Name + "` " + columnTypeDDL(column)
}

func columnTypeDDL(column *views.Column) string {
	switch column.Type {
	case views.String:
		return "TEXT"
	case views.Float:
		return "REAL"
	case views.Date:
		return "DATE"
	default:
		return "INTEGER"
	}
}
