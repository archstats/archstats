package core

import (
	"fmt"
	"github.com/archstats/archstats/core/file"
	"github.com/samber/lo"
	"sort"
	"time"
)

type View struct {
	Name    string
	Columns []*Column
	Rows    []*Row
}

// CreateViewFromRows infers a view's columns from its rows. A view should
// declare its columns instead: with no rows there is nothing to infer from,
// and the table comes out with none, so every query naming one fails.
// Columns are in name order, so the same rows always make the same table.
func CreateViewFromRows(name string, rows []*Row) *View {
	distinctColumns := make(map[string]*Column)
	for _, row := range rows {
		columns := getColumnsFromRow(row)
		for _, column := range columns {
			distinctColumns[column.Name] = column
		}
	}
	columns := lo.Values(distinctColumns)
	sort.Slice(columns, func(i, j int) bool { return columns[i].Name < columns[j].Name })
	return &View{
		Name:    name,
		Columns: columns,
		Rows:    rows,
	}
}

func getColumnsFromRow(row *Row) []*Column {
	data := row.Data
	columns := make([]*Column, 0, len(data))
	for column, data := range data {
		if t, ok := ColumnTypeOf(data); ok {
			columns = append(columns, &Column{Name: column, Type: t})
		}
	}
	return columns
}

// ColumnTypeOf is the column a value is stored in, and false for a value the
// export cannot store (nil included: it says nothing about its column).
func ColumnTypeOf(value interface{}) (ColumnType, bool) {
	switch value.(type) {
	case int, int64, int32, bool:
		return Integer, true
	case float64, float32:
		return Float, true
	case string:
		return String, true
	case *file.Position:
		return PositionInFile, true
	case time.Time:
		return Date, true
	}
	return 0, false
}

// Check lists what the export would lose or misread in this view: a row
// value under no declared column is dropped, and a value of another type
// than its column's is stored as something the column does not promise.
// Each problem is reported once per column.
func (v *View) Check() []string {
	declared := make(map[string]ColumnType, len(v.Columns))
	var problems []string
	for _, c := range v.Columns {
		if _, twice := declared[c.Name]; twice {
			problems = append(problems, fmt.Sprintf("%s: column %s declared twice", v.Name, c.Name))
		}
		declared[c.Name] = c.Type
	}
	reported := map[string]bool{}
	report := func(column, problem string) {
		if !reported[column] {
			reported[column] = true
			problems = append(problems, fmt.Sprintf("%s.%s: %s", v.Name, column, problem))
		}
	}
	for _, row := range v.Rows {
		for column, value := range row.Data {
			want, ok := declared[column]
			if !ok {
				report(column, "in the rows but not a declared column, so never exported")
				continue
			}
			if value == nil {
				continue
			}
			got, storable := ColumnTypeOf(value)
			switch {
			case !storable:
				report(column, fmt.Sprintf("holds a %T, which the export cannot store", value))
			case got != want && !((want == Float || want == Number) && got == Integer) && !(want == Number && got == Float):
				report(column, fmt.Sprintf("declared %s but holds a %T", want, value))
			}
		}
	}
	sort.Strings(problems)
	return problems
}

type ViewFactory struct {
	Name           string
	CreateViewFunc ViewFactoryFunction
}
type ViewFactoryFunction func(results *Results) *View

type RowData map[string]interface{}

type Row struct {
	Data RowData
}
type ColumnType int

const (
	Integer ColumnType = iota
	Float
	String
	Date
	PositionInFile
	// Number holds integers and fractions in one column, as a name/value
	// table does. SQLite stores it with NUMERIC affinity.
	Number
)

func (t ColumnType) String() string {
	switch t {
	case Integer:
		return "integer"
	case Float:
		return "float"
	case String:
		return "string"
	case Date:
		return "date"
	case PositionInFile:
		return "position"
	case Number:
		return "number"
	}
	return fmt.Sprintf("type %d", int(t))
}

type Column struct {
	Name string
	Type ColumnType
}

func StringColumn(name string) *Column {
	return &Column{
		Name: name,
		Type: String,
	}
}
func IntColumn(name string) *Column {
	return &Column{
		Name: name,
		Type: Integer,
	}
}
func PositionInFileColumn(name string) *Column {
	return &Column{
		Name: name,
		Type: PositionInFile,
	}
}

func FloatColumn(name string) *Column {
	return &Column{
		Name: name,
		Type: Float,
	}
}
func NumberColumn(name string) *Column {
	return &Column{
		Name: name,
		Type: Number,
	}
}
func DateColumn(name string) *Column {
	return &Column{
		Name: name,
		Type: Date,
	}
}
