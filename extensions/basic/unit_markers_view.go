package basic

import (
	"github.com/archstats/archstats/core"
)

// Every piece of evidence about every unit, one row each.
//
// The units view flattens markers into a readable summary, which is right for
// a person and wrong for a reader: a value can contain a space, and anything
// parsing the summary back apart is guessing. This is the same data with the
// structure kept.
//
// Source is the kind of evidence, and it matters as much as the key does.
// "filename:models" and "annotation:models" are different claims -- Django
// puts the role in the filename, Java in an annotation, Go in a struct tag --
// and a consumer that flattens them has thrown away the thing that says how
// much to trust it.
func unitMarkerView(results *core.Results) *core.View {
	var rows []*core.Row
	for _, u := range results.Units {
		for _, m := range u.Markers {
			rows = append(rows, &core.Row{
				Data: core.RowData{
					"unit":   u.ID,
					"kind":   u.Kind,
					"source": m.Source,
					"key":    m.Key,
					"value":  m.Value,
				},
			})
		}
	}
	return &core.View{
		Columns: []*core.Column{
			core.StringColumn("unit"),
			core.StringColumn("kind"),
			core.StringColumn("source"),
			core.StringColumn("key"),
			core.StringColumn("value"),
		},
		Rows: rows,
	}
}
