package views

import "github.com/RyanSusana/archstats/snippets"

func ComponentConnectionsView(results *snippets.Results) *View {
	connections := make([]*Row, 0, len(results.Connections))
	grouped := snippets.GroupConnectionsBy(results.Connections, func(connection *snippets.ComponentConnection) string {
		return connection.From + " -> " + connection.To
	})

	for connectionName, groupedConnections := range grouped {
		connections = append(connections, &Row{
			Data: map[string]interface{}{
				"name":  connectionName,
				"from":  groupedConnections[0].From,
				"to":    groupedConnections[0].To,
				"count": len(groupedConnections),
			},
		})
	}
	return &View{
		OrderedColumns: []string{"from", "to", "count"},
		Rows:           connections,
	}
}
