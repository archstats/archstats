package components

import (
	"github.com/archstats/archstats/core"
	"github.com/archstats/archstats/core/component"
	"github.com/samber/lo"
)

// Every edge between components, including the ones the compiler erases.
//
// Coupling metrics are computed from results.Connections, which holds only
// the runtime edges. This view reads AllConnections and names each edge's
// kind instead, because "LibreChat's client depends on data-provider, but
// only for its types" is a thing an architect wants to see rather than have
// silently dropped.
func ConnectionsView(results *core.Results) *core.View {
	groupedConnections := lo.GroupBy(results.AllConnections, func(connection *component.Connection) string {
		return connection.From + ":" + connection.File + ":" + connection.Kind() + " -> " + connection.To
	})

	var rows []*core.Row
	for _, connections := range groupedConnections {
		connection := connections[0]
		rows = append(rows, &core.Row{
			Data: map[string]interface{}{
				"from":            connection.From,
				"to":              connection.To,
				"kind":            connection.Kind(),
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
			core.StringColumn("kind"),
			core.StringColumn("file"),
			core.IntColumn("reference_count"),
		},
		Rows: rows,
	}
}
