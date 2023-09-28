package basic

import (
	"github.com/RyanSusana/archstats/core"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestSortRows(t *testing.T) {

	createNewLineRow := func(newLine int) *core.Row {
		return &core.Row{
			Data: map[string]interface{}{"newLine": newLine},
		}
	}
	rowsUnsorted := []*core.Row{
		createNewLineRow(10),
		createNewLineRow(30),
		createNewLineRow(40),
		createNewLineRow(20),
		createNewLineRow(50),
		createNewLineRow(70),
		createNewLineRow(60),
	}
	rowsSorted := []*core.Row{
		createNewLineRow(70),
		createNewLineRow(60),
		createNewLineRow(50),
		createNewLineRow(40),
		createNewLineRow(30),
		createNewLineRow(20),
		createNewLineRow(10),
	}

	SortRows("newLine", &core.View{
		Columns: []*core.Column{core.StringColumn("name"), core.IntColumn("newLine")},
		Rows:    rowsUnsorted,
	})
	for i, row := range rowsUnsorted {
		assert.Equal(t, row.Data["newLine"], rowsSorted[i].Data["newLine"])
	}
}
