package basic

import (
	"github.com/archstats/archstats/core"
	definitions2 "github.com/archstats/archstats/core/definitions"
	"github.com/samber/lo"
)

func definitionsView(results *core.Results) *core.View {
	return &core.View{
		Columns: []*core.Column{
			core.StringColumn("id"),
			core.StringColumn("name"),
			core.StringColumn("short"),
			core.StringColumn("long"),
		},
		Rows: lo.MapToSlice(results.GetDefinitions(), func(_ string, definition *definitions2.Definition) *core.Row {
			return &core.Row{
				Data: core.RowData{
					"id":    definition.Id,
					"name":  definition.Name,
					"short": definition.Short,
					"long":  definition.Long,
				},
			}
		}),
	}
}
