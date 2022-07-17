package main

import (
	"github.com/stretchr/testify/assert"
	"regexp"
	"testing"
)

func TestFilterRows(t *testing.T) {
	rows := []*Row{
		{Name: "file1"},
		{Name: "file2"},
		{Name: "file3"},
		{Name: "file4"},
		{Name: "file5"},
		{Name: "file6"},
		{Name: "nah fam"},
		{Name: "file7"},
	}
	exp := regexp.MustCompile("file\\d")
	filteredRows := filterRows(exp, rows)
	assert.Len(t, filteredRows, 7)
	assert.NotContains(t, toNames(filteredRows), "nah fam")
}

func toNames(rows []*Row) interface{} {
	var names []string
	for _, row := range rows {
		names = append(names, row.Name)
	}
	return names
}
