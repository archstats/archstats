package views

import "github.com/RyanSusana/archstats/snippets"

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
		OrderedColumns: []string{"value", "file", "directory", "component", "type", "begin", "end"},
		Rows:           toReturn,
	}
}
