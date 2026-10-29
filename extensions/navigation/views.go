package navigation

import (
	"github.com/archstats/archstats/core"
)

func (e *extension) entryPointsView(*core.Results) *core.View {
	rows := make([]*core.Row, 0, len(e.entryRows))
	for _, r := range e.entryRows {
		rows = append(rows, &core.Row{Data: core.RowData{
			"id": r.ID, "kind": r.Kind, "method": r.Method, "path": r.Path, "template": r.Template, "framework": r.Framework, "handler": r.Handler,
			"unit": r.Unit, "function": r.Function, "component": r.Component, "auth": r.Auth, "documented": r.Documented, "file": r.File, "line": r.Line,
		}})
	}
	return &core.View{Columns: columns("#id", "kind", "method", "path", "template", "framework", "handler", "unit", "function", "component", "auth", "documented", "file", "#line"), Rows: rows}
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

func (e *extension) operationsView(*core.Results) *core.View {
	rows := make([]*core.Row, 0, len(e.operationRows))
	for _, r := range e.operationRows {
		dep := 0
		if r.Deprecated {
			dep = 1
		}
		rows = append(rows, &core.Row{Data: core.RowData{
			"spec": r.Spec, "title": r.Title, "version": r.Version, "method": r.Method, "path": r.Path, "template": r.Template,
			"operation_id": r.OperationID, "tag": r.Tag, "summary": r.Summary, "deprecated": dep, "security": r.Security,
			"side": r.Side, "entry": r.Entry, "line": r.Line,
		}})
	}
	return &core.View{Columns: columns("spec", "title", "version", "method", "path", "template", "operation_id", "tag", "summary", "#deprecated", "security", "side", "#entry", "#line"), Rows: rows}
}

func (e *extension) findingsView(*core.Results) *core.View {
	rows := make([]*core.Row, 0, len(e.findingRows))
	for _, r := range e.findingRows {
		rows = append(rows, &core.Row{Data: core.RowData{
			"rule": r.Rule, "severity": r.Severity, "subject": r.Subject, "entry": r.Entry, "spec": r.Spec,
			"method": r.Method, "path": r.Path, "detail": r.Detail, "file": r.File, "line": r.Line,
		}})
	}
	return &core.View{Columns: columns("rule", "severity", "subject", "#entry", "spec", "method", "path", "detail", "file", "#line"), Rows: rows}
}

func (e *extension) outboundView(*core.Results) *core.View {
	rows := make([]*core.Row, 0, len(e.outboundRows))
	for _, r := range e.outboundRows {
		rows = append(rows, &core.Row{Data: core.RowData{
			"id": r.ID, "kind": r.Kind, "client": r.Client, "method": r.Method, "path": r.Path, "template": r.Template,
			"target": r.Target, "system": r.System, "host": r.Host, "confidence": r.Confidence, "resolved_via": r.ResolvedVia,
			"config_key": r.ConfigKey, "entry": r.Entry, "consumer": r.Consumer, "resilience": r.Resilience,
			"unit": r.Unit, "function": r.Function, "component": r.Component, "file": r.File, "line": r.Line,
		}})
	}
	return &core.View{Columns: columns("#id", "kind", "client", "method", "path", "template", "target", "system", "host", "confidence", "resolved_via", "config_key", "#entry", "#consumer", "resilience", "unit", "function", "component", "file", "#line"), Rows: rows}
}

func (e *extension) systemsView(*core.Results) *core.View {
	rows := make([]*core.Row, 0, len(e.systemRows))
	for _, r := range e.systemRows {
		rows = append(rows, &core.Row{Data: core.RowData{
			"system": r.System, "kind": r.Kind, "environment": r.Environment, "host": r.Host, "source": r.Source,
			"config_key": r.ConfigKey, "file": r.File, "line": r.Line,
		}})
	}
	return &core.View{Columns: columns("system", "kind", "environment", "host", "source", "config_key", "file", "#line"), Rows: rows}
}

func (e *extension) configView(*core.Results) *core.View {
	rows := make([]*core.Row, 0, len(e.configRows))
	for _, r := range e.configRows {
		rows = append(rows, &core.Row{Data: core.RowData{
			"key": r.Key, "value": r.Value, "environment": r.Environment, "format": r.Format, "file": r.File, "line": r.Line,
		}})
	}
	return &core.View{Columns: columns("key", "value", "environment", "format", "file", "#line"), Rows: rows}
}

func (e *extension) callsView(*core.Results) *core.View {
	rows := make([]*core.Row, 0, len(e.callRows))
	for _, r := range e.callRows {
		rows = append(rows, &core.Row{Data: core.RowData{
			"from_unit": r.FromUnit, "from_function": r.FromFunction, "to_unit": r.ToUnit, "to_function": r.ToFunction,
			"via": r.Via, "file": r.File, "line": r.Line,
		}})
	}
	return &core.View{Columns: columns("from_unit", "from_function", "to_unit", "to_function", "via", "file", "#line"), Rows: rows}
}

func (e *extension) reachView(*core.Results) *core.View {
	rows := make([]*core.Row, 0, len(e.reachRows))
	for _, r := range e.reachRows {
		rows = append(rows, &core.Row{Data: core.RowData{
			"entry": r.Entry, "call_sites": r.CallSites, "resolved": r.Resolved, "unknown": r.Unknown, "units": r.Units,
			"functions": r.Functions, "tables": r.Tables, "outbound": r.Outbound, "depth": r.Depth,
		}})
	}
	return &core.View{Columns: columns("#entry", "#call_sites", "#resolved", "#unknown", "#units", "#functions", "#tables", "#outbound", "#depth"), Rows: rows}
}

func (e *extension) targetsView(*core.Results) *core.View {
	rows := make([]*core.Row, 0, len(e.targetRows))
	for _, r := range e.targetRows {
		rows = append(rows, &core.Row{Data: core.RowData{"entry": r.Entry, "kind": r.Kind, "target": r.Target, "depth": r.Depth}})
	}
	return &core.View{Columns: columns("#entry", "kind", "target", "#depth"), Rows: rows}
}
