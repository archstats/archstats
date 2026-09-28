package apple

import (
	"strings"

	"github.com/archstats/archstats/core/file"
	"github.com/archstats/archstats/core/module"
	"github.com/archstats/archstats/core/unit"
)

// Linker runs once every file is parsed, and does what no single Swift or
// Objective-C file can: say which target it is in, and so which declarations
// its names mean. Both languages share a target's namespace -- Swift sees the
// Objective-C a bridging header exposes, Objective-C the Swift its -Swift.h
// generates -- so they are resolved together.
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
// language pack, so the edges it adds are not resolved a second time. Both
// packs register it; the second run finds nothing left to do.
type Linker struct {
	Root string
}

type declared struct {
	id        string
	component string
}

func (l *Linker) EditFileResults(all []*file.Results) {
	var sources []*file.Results
	var names []string
	for _, fr := range all {
		names = append(names, strings.TrimPrefix(fr.Name, "./"))
		if IsAppleSource(fr.Name) && hasPlaceholder(fr) {
			sources = append(sources, fr)
		}
	}
	if len(sources) == 0 {
		return
	}
	mods := module.ReadFrom(l.Root, names)
	targetOf := map[*file.Results]string{}
	for _, fr := range sources {
		targetOf[fr] = target(mods, fr.Name)
	}

	// What each target declares, by name: nested types by their full name
	// (`Outer.Inner`) and by their own.
	index := map[string]map[string]declared{}
	for _, fr := range sources {
		t := targetOf[fr]
		if index[t] == nil {
			index[t] = map[string]declared{}
		}
		for _, u := range fr.Units {
			if u.Kind != unit.KindType || IsExtension(u) {
				continue
			}
			full := strings.TrimPrefix(u.ID, Placeholder+"#")
			d := declared{id: t + "#" + full, component: fr.Directory}
			// A nested type answers only to its full name. isowords declares
			// 27 types called State, one inside each reducer; by its own name
			// alone, every reducer's `State` would have meant the first.
			if _, taken := index[t][full]; !taken {
				index[t][full] = d
			}
		}
	}
	// Headers by file name, for `#import "X.h"`: Xcode searches a target's
	// headers by name, whatever folder they are in.
	headers := map[string][]*file.Results{}
	for _, fr := range all {
		if strings.HasSuffix(fr.Name, ".h") {
			base := fr.Name[strings.LastIndex(fr.Name, "/")+1:]
			headers[base] = append(headers[base], fr)
		}
	}
	header := func(from *file.Results, written string) string {
		base := written[strings.LastIndex(written, "/")+1:]
		var sameTarget, suffix []*file.Results
		for _, h := range headers[base] {
			if strings.Contains(written, "/") && !strings.HasSuffix(h.Name, "/"+written) {
				continue
			}
			suffix = append(suffix, h)
			if targetOf[h] == targetOf[from] {
				sameTarget = append(sameTarget, h)
			}
		}
		switch {
		case len(sameTarget) == 1:
			return sameTarget[0].Directory
		case len(sameTarget) == 0 && len(suffix) == 1:
			return suffix[0].Directory
		}
		// Two headers of that name and nothing to choose between them: no
		// edge, rather than a guessed one.
		return ""
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

	for _, fr := range sources {
		t := targetOf[fr]
		var imports []string
		for _, s := range fr.Snippets {
			if s.Type == file.ImportRaw {
				imports = append(imports, s.Value)
			}
		}
		rename := func(id string) string {
			if strings.HasPrefix(id, Placeholder+"#") {
				return t + "#" + strings.TrimPrefix(id, Placeholder+"#")
			}
			return id
		}
		var units []*unit.Unit
		for _, u := range fr.Units {
			if IsExtension(u) {
				// More of a type declared in this target: the same unit, so
				// it takes that type's ID and loses the extension keyword. A
				// type from elsewhere (View, String) is not this target's to
				// declare; its extension adds members, which keep it as
				// their owner.
				full := strings.TrimPrefix(u.ID, Placeholder+"#")
				if _, ok := index[t][full]; !ok {
					continue
				}
				var markers []unit.Marker
				for _, m := range u.Markers {
					if !(m.Source == SourceKeyword && m.Key == "extension") {
						markers = append(markers, m)
					}
				}
				u.Markers = markers
			}
			u.ID = rename(u.ID)
			u.Owner = rename(u.Owner)
			var refs []unit.Ref
			for _, r := range u.Refs {
				if r.Module != Placeholder {
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
			var to string
			switch s.Type {
			case CaptureRef:
				if d, _, ok := lookup(fr, imports, s.Value); ok {
					to = d.component
				}
			case CaptureInclude:
				to = header(fr, s.Value)
			default:
				kept = append(kept, s)
				continue
			}
			if to == fr.Directory || to == "" || reached[to] {
				continue
			}
			reached[to] = true
			d := declared{component: to}
			edges = append(edges, &file.Snippet{
				File: s.File, Directory: fr.Directory, Component: s.Component,
				Type: file.ComponentImport, Value: d.component, Begin: s.Begin, End: s.End,
			})
		}
		fr.Snippets = append(kept, edges...)
		fr.Stats = append(fr.Stats, file.SnippetsToStats(edges)...)
	}
}

func IsExtension(u *unit.Unit) bool {
	for _, m := range u.Markers {
		if m.Source == SourceKeyword && m.Key == "extension" {
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

// Placeholder stands for the target in unit IDs and references until the
// linker knows it.
const Placeholder = "\x00apple"

// CaptureRef is every capitalised name a file mentions, kept on the file's
// results for the linker, which turns them into component edges and
// removes them.
const CaptureRef = "apple__ref"

// CaptureInclude is an Objective-C `#import "X.h"`, resolved by the linker to
// the directory holding that header.
const CaptureInclude = "apple__include"

// SourceKeyword marks what a type was declared as: class, struct, enum,
// actor, protocol, or extension for a type a file only extends.
const SourceKeyword = "keyword"

func IsAppleSource(name string) bool {
	for _, ext := range []string{".swift", ".m", ".mm", ".h"} {
		if strings.HasSuffix(name, ext) {
			return true
		}
	}
	return false
}

func hasPlaceholder(fr *file.Results) bool {
	for _, u := range fr.Units {
		if strings.HasPrefix(u.ID, Placeholder) {
			return true
		}
	}
	for _, s := range fr.Snippets {
		if s.Type == CaptureRef || s.Type == CaptureInclude {
			return true
		}
	}
	return false
}
