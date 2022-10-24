package basic

import (
	"github.com/RyanSusana/archstats/analysis"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestSortRows(t *testing.T) {

	createNewLineRow := func(newLine int) *analysis.Row {
		return &analysis.Row{
			Data: map[string]interface{}{"newLine": newLine},
		}
	}
	rowsUnsorted := []*analysis.Row{
		createNewLineRow(10),
		createNewLineRow(30),
		createNewLineRow(40),
		createNewLineRow(20),
		createNewLineRow(50),
		createNewLineRow(70),
		createNewLineRow(60),
	}
	rowsSorted := []*analysis.Row{
		createNewLineRow(70),
		createNewLineRow(60),
		createNewLineRow(50),
		createNewLineRow(40),
		createNewLineRow(30),
		createNewLineRow(20),
		createNewLineRow(10),
	}

	SortRows("newLine", &analysis.View{
		Columns: []*analysis.Column{analysis.StringColumn("name"), analysis.IntColumn("newLine")},
		Rows:    rowsUnsorted,
	})
	for i, row := range rowsUnsorted {
		assert.Equal(t, row.Data["newLine"], rowsSorted[i].Data["newLine"])
	}
}
