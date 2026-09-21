package basic

import (
	"github.com/archstats/archstats/core"
	"github.com/archstats/archstats/core/file"
	"github.com/archstats/archstats/core/stats"
	"github.com/archstats/archstats/extensions/util"
	"github.com/samber/lo"
)

func getStatsByFile(results *core.Results) stats.StatsGroup {
	return results.CalculateAccumulatedStatRecords(results.StatRecordsByFile)
}
func fileView(results *core.Results) *core.View {
	statsByFile := getStatsByFile(results)
	view := util.GenericView(util.GetDistinctColumnsFrom(statsByFile), statsByFile)

	view.Columns = append(view.Columns, []*core.Column{core.StringColumn("directory"), core.StringColumn("component"), core.StringColumn("module")}...)
	for _, row := range view.Rows {
		name := row.Data["name"].(string)
		row.Data["directory"] = results.FileToDirectory[name]
		row.Data["component"] = results.FileToComponent[name]
		// Empty when the project declares no modules, which is most
		// single-package repositories.
		row.Data["module"] = results.FileToModule[name]
	}
	view.Columns = lo.Filter(view.Columns, func(c *core.Column, _ int) bool {
		return c.Name != file.FileCount
	})
	return view
}
