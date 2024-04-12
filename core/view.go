package core

import (
	"github.com/archstats/archstats/core/file"
	"github.com/samber/lo"
	"time"
)

type View struct {
	Name    string
	Columns []*Column
	Rows    []*Row
}

func CreateViewFromRows(name string, rows []*Row) *View {
	distinctColumns := make(map[string]*Column)
	for _, row := range rows {
		columns := getColumnsFromRow(row)
		for _, column := range columns {
			distinctColumns[column.Name] = column
		}
	}
	return &View{
		Name:    name,
		Columns: lo.Values(distinctColumns),
		Rows:    rows,
	}
}

func getColumnsFromRow(row *Row) []*Column {
	data := row.Data
	columns := make([]*Column, 0, len(data))
	for column, data := range data {
		switch data.(type) {
		case int:
			columns = append(columns, IntColumn(column))
		case float64:
			columns = append(columns, FloatColumn(column))
		case string:
			columns = append(columns, StringColumn(column))
		case file.Position:
			columns = append(columns, PositionInFileColumn(column))
		case time.Time:
			columns = append(columns, DateColumn(column))
		}
	}
	return columns
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
)

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
func DateColumn(name string) *Column {
	return &Column{
		Name: name,
		Type: Date,
	}
}
