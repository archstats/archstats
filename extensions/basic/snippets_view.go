package basic

import "github.com/RyanSusana/archstats/analysis"

// TODO: this is a noisy, not insightful, view. But it's handy for something like `--raw-snippets
func snippetsView(results *analysis.Results) *analysis.View {
	toReturn := make([]*analysis.Row, 0, len(results.Snippets))
	for _, snippet := range results.Snippets {
		toReturn = append(toReturn, &analysis.Row{
			Data: map[string]interface{}{
				"file":           snippet.File,
				"component":      snippet.Component,
				"snippet_type":   snippet.Type,
				"begin_position": snippet.Begin,
				"end_position":   snippet.End,
				"content":        snippet.Value,
			},
		})
	}
	return &analysis.View{
		Columns: []*analysis.Column{
			analysis.StringColumn("content"),
			analysis.StringColumn("file"),
			analysis.StringColumn("component"),
			analysis.StringColumn("snippet_type"),
			analysis.PositionInFileColumn("begin_position"),
			analysis.PositionInFileColumn("end_position"),
		},
		Rows: toReturn,
	}
}
