package basic

import (
	"github.com/archstats/archstats/core"
	"github.com/samber/lo"
	"sort"
)

func SortRows(columnName string, resultsFromCommand *core.View) {
	column, columnFound := lo.Find(resultsFromCommand.Columns, func(column *core.Column) bool {
		return column.Name == columnName
	})

	if len(resultsFromCommand.Rows) == 0 {
		return
	}

	var lessFunc func(i int, j int) bool

	if columnFound {
		lessFunc = getLessFunc(resultsFromCommand.Rows, column)
	} else {
		defaultColumn := resultsFromCommand.Columns[0]
		lessFunc = getLessFunc(resultsFromCommand.Rows, defaultColumn)
	}
	sort.Slice(resultsFromCommand.Rows, lessFunc)
}

func getLessFunc(resultsFromCommand []*core.Row, column *core.Column) func(i int, j int) bool {
	columnName := column.Name
	if column.Type == core.String {
		return func(i, j int) bool {
			iValue, _ := resultsFromCommand[i].Data[columnName].(string)
			jValue, _ := resultsFromCommand[j].Data[columnName].(string)
			return iValue < jValue
		}
	}
	// Largest first. A value that is no number, a nil "unknown" among them,
	// sorts as zero rather than panicking the type assertion it used to.
	return func(i, j int) bool {
		return numeric(resultsFromCommand[i].Data[columnName]) > numeric(resultsFromCommand[j].Data[columnName])
	}
}

func numeric(v interface{}) float64 {
	switch n := v.(type) {
	case int:
		return float64(n)
	case int64:
		return float64(n)
	case float64:
		return n
	}
	return 0
}
