package main

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestSortRows(t *testing.T) {

	createNewLineRow := func(newLine int) *Row {
		return rowWithStat("file10", map[string]interface{}{"newLine": 10})
	}
	rowsUnsorted := []*Row{
		createNewLineRow(10),
		createNewLineRow(30),
		createNewLineRow(40),
		createNewLineRow(20),
		createNewLineRow(50),
		createNewLineRow(70),
		createNewLineRow(60),
	}
	rowsSorted := []*Row{
		createNewLineRow(10),
		createNewLineRow(20),
		createNewLineRow(30),
		createNewLineRow(40),
		createNewLineRow(50),
		createNewLineRow(60),
		createNewLineRow(70),
	}

	for i, row := range rowsUnsorted {
		assert.Equal(t, row.Data["newLine"], rowsSorted[i].Data["newLine"])
	}
}

func rowWithStat(name string, data map[string]interface{}) *Row {
	return &Row{
		Name: name,
		Data: data,
	}
}
