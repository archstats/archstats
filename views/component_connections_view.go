package views

import (
	"github.com/RyanSusana/archstats/snippets"
	"github.com/samber/lo"
)

type connectionFileCount struct {
	from  string
	to    string
	file  string
	count int
}

func ComponentConnectionsView(results *snippets.Results) *View {
	groupedConnections := lo.GroupBy(results.Connections, func(connection *snippets.ComponentConnection) string {
		return connection.From + ":" + connection.File + " -> " + connection.To
	})

	var rows []*Row
	for _, connections := range groupedConnections {
		connection := connections[0]
		rows = append(rows, &Row{
			Data: map[string]interface{}{
				"from":  connection.From,
				"to":    connection.To,
				"file":  connection.File,
				"count": len(connections),
			},
		})
	}

	return &View{
		Name: "component_connections",
		Columns: []*Column{
			StringColumn("from"),
			StringColumn("to"),
			StringColumn("file"),
			IntColumn("count"),
		},
		Rows: rows,
	}
}
