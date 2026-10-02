package navigation

import (
	"github.com/archstats/archstats/core"
)

func (e *extension) entryPointsView(*core.Results) *core.View {
	rows := make([]*core.Row, 0, len(e.entryRows))
	for _, r := range e.entryRows {
		rows = append(rows, &core.Row{Data: core.RowData{
			"kind": r.Kind, "method": r.Method, "path": r.Path, "framework": r.Framework, "handler": r.Handler,
			"unit": r.Unit, "function": r.Function, "component": r.Component, "file": r.File, "line": r.Line,
		}})
	}
	return &core.View{Columns: columns("kind", "method", "path", "framework", "handler", "unit", "function", "component", "file", "#line"), Rows: rows}
}

func (e *extension) entitiesView(*core.Results) *core.View {
	rows := make([]*core.Row, 0, len(e.entityRows))
	for _, r := range e.entityRows {
		rows = append(rows, &core.Row{Data: core.RowData{
			"entity": r.Entity, "name": r.Name, "table": r.Table, "store": r.Store, "framework": r.Framework, "file": r.File, "line": r.Line,
		}})
	}
	return &core.View{Columns: columns("entity", "name", "table", "store", "framework", "file", "#line"), Rows: rows}
}

func (e *extension) accessView(*core.Results) *core.View {
	rows := make([]*core.Row, 0, len(e.accessRows))
	for _, r := range e.accessRows {
		rows = append(rows, &core.Row{Data: core.RowData{
			"unit": r.Unit, "function": r.Function, "component": r.Component, "target": r.Target, "entity": r.Entity,
			"access": r.Access, "via": r.Via, "file": r.File, "line": r.Line,
		}})
	}
	return &core.View{Columns: columns("unit", "function", "component", "target", "entity", "access", "via", "file", "#line"), Rows: rows}
}

func (e *extension) supertypesView(*core.Results) *core.View {
	rows := make([]*core.Row, 0, len(e.superRows))
	for _, r := range e.superRows {
		rows = append(rows, &core.Row{Data: core.RowData{"unit": r.Unit, "supertype": r.Supertype, "name": r.Name}})
	}
	return &core.View{Columns: columns("unit", "supertype", "name"), Rows: rows}
}

func (e *extension) bindingsView(*core.Results) *core.View {
	rows := make([]*core.Row, 0, len(e.bindingRows))
	for _, r := range e.bindingRows {
		rows = append(rows, &core.Row{Data: core.RowData{
			"interface": r.Interface, "interface_unit": r.InterfaceUnit, "implementation": r.Implementation,
			"implementation_unit": r.ImplementationUnit, "mechanism": r.Mechanism, "file": r.File, "line": r.Line,
		}})
	}
	return &core.View{Columns: columns("interface", "interface_unit", "implementation", "implementation_unit", "mechanism", "file", "#line"), Rows: rows}
}

func (e *extension) docsView(*core.Results) *core.View {
	rows := make([]*core.Row, 0, len(e.docRows))
	for _, d := range e.docRows {
		rows = append(rows, &core.Row{Data: core.RowData{
			"file": d.File, "kind": d.Kind, "title": d.Title, "status": d.Status, "date": d.Date, "words": d.Words,
		}})
	}
	return &core.View{Columns: columns("file", "kind", "title", "status", "date", "#words"), Rows: rows}
}

func (e *extension) docLinksView(*core.Results) *core.View {
	rows := make([]*core.Row, 0, len(e.linkRows))
	for _, r := range e.linkRows {
		rows = append(rows, &core.Row{Data: core.RowData{"doc": r.Doc, "component": r.Component, "how": r.How, "mentions": r.Mentions}})
	}
	return &core.View{Columns: columns("doc", "component", "how", "#mentions"), Rows: rows}
}

// columns declares a view's columns in order; a leading # makes one an
// integer, the rest are text.
func columns(names ...string) []*core.Column {
	out := make([]*core.Column, 0, len(names))
	for _, n := range names {
		if n[0] == '#' {
			out = append(out, core.IntColumn(n[1:]))
			continue
		}
		out = append(out, core.StringColumn(n))
	}
	return out
}
