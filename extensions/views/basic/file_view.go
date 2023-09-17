package basic

import (
	"github.com/RyanSusana/archstats/analysis"
	"github.com/RyanSusana/archstats/analysis/file"
	"github.com/samber/lo"
)

func fileView(results *analysis.Results) *analysis.View {
	view := genericView(getDistinctColumnsFromResults(results), results.StatsByFile)

	view.Columns = append(view.Columns, []*analysis.Column{analysis.StringColumn("directory"), analysis.StringColumn("component")}...)
	for _, row := range view.Rows {
		row.Data["directory"] = results.FileToDirectory[row.Data["name"].(string)]
		row.Data["component"] = results.FileToComponent[row.Data["name"].(string)]
	}
	view.Columns = lo.Filter(view.Columns, func(c *analysis.Column, _ int) bool {
		return c.Name != file.FileCount
	})
	return view
}
