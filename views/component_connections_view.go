package views

import "github.com/RyanSusana/archstats/snippets"

func ComponentConnectionsView(results *snippets.Results) *View {
	connections := getConnectionsWithCount(results)

	var rows []*Row
	for _, connection := range connections {
		rows = append(rows, &Row{
			Data: map[string]interface{}{
				"from":  connection.from,
				"to":    connection.to,
				"count": connection.count,
			},
		})
	}

	return &View{
		Name: "component_connections",
		Columns: []*Column{
			StringColumn("from"),
			StringColumn("to"),
			IntColumn("count"),
		},
		Rows: rows,
	}
}

func getConnectionsWithCount(results *snippets.Results) []*connectionWithCount {
	connections := make([]*connectionWithCount, 0, len(results.Connections))
	grouped := snippets.GroupConnectionsBy(results.Connections, func(connection *snippets.ComponentConnection) string {
		return connection.From + " -> " + connection.To
	})

	for connectionName, groupedConnections := range grouped {
		connections = append(connections, &connectionWithCount{
			name:  connectionName,
			count: len(groupedConnections),
			from:  groupedConnections[0].From,
			to:    groupedConnections[0].To,
		})
	}
	return connections
}

type connectionWithCount struct {
	name  string
	from  string
	to    string
	count int
}
