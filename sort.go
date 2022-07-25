package main

import (
	"github.com/RyanSusana/archstats/views"
	"sort"
)

func sortRows(sortFieldName string, resultsFromCommand *views.View) {
	if len(resultsFromCommand.Rows) == 0 {
		return
	}

	aFieldExample := getFieldExample(resultsFromCommand.Rows, sortFieldName)
	var lessFunc func(i int, j int) bool

	if sortFieldName == "" || aFieldExample == nil {
		defaultColumn := resultsFromCommand.OrderedColumns[0]
		lessFunc = getLessFunc(aFieldExample, resultsFromCommand.Rows, defaultColumn)
	} else {
		lessFunc = getLessFunc(aFieldExample, resultsFromCommand.Rows, sortFieldName)
	}
	sort.Slice(resultsFromCommand.Rows, lessFunc)
}

func getFieldExample(resultsFromCommand []*views.Row, sortFieldName string) interface{} {
	for _, row := range resultsFromCommand {
		if row.Data[sortFieldName] != nil {
			return row.Data[sortFieldName]
		}
	}
	// if we get here, we didn't find a field with a value, very unlikely
	return 0
}

func getLessFunc(aFieldExample interface{}, resultsFromCommand []*views.Row, sortFieldName string) func(i int, j int) bool {
	switch aFieldExample.(type) {
	case string:
		return func(i, j int) bool {
			return resultsFromCommand[i].Data[sortFieldName].(string) > resultsFromCommand[j].Data[sortFieldName].(string)
		}
	case float32, float64:
		return func(i, j int) bool {
			return resultsFromCommand[i].Data[sortFieldName].(float64) > resultsFromCommand[j].Data[sortFieldName].(float64)
		}
	default:
		return func(i, j int) bool {
			iValue, isIInteger := resultsFromCommand[i].Data[sortFieldName].(int)
			jValue, isJInteger := resultsFromCommand[j].Data[sortFieldName].(int)

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
