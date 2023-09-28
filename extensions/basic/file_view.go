package basic

import (
	"github.com/RyanSusana/archstats/core"
	"github.com/RyanSusana/archstats/core/file"
	"github.com/samber/lo"
)

func fileView(results *core.Results) *core.View {
	view := genericView(getDistinctColumnsFromResults(results), results.StatsByFile)

	view.Columns = append(view.Columns, []*core.Column{core.StringColumn("directory"), core.StringColumn("component")}...)
	for _, row := range view.Rows {
		row.Data["directory"] = results.FileToDirectory[row.Data["name"].(string)]
		row.Data["component"] = results.FileToComponent[row.Data["name"].(string)]
	}
	view.Columns = lo.Filter(view.Columns, func(c *core.Column, _ int) bool {
		return c.Name != file.FileCount
	})
	return view
}
