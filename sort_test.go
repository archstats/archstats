package main

import (
	"archstats/views"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestSortRows(t *testing.T) {

	createNewLineRow := func(newLine int) *views.Row {
		return &views.Row{
			Data: map[string]interface{}{"newLine": newLine},
		}
	}
	rowsUnsorted := []*views.Row{
		createNewLineRow(10),
		createNewLineRow(30),
		createNewLineRow(40),
		createNewLineRow(20),
		createNewLineRow(50),
		createNewLineRow(70),
		createNewLineRow(60),
	}
	rowsSorted := []*views.Row{
		createNewLineRow(70),
		createNewLineRow(60),
		createNewLineRow(50),
		createNewLineRow(40),
		createNewLineRow(30),
		createNewLineRow(20),
		createNewLineRow(10),
	}

	sortRows("newLine", &views.View{
		OrderedColumns: []string{"name", "newLine"},
		Rows:           rowsUnsorted,
	})
	for i, row := range rowsUnsorted {
		assert.Equal(t, row.Data["newLine"], rowsSorted[i].Data["newLine"])
	}
}
