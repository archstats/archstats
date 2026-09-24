package basic

import (
	"sort"

	"github.com/archstats/archstats/core"
)

// Every module a unit uses something from, one row each, whether or not the
// module is part of this codebase.
//
// A file's imports are the file's, not each declaration's. gin's
// context_test.go imports a MongoDB package for one BSON test, and every one
// of the file's hundred-odd test functions was read as reaching a database.
// This is the same question asked of each unit: which of the modules its
// file imports does it actually use.
//
// A method's uses are recorded on the method; a reader wanting a type's uses
// gathers its members'.
func unitUsesView(results *core.Results) *core.View {
	var rows []*core.Row
	for _, u := range results.Units {
		seen := map[string]bool{}
		var modules []string
		for _, r := range u.Refs {
			if r.Module == "" || seen[r.Module] {
				continue
			}
			seen[r.Module] = true
			modules = append(modules, r.Module)
		}
		sort.Strings(modules)
		for _, m := range modules {
			rows = append(rows, &core.Row{Data: core.RowData{"unit": u.ID, "module": m}})
		}
	}
	return &core.View{
		Columns: []*core.Column{core.StringColumn("unit"), core.StringColumn("module")},
		Rows:    rows,
	}
}
