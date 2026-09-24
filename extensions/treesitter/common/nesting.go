package common

import (
	"sort"

	"github.com/archstats/archstats/core/file"
)

// NestedNames gives each declared type its name within the file: "Town" for
// a top-level type, "UuidTableEntityTest.Town" for one declared inside
// another.
//
// A nested type named only by its package collides with every other type of
// that name in the package. Exposed declares a `Town` inside six test
// classes of one package, and the six were folded into one unit that
// "depended" on everything any of them used. The enclosing types are part of
// the name, as they are to the compiler.
//
// names are the declared names; spans the whole declarations of the types
// that can enclose others. A span belongs to the first name inside it, which
// is the name of the type it declares.
func NestedNames(names, spans []*file.Snippet) map[*file.Snippet]string {
	sorted := append([]*file.Snippet{}, names...)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].Begin.Offset < sorted[j].Begin.Offset })

	type owned struct {
		begin, end int
		owner      *file.Snippet
	}
	var declared []owned
	for _, sp := range spans {
		for _, n := range sorted {
			if n.Begin.Offset >= sp.Begin.Offset && n.Begin.Offset <= sp.End.Offset {
				declared = append(declared, owned{sp.Begin.Offset, sp.End.Offset, n})
				break
			}
		}
	}
	// Outermost first, so the chain reads from the outside in.
	sort.SliceStable(declared, func(i, j int) bool {
		return declared[i].end-declared[i].begin > declared[j].end-declared[j].begin
	})

	out := make(map[*file.Snippet]string, len(names))
	for _, n := range names {
		name := ""
		for _, d := range declared {
			if d.owner == n || n.Begin.Offset < d.begin || n.Begin.Offset > d.end {
				continue
			}
			name += d.owner.Value + "."
		}
		out[n] = name + n.Value
	}
	return out
}
