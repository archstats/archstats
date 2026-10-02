package basic

import (
	"github.com/archstats/archstats/core"
)

// One unit using another.
//
// Java had this and nothing else did, because Java imports the type itself:
// `import com.acme.Order` names the unit. Every other language imports a
// module and takes names out of it, so the edge is only recoverable by
// resolving what an import clause took against what every module declares --
// which needs the whole codebase and therefore happens here.
//
// A reference naming something outside the codebase resolves to nothing and
// is dropped. `react` and `net/http` are real dependencies and not edges in
// this graph, and a resolver loose enough to invent one would fill it with
// units that do not exist.
func unitConnectionsView(results *core.Results) *core.View {
	var rows []*core.Row
	for _, c := range results.UnitConnections() {
		from, to := results.UnitByID[c.From], results.UnitByID[c.To]
		row := core.RowData{
			"from": c.From,
			"to":   c.To,
			// What the source called the module, kept so a reader can see
			// why the edge exists rather than having to trust it.
			"via": c.Via,
		}
		if from != nil {
			row["from_component"] = from.Component
			row["from_file"] = firstFile(from)
		}
		if to != nil {
			row["to_component"] = to.Component
			row["to_file"] = firstFile(to)
		}
		rows = append(rows, &core.Row{Data: row})
	}
	return &core.View{
		Columns: []*core.Column{
			core.StringColumn("from"),
			core.StringColumn("to"),
			core.StringColumn("via"),
			core.StringColumn("from_component"),
			core.StringColumn("to_component"),
			core.StringColumn("from_file"),
			core.StringColumn("to_file"),
		},
		Rows: rows,
	}
}
