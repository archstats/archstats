package basic

import (
	"github.com/archstats/archstats/core"
	"github.com/archstats/archstats/core/file"
	"github.com/archstats/archstats/extensions/util"
	"github.com/samber/lo"
)

func fileView(results *core.Results) *core.View {
	view := util.GenericView(util.GetDistinctColumnsFrom(results.StatsByFile), results.StatsByFile)

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
