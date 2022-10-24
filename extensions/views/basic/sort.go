package basic

import (
	"github.com/RyanSusana/archstats/analysis"
	"github.com/samber/lo"
	"sort"
)

func SortRows(columnName string, resultsFromCommand *analysis.View) {
	column, columnFound := lo.Find(resultsFromCommand.Columns, func(column *analysis.Column) bool {
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

func getLessFunc(resultsFromCommand []*analysis.Row, column *analysis.Column) func(i int, j int) bool {
	columnName := column.Name
	switch column.Type {
	case analysis.String:
		return func(i, j int) bool {
			return resultsFromCommand[i].Data[columnName].(string) < resultsFromCommand[j].Data[columnName].(string)
		}
	case analysis.Float:
		return func(i, j int) bool {
			return resultsFromCommand[i].Data[columnName].(float64) > resultsFromCommand[j].Data[columnName].(float64)
		}
	default:
		return func(i, j int) bool {
			iValue, isIInteger := resultsFromCommand[i].Data[columnName].(int)
			jValue, isJInteger := resultsFromCommand[j].Data[columnName].(int)

			if !isIInteger {
				iValue = 0
			}

			if !isJInteger {
				jValue = 0
			}
			return iValue > jValue
		}
	}
}
