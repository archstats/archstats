package main

import (
	"sort"
	"strings"
)

func sortRows(generalOptions *GeneralOptions, resultsFromCommand []*Row) {
	if len(resultsFromCommand) == 0 {
		return
	}

	sortFieldName := generalOptions.SortedBy
	aFieldExample := resultsFromCommand[0].Data[sortFieldName]
	if sortFieldName == "" || aFieldExample == nil {
		sort.Slice(resultsFromCommand, func(i, j int) bool {
			return strings.Compare(resultsFromCommand[i].Name, resultsFromCommand[j].Name) == 0
		})
	} else {
		lessFunc := getLessFunc(aFieldExample, resultsFromCommand, sortFieldName)
		sort.Slice(resultsFromCommand, lessFunc)
	}
}

func getLessFunc(aFieldExample interface{}, resultsFromCommand []*Row, sortFieldName string) func(i int, j int) bool {
	var lessFunc func(i, j int) bool
	switch aFieldExample.(type) {
	case int:
		lessFunc = func(i, j int) bool {
			return resultsFromCommand[i].Data[sortFieldName].(int) > resultsFromCommand[j].Data[sortFieldName].(int)
		}
	case string:
		lessFunc = func(i, j int) bool {
			return resultsFromCommand[i].Data[sortFieldName].(string) > resultsFromCommand[j].Data[sortFieldName].(string)
		}
	}
	return lessFunc
}
