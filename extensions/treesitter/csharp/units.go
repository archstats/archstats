package csharp

import (
	"sort"
	"strings"

	"github.com/archstats/archstats/core/file"
	"github.com/archstats/archstats/core/unit"
	"github.com/archstats/archstats/extensions/treesitter/common"
)

// csharpAnalyzer turns what the pack captured into units.
//
// C# is where the file stops being a fair stand-in for the type, in both
// directions at once. nopCommerce declares 1,567 partial classes -- one type
// spread across several files -- and 141 of its files hold more than one
// top-level type. A per-file record of "the type in this file" cannot
// represent either, and last-record-wins silently discards the rest.
type csharpAnalyzer struct {
	lp *common.LanguagePack
}

func (a *csharpAnalyzer) AnalyzeFile(f file.File) *file.Results {
	res := a.lp.AnalyzeFile(f)
	if res == nil {
		return nil
	}
	res.Units = unitsFrom(f.Path(), res)
	// The type uses and declaration spans exist to build references; every
	// `x.Name` is one, and storing them all would bury the snippets table.
	kept := res.Snippets[:0]
	for _, sn := range res.Snippets {
		if sn.Type == "csharp__type__use" || sn.Type == "csharp__declaration__span" {
			continue
		}
		kept = append(kept, sn)
	}
	res.Snippets = kept
	return res
}

// An attribute sits above the thing it is about, so it belongs to the first
// type declared at or after it. In a file with one type this is the same as
// attaching everything to that type; in a file with four it is the only way
// to keep `[Serializable]` off the three types that do not carry it.
func unitsFrom(path string, res *file.Results) []*unit.Unit {
	namespace := ""
	var types []*file.Snippet
	var attributes []*file.Snippet
	var bases []*file.Snippet
	var uses, spans, usings []*file.Snippet

	for _, s := range res.Snippets {
		switch s.Type {
		case file.ComponentDeclaration:
			if namespace == "" {
				namespace = s.Value
			}
		case file.Type:
			types = append(types, s)
		case "csharp__class__attribute":
			attributes = append(attributes, s)
		case "csharp__class__base":
			bases = append(bases, s)
		case "csharp__type__use":
			uses = append(uses, s)
		case "csharp__declaration__span":
			spans = append(spans, s)
		case file.ImportRaw:
			usings = append(usings, s)
		}
	}
	if len(types) == 0 {
		return nil
	}
	sort.SliceStable(types, func(i, j int) bool { return types[i].Begin.Offset < types[j].Begin.Offset })

	markers := make(map[int][]unit.Marker)
	attach := func(snippets []*file.Snippet, source string) {
		for _, s := range snippets {
			if idx := ownerOf(s, types, source); idx >= 0 {
				markers[idx] = append(markers[idx], unit.Marker{Source: source, Key: s.Value})
			}
		}
	}
	attach(attributes, unit.SourceAnnotation)
	attach(bases, unit.SourceSupertype)

	// A type inside another is named through it, as the compiler names it:
	// nopCommerce's ConvertFrom and ConvertTo are declared inside several
	// classes of one namespace, and named by namespace alone were folded
	// into one unit each.
	nested := common.NestedNames(types, spans)

	var out []*unit.Unit
	for i, t := range types {
		id := nested[t]
		if namespace != "" {
			id = namespace + "." + id
		}
		// A nested type is a member of the one around it, as a method is:
		// StandardPermission's eleven permission groups are one thing it
		// declares, not twelve.
		owner := ""
		if i := strings.LastIndex(id, "."); i > 0 && strings.Contains(nested[t], ".") {
			owner = id[:i]
		}
		out = append(out, &unit.Unit{
			ID:      id,
			Kind:    unit.KindType,
			Name:    t.Value,
			Files:   []string{path},
			Owner:   owner,
			Markers: markers[i],
		})
	}
	attachUses(out, types, spans, uses, usings, namespace)
	return out
}

// Which type each unit named, and where it might live.
//
// C# imports a namespace rather than a type: `using Acme.Core` says nothing
// about which of its types are used. So a usage carries only a bare name,
// and the namespaces it could belong to are the file's usings plus its own.
// Every pairing is offered and resolution keeps the ones that exist, which
// is why the references are generated broadly and filtered late rather than
// guessed at narrowly here.
func attachUses(units []*unit.Unit, types, spans, uses, usings []*file.Snippet, namespace string) {
	if len(uses) == 0 || len(units) == 0 {
		return
	}
	namespaces := []string{namespace}
	for _, u := range usings {
		namespaces = append(namespaces, u.Value)
	}

	// Which unit owns each declaration span: the type declared inside it.
	type span struct{ begin, end, unitIdx int }
	var owned []span
	for _, sp := range spans {
		for i, t := range types {
			if t.Begin.Offset < sp.Begin.Offset || t.Begin.Offset > sp.End.Offset {
				continue
			}
			if i < len(units) {
				owned = append(owned, span{sp.Begin.Offset, sp.End.Offset, i})
			}
			break
		}
	}
	sort.SliceStable(owned, func(i, j int) bool {
		return (owned[i].end - owned[i].begin) < (owned[j].end - owned[j].begin)
	})

	for _, use := range uses {
		for _, sp := range owned {
			if use.Begin.Offset < sp.begin || use.Begin.Offset > sp.end {
				continue
			}
			u := units[sp.unitIdx]
			if u.Name == use.Value {
				break // a type naming itself is not a dependency
			}
			for _, ns := range namespaces {
				if ns == "" {
					continue
				}
				u.Refs = appendRef(u.Refs, unit.Ref{Module: ns, Name: use.Value})
			}
			break
		}
	}
}

func appendRef(list []unit.Ref, r unit.Ref) []unit.Ref {
	for _, existing := range list {
		if existing == r {
			return list
		}
	}
	return append(list, r)
}

// ownerOf finds which declaration a marker belongs to. An attribute precedes
// its type; a base type follows the name it is attached to.
func ownerOf(s *file.Snippet, types []*file.Snippet, source string) int {
	if source == unit.SourceSupertype {
		last := -1
		for i, t := range types {
			if t.Begin.Offset <= s.Begin.Offset {
				last = i
			}
		}
		return last
	}
	for i, t := range types {
		if t.Begin.Offset >= s.Begin.Offset {
			return i
		}
	}
	return -1
}
