package components

import (
	"github.com/archstats/archstats/core"
	"github.com/archstats/archstats/core/file"
)

// The dependencies archstats could see but could not place.
//
// A codebase whose edges are resolved from strings at runtime does not
// announce itself. django-oscar makes 746 get_class and get_model calls
// against 1,645 static imports -- roughly 31% of its dependency graph -- and
// that is not sloppiness: it is how every one of its apps is made
// overridable. Drawn from static imports alone, a third of that codebase is
// missing from every coupling number archstats prints, and nothing says so.
//
// Most of those calls name a module as a plain string, and the component
// linker resolves them like any other import. This view is for the rest: the
// ones that named something this analysis never saw, or named it with a
// variable. Reporting them is the point. An architect can weigh "31% of these
// edges are dynamic and four of them go nowhere I can find" -- they cannot
// weigh a confident graph that quietly omits them.
func UnresolvedEdgesView(results *core.Results) *core.View {
	var rows []*core.Row
	for _, snippet := range results.SnippetsByType[file.ComponentImportDynamic] {
		// The linker rewrote the snippet's value to the component it names.
		// If no component by that name exists, nothing in this codebase
		// answers to it.
		if _, found := results.SnippetsByComponent[snippet.Value]; found {
			continue
		}
		reason := "names a module this analysis did not see"
		if snippet.Value == "" {
			reason = "named by an expression rather than a string"
		}
		rows = append(rows, &core.Row{
			Data: map[string]interface{}{
				"from":   snippet.Component,
				"names":  snippet.Value,
				"file":   snippet.File,
				"line":   snippet.Begin.Line,
				"reason": reason,
			},
		})
	}
	return &core.View{
		Name: "unresolved_edges",
		Columns: []*core.Column{
			core.StringColumn("from"),
			core.StringColumn("names"),
			core.StringColumn("file"),
			core.IntColumn("line"),
			core.StringColumn("reason"),
		},
		Rows: rows,
	}
}
