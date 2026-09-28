package swift

import (
	"sort"
	"strings"

	"github.com/archstats/archstats/core/file"
	"github.com/archstats/archstats/core/module"
	"github.com/archstats/archstats/core/unit"
)

// The linker runs once every file is parsed, and does what no single Swift
// file can: say which target it is in, and so which declarations its names
// mean.
//
//   - A file's target is the SwiftPM or Xcode target that owns it. A file no
//     manifest claims is filed under its top-level folder, which is what an
//     Xcode app with no project file in the checkout looks like.
//   - Unit IDs get their target: `Timeline#TimelineView`.
//   - A capitalised name resolves to the type of that name declared in the
//     file's own target, else in a target the file imports. A name found in
//     neither -- `View`, `String`, `URLSession` -- is the platform's and is
//     dropped.
//   - Where the declaring file is in another directory, that is an edge
//     between the two directories' components. This is where every Swift
//     component edge comes from.
//
// It runs after the component linker, which is registered before any
// language pack, so the edges it adds are not resolved a second time.
type linker struct {
	root string
}

type declared struct {
	id        string
	component string
}

func (l *linker) EditFileResults(all []*file.Results) {
	var swiftFiles []*file.Results
	var names []string
	for _, fr := range all {
		names = append(names, strings.TrimPrefix(fr.Name, "./"))
		if strings.HasSuffix(fr.Name, ".swift") {
			swiftFiles = append(swiftFiles, fr)
		}
	}
	if len(swiftFiles) == 0 {
		return
	}
	mods := module.ReadFrom(l.root, names)
	targetOf := map[*file.Results]string{}
	for _, fr := range swiftFiles {
		targetOf[fr] = target(mods, fr.Name)
	}

	// What each target declares, by name: nested types by their full name
	// (`Outer.Inner`) and by their own.
	index := map[string]map[string]declared{}
	for _, fr := range swiftFiles {
		t := targetOf[fr]
		if index[t] == nil {
			index[t] = map[string]declared{}
		}
		for _, u := range fr.Units {
			if u.Kind != unit.KindType || isExtension(u) {
				continue
			}
			full := strings.TrimPrefix(u.ID, placeholder+"#")
			d := declared{id: t + "#" + full, component: fr.Directory}
			// A nested type answers only to its full name. isowords declares
			// 27 types called State, one inside each reducer; by its own name
			// alone, every reducer's `State` would have meant the first.
			if _, taken := index[t][full]; !taken {
				index[t][full] = d
			}
		}
	}
	lookup := func(fr *file.Results, imports []string, name string) (declared, string, bool) {
		t := targetOf[fr]
		if d, ok := index[t][name]; ok {
			return d, t, true
		}
		for _, imp := range imports {
			if d, ok := index[imp][name]; ok {
				return d, imp, true
			}
		}
		return declared{}, "", false
	}

	for _, fr := range swiftFiles {
		t := targetOf[fr]
		var imports []string
		for _, s := range fr.Snippets {
			if s.Type == file.ImportRaw {
				imports = append(imports, s.Value)
			}
		}
		rename := func(id string) string {
			if strings.HasPrefix(id, placeholder+"#") {
				return t + "#" + strings.TrimPrefix(id, placeholder+"#")
			}
			return id
		}
		var units []*unit.Unit
		for _, u := range fr.Units {
			if isExtension(u) {
				// More of a type declared in this target: the same unit, so
				// it takes that type's ID and loses the extension keyword. A
				// type from elsewhere (View, String) is not this target's to
				// declare; its extension adds members, which keep it as
				// their owner.
				full := strings.TrimPrefix(u.ID, placeholder+"#")
				if _, ok := index[t][full]; !ok {
					continue
				}
				var markers []unit.Marker
				for _, m := range u.Markers {
					if !(m.Source == sourceKeyword && m.Key == "extension") {
						markers = append(markers, m)
					}
				}
				u.Markers = markers
			}
			u.ID = rename(u.ID)
			u.Owner = rename(u.Owner)
			var refs []unit.Ref
			for _, r := range u.Refs {
				if r.Module != placeholder {
					refs = append(refs, r)
					continue
				}
				if d, in, ok := lookup(fr, imports, r.Name); ok {
					refs = append(refs, unit.Ref{Module: in, Name: strings.TrimPrefix(d.id, in+"#"), Exact: true})
				}
			}
			u.Refs = refs
			units = append(units, u)
		}
		fr.Units = units

		// Component edges, one per directory this file reaches, at the first
		// mention.
		reached := map[string]bool{}
		kept := fr.Snippets[:0]
		var edges []*file.Snippet
		for _, s := range fr.Snippets {
			if s.Type != captureRef {
				kept = append(kept, s)
				continue
			}
			d, _, ok := lookup(fr, imports, s.Value)
			if !ok || d.component == fr.Directory || d.component == "" || reached[d.component] {
				continue
			}
			reached[d.component] = true
			edges = append(edges, &file.Snippet{
				File: s.File, Directory: fr.Directory, Component: s.Component,
				Type: file.ComponentImport, Value: d.component, Begin: s.Begin, End: s.End,
			})
		}
		fr.Snippets = append(kept, edges...)
		fr.Stats = append(fr.Stats, file.SnippetsToStats(edges)...)
	}
}

func isExtension(u *unit.Unit) bool {
	for _, m := range u.Markers {
		if m.Source == sourceKeyword && m.Key == "extension" {
			return true
		}
	}
	return false
}

// target names the module a Swift file is compiled into: the SwiftPM or
// Xcode target that owns it, else whatever module owns it, else its
// top-level folder.
func target(mods *module.Map, name string) string {
	rel := strings.TrimPrefix(name, "./")
	if m := mods.Of(rel); m != nil && m.Name != "" {
		return m.Name
	}
	if i := strings.Index(rel, "/"); i > 0 {
		return rel[:i]
	}
	return "."
}

// sortedKeys is for tests and diagnostics.
func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
