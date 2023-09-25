package basic

import (
	"github.com/RyanSusana/archstats/analysis"
	definitions2 "github.com/RyanSusana/archstats/analysis/definitions"
	"github.com/samber/lo"
)

func definitionsView(results *analysis.Results) *analysis.View {
	return &analysis.View{
		Columns: []*analysis.Column{
			analysis.StringColumn("name"),
			analysis.StringColumn("short"),
			analysis.StringColumn("long"),
		},
		Rows: lo.MapToSlice(results.GetDefinitions(), func(_ string, definition *definitions2.Definition) *analysis.Row {
			return &analysis.Row{
				Data: analysis.RowData{
					"name":  definition.Name,
					"short": definition.Short,
					"long":  definition.Long,
				},
			}
		}),
	}
}
