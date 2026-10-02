package codesmells

import (
	"sort"

	"github.com/archstats/archstats/core"
	"github.com/archstats/archstats/extensions/treesitter/common"
)

// functionsView is every measured function, one row each: where it is and
// how hard it is to follow. It is how a reader gets from a file's
// complex-code deduction to the functions behind it. Vendored and generated
// code is left out, as it is from health: a minified bundle's one function
// topped hugo's list at a cognitive complexity of 4,200.
func functionsView(results *core.Results) *core.View {
	var rows []*core.Row
	for _, f := range ownFiles(results) {
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

// ownFiles are the files with measured functions that are the codebase's
// own, in order.
func ownFiles(results *core.Results) []string {
	files := make([]string, 0, len(results.FunctionsByFile))
	for f := range results.FunctionsByFile {
		if results.ThirdPartyFiles[f] || results.GeneratedFiles[f] {
			continue
		}
		files = append(files, f)
	}
	sort.Strings(files)
	return files
}

// complexityIncrementsView is what each complex function's cognitive
// complexity is made of: the construct on each line and what it cost. The
// source view draws them beside the code, so a reader sees which nested loop
// made a function score 276. Only functions over the threshold are kept; the
// rest have nothing to explain.
func complexityIncrementsView(results *core.Results) *core.View {
	var rows []*core.Row
	for _, f := range ownFiles(results) {
		for _, fn := range results.FunctionsByFile[f] {
			if fn.Cognitive <= common.ComplexThreshold {
				continue
			}
			for _, inc := range fn.Increments {
				rows = append(rows, &core.Row{Data: core.RowData{
					"file":           f,
					"function":       fn.Name,
					"function_begin": fn.Begin,
					"line":           inc.Line,
					"points":         inc.Points,
					"construct":      inc.Construct,
					"nesting":        inc.Nesting,
				}})
			}
		}
	}
	return &core.View{
		Columns: []*core.Column{
			core.StringColumn("file"),
			core.StringColumn("function"),
			core.IntColumn("function_begin"),
			core.IntColumn("line"),
			core.IntColumn("points"),
			core.StringColumn("construct"),
			core.IntColumn("nesting"),
		},
		Rows: rows,
	}
}
