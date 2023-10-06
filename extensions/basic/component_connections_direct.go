package basic

import (
	"github.com/archstats/archstats/core"
	"github.com/archstats/archstats/core/component"
	"github.com/samber/lo"
)

func componentConnectionsView(results *core.Results) *core.View {
	groupedConnections := lo.GroupBy(results.Connections, func(connection *component.Connection) string {
		return connection.From + ":" + connection.File + " -> " + connection.To
	})

	var rows []*core.Row
	for _, connections := range groupedConnections {
		connection := connections[0]
		rows = append(rows, &core.Row{
			Data: map[string]interface{}{
				"from":            connection.From,
				"to":              connection.To,
				"file":            connection.File,
				"reference_count": len(connections),
			},
		})
	}

	return &core.View{
		Name: "component_connections",
		Columns: []*core.Column{
			core.StringColumn("from"),
			core.StringColumn("to"),
			core.StringColumn("file"),
			core.IntColumn("reference_count"),
		},
		Rows: rows,
	}
}
