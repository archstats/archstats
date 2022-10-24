package basic

import (
	"github.com/RyanSusana/archstats/analysis"
)

func fileView(results *analysis.Results) *analysis.View {
	view := genericView(getDistinctColumnsFromResults(results), results.StatsByFile)

	view.Columns = append(view.Columns, []*analysis.Column{analysis.StringColumn("component"), analysis.StringColumn("directory")}...)
	for _, row := range view.Rows {
		row.Data["component"] = results.FileToComponent[row.Data["name"].(string)]
		row.Data["directory"] = results.FileToDirectory[row.Data["name"].(string)]
	}
	return view
}
