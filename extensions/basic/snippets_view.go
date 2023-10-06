package basic

import "github.com/archstats/archstats/core"

// TODO: this is a noisy, not insightful, view. But it's handy for something like `--raw-snippets
func snippetsView(results *core.Results) *core.View {
	toReturn := make([]*core.Row, 0, len(results.Snippets))
	for _, snippet := range results.Snippets {
		toReturn = append(toReturn, &core.Row{
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
	return &core.View{
		Columns: []*core.Column{
			core.StringColumn("content"),
			core.StringColumn("file"),
			core.StringColumn("component"),
			core.StringColumn("snippet_type"),
			core.PositionInFileColumn("begin_position"),
			core.PositionInFileColumn("end_position"),
		},
		Rows: toReturn,
	}
}
