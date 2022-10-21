package views

import (
	"github.com/RyanSusana/archstats/analysis"
)

func FileView(results *analysis.Results) *View {
	view := GenericView(getDistinctColumnsFromResults(results), results.StatsByFile)

	view.Columns = append(view.Columns, []*Column{StringColumn("component"), StringColumn("directory")}...)
	for _, row := range view.Rows {
		row.Data["component"] = results.FileToComponent[row.Data["name"].(string)]
		row.Data["directory"] = results.FileToDirectory[row.Data["name"].(string)]
	}
	return view
}
