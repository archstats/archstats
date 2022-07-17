package main

import (
	"sort"
	"strings"
)

func sortRows(sortFieldName string, resultsFromCommand []*Row) {
	if len(resultsFromCommand) == 0 {
		return
	}

	aFieldExample := getFieldExample(resultsFromCommand, sortFieldName)
	if sortFieldName == "" || aFieldExample == nil {
		sort.Slice(resultsFromCommand, func(i, j int) bool {
			return strings.Compare(resultsFromCommand[i].Name, resultsFromCommand[j].Name) == 0
		})
	} else {
		lessFunc := getLessFunc(aFieldExample, resultsFromCommand, sortFieldName)
		sort.Slice(resultsFromCommand, lessFunc)
	}
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
