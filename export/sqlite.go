package export

import (
	"database/sql"
	"errors"
	"fmt"
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
}

func SaveToDB(options *SqlOptions, views []*views.View) error {
	// check DB exists. If not, create it. If so, check tables exist.

	var db *sql.DB
	// check if file exist
	if _, err := os.Stat(options.DatabaseName); errors.Is(err, os.ErrNotExist) {
		// create db
		db, err = createDb(options)

		if err != nil {
			return err
		}
	}

	var allErrors []error
	for _, view := range views {
		err := ensureTableExists(db, view)

		if err != nil {
			allErrors = append(allErrors, err)
			fmt.Println(err)
		}
	}
	// combine errors
	if len(allErrors) > 0 {
		return errors.New("errors occurred creating tables")
	}
	for _, view := range views {
		err := executeRowsDML(db, view, options)
		if err != nil {
			allErrors = append(allErrors, err)
			fmt.Println(err)
		}
	}

	// combine errors
	if len(allErrors) > 0 {
		return errors.New("errors occurred inserting data")
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
func executeRowsDML(db *sql.DB, view *views.View, options *SqlOptions) error {
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
				valueArgs = append(valueArgs, time.Now())
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

func ensureTableExists(db *sql.DB, view *views.View) error {

	ddl := tableDDL(view)
	_, err := db.Exec(ddl)
	if err != nil {
		return err
	}
	return nil
}

func tableDDL(view *views.View) string {
	return "CREATE TABLE IF NOT EXISTS `" + view.Name + "` (" + columnDDL(view) + "report_id TEXT, timestamp DATE)"
}

func columnDDL(view *views.View) string {
	var columns string
	for _, column := range view.Columns {
		columns += "`" + column.Name + "` " + columnTypeDDL(column) + ", "
	}
	return columns
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
