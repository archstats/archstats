package basic

import "github.com/RyanSusana/archstats/analysis"

// TODO: this is a noisy, not insightful, view. But it's handy for something like `--raw-snippets
func snippetsView(results *analysis.Results) *analysis.View {
	toReturn := make([]*analysis.Row, 0, len(results.Snippets))
	for _, snippet := range results.Snippets {
		toReturn = append(toReturn, &analysis.Row{
			Data: map[string]interface{}{
				"file":      snippet.File,
				"directory": snippet.Directory,
				"component": snippet.Component,
				"type":      snippet.Type,
				"begin":     snippet.Begin,
				"end":       snippet.End,
				"value":     snippet.Value,
			},
		})
	}
	return &analysis.View{
		Columns: []*analysis.Column{
			analysis.StringColumn("value"),
			analysis.StringColumn("file"),
			analysis.StringColumn("directory"),
			analysis.StringColumn("component"),
			analysis.StringColumn("type"),
			analysis.PositionInFileColumn("begin"),
			analysis.PositionInFileColumn("end"),
		},
		Rows: toReturn,
	}
}
