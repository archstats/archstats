package main

import (
	"github.com/stretchr/testify/assert"
	"strconv"
	"testing"
)

func TestSortRows(t *testing.T) {

	createNewLineRow := func(newLine int) *Row {
		return &Row{
			Name: "file" + strconv.Itoa(newLine),
			Data: map[string]interface{}{"newLine": newLine},
		}
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
		createNewLineRow(70),
		createNewLineRow(60),
		createNewLineRow(50),
		createNewLineRow(40),
		createNewLineRow(30),
		createNewLineRow(20),
		createNewLineRow(10),
	}

	sortRows("newLine", rowsUnsorted)
	for i, row := range rowsUnsorted {
		assert.Equal(t, row.Data["newLine"], rowsSorted[i].Data["newLine"])
	}
}