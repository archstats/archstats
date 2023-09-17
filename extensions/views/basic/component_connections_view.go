package basic

import (
	"github.com/RyanSusana/archstats/analysis"
	"github.com/RyanSusana/archstats/analysis/component"
	"github.com/samber/lo"
)

func componentConnectionsView(results *analysis.Results) *analysis.View {
	groupedConnections := lo.GroupBy(results.Connections, func(connection *component.Connection) string {
		return connection.From + ":" + connection.File + " -> " + connection.To
	})

	var rows []*analysis.Row
	for _, connections := range groupedConnections {
		connection := connections[0]
		rows = append(rows, &analysis.Row{
			Data: map[string]interface{}{
				"from":  connection.From,
				"to":    connection.To,
				"file":  connection.File,
				"count": len(connections),
			},
		})
	}

	return &analysis.View{
		Name: "component_connections",
		Columns: []*analysis.Column{
			analysis.StringColumn("from"),
			analysis.StringColumn("to"),
			analysis.StringColumn("file"),
			analysis.IntColumn("count"),
		},
		Rows: rows,
	}
}
