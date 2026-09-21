package basic

import (
	"strings"

	"github.com/archstats/archstats/core"
	"github.com/archstats/archstats/core/unit"
)

// The named things in the codebase, whatever kind of thing the ecosystem
// makes its architecture out of.
//
// Not "classes". gin is 1,344 functions to 178 types and LibreChat is 997 to
// 14, so a view of classes shows fourteen things in a codebase of a thousand.
// And not one row per file either: nopCommerce declares 1,567 partial
// classes, each one type spread across several files, which is why a unit has
// files rather than a file.
func unitView(results *core.Results) *core.View {
	var rows []*core.Row
	for _, u := range results.Units {
		rows = append(rows, &core.Row{
			Data: core.RowData{
				"id":        u.ID,
				"kind":      u.Kind,
				"name":      u.Name,
				"component": u.Component,
				"module":    u.Module,
				"owner":     u.Owner,
				"file":      firstFile(u),
				"files":     len(u.Files),
				"markers":   markerSummary(u),
			},
		})
	}
	return &core.View{
		Columns: []*core.Column{
			core.StringColumn("id"),
			core.StringColumn("kind"),
			core.StringColumn("name"),
			core.StringColumn("component"),
			core.StringColumn("module"),
			core.StringColumn("owner"),
			core.StringColumn("file"),
			core.IntColumn("files"),
			core.StringColumn("markers"),
		},
		Rows: rows,
	}
}

// The file a person would open first. Units are sorted, so this is stable.
func firstFile(u *unit.Unit) string {
	if len(u.Files) == 0 {
		return ""
	}
	return u.Files[0]
}

// Markers as one readable field, qualified by the evidence they came from,
// because "annotation:Service" and "filename:models" are not the same claim
// and a reader should not have to guess which they are looking at.
func markerSummary(u *unit.Unit) string {
	seen := map[string]bool{}
	var parts []string
	for _, m := range u.Markers {
		key := m.Source + ":" + m.Key
		if m.Value != "" && m.Value != m.Key {
			key += "=" + m.Value
		}
		if seen[key] {
			continue
		}
		seen[key] = true
		parts = append(parts, key)
	}
	return strings.Join(parts, " ")
}

// Counts per kind for the summary, so "this project has units at all" is
// answerable without rendering the whole table.
func unitCounts(results *core.Results) []*core.Row {
	var rows []*core.Row
	for kind, units := range results.UnitsByKind {
		rows = append(rows, &core.Row{
			Data: core.RowData{
				NameColumn:  "unit_count__" + kind,
				ValueColumn: len(units),
			},
		})
	}
	return rows
}
