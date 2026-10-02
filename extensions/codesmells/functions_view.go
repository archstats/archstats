package codesmells

import (
	"sort"

	"github.com/archstats/archstats/core"
)

// functionsView is every measured function, one row each: where it is and
// how hard it is to follow. It is how a reader gets from a file's
// complex-code deduction to the functions behind it. Vendored and generated
// code is left out, as it is from health: a minified bundle's one function
// topped hugo's list at a cognitive complexity of 4,200.
func functionsView(results *core.Results) *core.View {
	files := make([]string, 0, len(results.FunctionsByFile))
	for f := range results.FunctionsByFile {
		if results.ThirdPartyFiles[f] || results.GeneratedFiles[f] {
			continue
		}
		files = append(files, f)
	}
	sort.Strings(files)
	var rows []*core.Row
	for _, f := range files {
		for _, fn := range results.FunctionsByFile[f] {
			rows = append(rows, &core.Row{Data: core.RowData{
				"file":       f,
				"component":  results.FileToComponent[f],
				"name":       fn.Name,
				"begin_line": fn.Begin,
				"end_line":   fn.End,
				"lines":      fn.Lines(),
				"cognitive":  fn.Cognitive,
				"nesting":    fn.Nesting,
				"params":     fn.Params,
			}})
		}
	}
	return &core.View{
		Columns: []*core.Column{
			core.StringColumn("file"),
			core.StringColumn("component"),
			core.StringColumn("name"),
			core.IntColumn("begin_line"),
			core.IntColumn("end_line"),
			core.IntColumn("lines"),
			core.IntColumn("cognitive"),
			core.IntColumn("nesting"),
			core.IntColumn("params"),
		},
		Rows: rows,
	}
}
