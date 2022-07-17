package main

import "regexp"

func filterRows(exp *regexp.Regexp, rows []*Row) []*Row {
	var filteredRows []*Row
	for _, row := range rows {
		if exp.MatchString(row.Name) {
			filteredRows = append(filteredRows, row)
		}
	}
	return filteredRows
}
