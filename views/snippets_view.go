package views

import "github.com/RyanSusana/archstats/snippets"

//TODO: this is a noisy, not insightful, view. But it's handy for something like `--raw-snippets
func SnippetsView(results *snippets.Results) *View {
	toReturn := make([]*Row, 0, len(results.Snippets))
	for _, snippet := range results.Snippets {
		toReturn = append(toReturn, &Row{
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
	return &View{
		Columns: []*Column{
			StringColumn("value"),
			StringColumn("file"),
			StringColumn("directory"),
			StringColumn("component"),
			StringColumn("type"),
			IntColumn("begin"),
			IntColumn("end"),
		},
		Rows: toReturn,
	}
}
