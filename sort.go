package main

import (
	"sort"
)

func sortRows(sortFieldName string, resultsFromCommand *View) {
	if len(resultsFromCommand.rows) == 0 {
		return
	}

	aFieldExample := getFieldExample(resultsFromCommand.rows, sortFieldName)
	var lessFunc func(i int, j int) bool

	if sortFieldName == "" || aFieldExample == nil {
		defaultColumn := resultsFromCommand.OrderedColumns[0]
		lessFunc = getLessFunc(aFieldExample, resultsFromCommand.rows, defaultColumn)
	} else {
		lessFunc = getLessFunc(aFieldExample, resultsFromCommand.rows, sortFieldName)
	}
	sort.Slice(resultsFromCommand.rows, lessFunc)
}

func getFieldExample(resultsFromCommand []*Row, sortFieldName string) interface{} {
	for _, row := range resultsFromCommand {
		if row.Data[sortFieldName] != nil {
			return row.Data[sortFieldName]
		}
	}
	// if we get here, we didn't find a field with a value, very unlikely
	return 0
}

func getLessFunc(aFieldExample interface{}, resultsFromCommand []*Row, sortFieldName string) func(i int, j int) bool {
	switch aFieldExample.(type) {
	case string:
		return func(i, j int) bool {
			return resultsFromCommand[i].Data[sortFieldName].(string) > resultsFromCommand[j].Data[sortFieldName].(string)
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
