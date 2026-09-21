package csharp

import (
	"sort"

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

	var out []*unit.Unit
	for i, t := range types {
		id := t.Value
		if namespace != "" {
			id = namespace + "." + t.Value
		}
		out = append(out, &unit.Unit{
			ID:      id,
			Kind:    unit.KindType,
			Name:    t.Value,
			Files:   []string{path},
			Markers: markers[i],
		})
	}
	return out
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
