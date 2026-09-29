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

const (
	captureAttribute = "csharp__class__attribute"
	captureBase      = "csharp__class__base"
	captureUse       = "csharp__type__use"
	captureSpan      = "csharp__declaration__span"
	// What a type was declared as, where the keyword is the evidence: a
	// record is a data shape by construction, and an interface is a
	// contract. Helper captures, folded into markers and not stored.
	captureRecord    = "csharp__record__name"
	captureStruct    = "csharp__struct__name"
	captureEnum      = "csharp__enum__name"
	captureInterface = "csharp__interface__name"
)

// The marker source for what a type was declared as, shared with the Swift
// and Kotlin packs.
const sourceKeyword = "keyword"

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
		switch sn.Type {
		case captureUse, captureSpan, captureRecord, captureStruct, captureEnum, captureInterface:
			continue
		}
		kept = append(kept, sn)
	}
	res.Snippets = kept
	return res
}

// An attribute belongs to the declaration it sits in: `[Table]` above a
// class is inside that class's node, and so is `[Key]` on one of its
// properties or `[HttpGet]` on one of its methods. It used to belong to the
// first type declared after it, which for a method attribute is the next
// class in the file, and in a file with one class is nothing at all -- so
// `[HttpPost]` never marked a controller and `[Key]` never marked an entity.
func unitsFrom(path string, res *file.Results) []*unit.Unit {
	namespace := ""
	var types []*file.Snippet
	var attributes []*file.Snippet
	var bases []*file.Snippet
	var uses, spans, usings []*file.Snippet
	keyword := map[int]string{}

	for _, s := range res.Snippets {
		switch s.Type {
		case file.ComponentDeclaration:
			if namespace == "" {
				namespace = s.Value
			}
		case file.Type:
			types = append(types, s)
		case captureAttribute:
			attributes = append(attributes, s)
		case captureBase:
			bases = append(bases, s)
		case captureUse:
			uses = append(uses, s)
		case captureSpan:
			spans = append(spans, s)
		case file.ImportRaw:
			usings = append(usings, s)
		case captureRecord:
			keyword[s.Begin.Offset] = "record"
		case captureStruct:
			keyword[s.Begin.Offset] = "struct"
		case captureEnum:
			keyword[s.Begin.Offset] = "enum"
		case captureInterface:
			keyword[s.Begin.Offset] = "interface"
		}
	}
	sort.SliceStable(types, func(i, j int) bool { return types[i].Begin.Offset < types[j].Begin.Offset })

	// Which type each declaration span declares: the first type named inside
	// it. Innermost first, so a nested type is preferred over the one around
	// it.
	type span struct{ begin, end, typeIdx int }
	var owned []span
	for _, sp := range spans {
		for i, t := range types {
			if t.Begin.Offset < sp.Begin.Offset || t.Begin.Offset > sp.End.Offset {
				continue
			}
			owned = append(owned, span{sp.Begin.Offset, sp.End.Offset, i})
			break
		}
	}
	sort.SliceStable(owned, func(i, j int) bool {
		return (owned[i].end - owned[i].begin) < (owned[j].end - owned[j].begin)
	})
	ownerAt := func(offset int) int {
		for _, sp := range owned {
			if offset >= sp.begin && offset <= sp.end {
				return sp.typeIdx
			}
		}
		return -1
	}

	markers := make(map[int][]unit.Marker)
	for _, s := range attributes {
		idx := ownerAt(s.Begin.Offset)
		if idx < 0 {
			idx = ownerOf(s, types, unit.SourceAnnotation)
		}
		if idx >= 0 {
			markers[idx] = appendMarker(markers[idx], unit.Marker{Source: unit.SourceAnnotation, Key: attributeName(s.Value)})
		}
	}
	for _, s := range bases {
		if idx := ownerOf(s, types, unit.SourceSupertype); idx >= 0 {
			markers[idx] = appendMarker(markers[idx], unit.Marker{Source: unit.SourceSupertype, Key: s.Value})
		}
	}
	for i, t := range types {
		switch kw := keyword[t.Begin.Offset]; kw {
		case "":
		case "interface":
			markers[i] = appendMarker(markers[i], unit.Marker{Source: unit.SourceSupertype, Key: kw})
		default:
			markers[i] = appendMarker(markers[i], unit.Marker{Source: sourceKeyword, Key: kw})
		}
	}

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

	// Which type each unit named, and where it might live.
	//
	// C# imports a namespace rather than a type: `using Acme.Core` says
	// nothing about which of its types are used. So a usage carries only a
	// bare name, and the namespaces it could belong to are the file's usings
	// plus its own. Every pairing is offered and resolution keeps the ones
	// that exist, which is why the references are generated broadly and
	// filtered late rather than guessed at narrowly here.
	namespaces := []string{namespace}
	for _, u := range usings {
		namespaces = append(namespaces, u.Value)
	}
	// A use outside every type is the file's own: the top-level statements
	// of a Program.cs, where a minimal API maps its routes and registers its
	// services. That file declares no type, and used to produce nothing.
	var moduleRefs []unit.Ref
	for _, use := range uses {
		idx := ownerAt(use.Begin.Offset)
		if idx >= 0 && out[idx].Name == use.Value {
			continue // a type naming itself is not a dependency
		}
		for _, ns := range namespaces {
			if ns == "" {
				continue
			}
			r := unit.Ref{Module: ns, Name: use.Value}
			if idx >= 0 {
				out[idx].Refs = appendRef(out[idx].Refs, r)
			} else {
				moduleRefs = appendRef(moduleRefs, r)
			}
		}
	}
	if mu := common.ModuleUnit(strings.TrimSuffix(path, ".cs"), path, moduleRefs); mu != nil {
		out = append(out, mu)
	}
	return out
}

// attributeName is the attribute as the compiler reads it: `[ApiController]`
// and `[ApiControllerAttribute]` name the same class.
func attributeName(written string) string {
	const suffix = "Attribute"
	if strings.HasSuffix(written, suffix) && len(written) > len(suffix) {
		return strings.TrimSuffix(written, suffix)
	}
	return written
}

func appendMarker(list []unit.Marker, m unit.Marker) []unit.Marker {
	for _, existing := range list {
		if existing == m {
			return list
		}
	}
	return append(list, m)
}

func appendRef(list []unit.Ref, r unit.Ref) []unit.Ref {
	for _, existing := range list {
		if existing == r {
			return list
		}
	}
	return append(list, r)
}

// ownerOf finds which declaration a marker belongs to when no declaration
// span holds it. An attribute precedes its type; a base type follows the
// name it is attached to.
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
