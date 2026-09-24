package golang

import (
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/archstats/archstats/core/file"
	"github.com/archstats/archstats/core/unit"
)

// Turning what the pack captured into units.
//
// Go is the case the unit grain was designed for. A method is written outside
// the type it belongs to, so where it lives and what it belongs to are
// different questions; a package holds many types and many more functions, so
// one file is neither one unit nor one type; and there is nothing to read as
// an annotation, so the markers come from struct tags and comment directives
// instead.

// Where a unit lives, as the part of its id before the name. Go identifies a
// type by its package, and a package is its directory -- `package core` says
// only the last element, which is why components come from the tree.
func packageOf(path string) string {
	dir := filepath.ToSlash(filepath.Dir(path))
	if dir == "." || dir == "/" {
		return ""
	}
	return dir
}

func id(pkg, name string) string {
	if pkg == "" {
		return name
	}
	return pkg + "." + name
}

// A tag is a back-quoted run of `key:"value"` pairs. Each key is a separate
// claim about the field -- `json:"name" form:"n"` says two things -- so each
// becomes its own marker.
var tagPair = regexp.MustCompile(`(\w+):"([^"]*)"`)

// A directive is `//go:embed`, `//go:generate`, `//go:build`. The word after
// the colon is the claim; the rest is its argument.
var directive = regexp.MustCompile(`^//go:(\w+)\s*(.*)$`)

func unitsFrom(path string, res *file.Results) []*unit.Unit {
	pkg := packageOf(path)

	var types, interfaces, funcs, methods, receivers, embeds, tags, directives []*file.Snippet
	var aliases, selPkgs, selNames, declSpans, importPaths, locals []*file.Snippet
	for _, s := range res.Snippets {
		switch s.Type {
		case captureTypeName:
			types = append(types, s)
		case captureInterface:
			interfaces = append(interfaces, s)
		case captureFunc:
			funcs = append(funcs, s)
		case captureMethod:
			methods = append(methods, s)
		case captureReceiver:
			receivers = append(receivers, s)
		case captureEmbeds:
			embeds = append(embeds, s)
		case captureTag:
			tags = append(tags, s)
		case captureDirective:
			directives = append(directives, s)
		case captureAlias:
			aliases = append(aliases, s)
		case captureSelPkg:
			selPkgs = append(selPkgs, s)
		case captureSelName:
			selNames = append(selNames, s)
		case captureDeclSpan:
			declSpans = append(declSpans, s)
		case file.ImportRaw:
			importPaths = append(importPaths, s)
		case captureLocalRef:
			locals = append(locals, s)
		}
	}
	byOffset(types)
	byOffset(methods)
	byOffset(receivers)

	isInterface := map[string]bool{}
	for _, s := range interfaces {
		isInterface[s.Value] = true
	}

	var out []*unit.Unit
	// The snippet that declares each unit, index for index with out. Spans
	// are attributed by this position: matching by name gave every method
	// called Render the first Render's span, and the others lost their
	// references.
	var declaredBy []*file.Snippet
	byName := map[string]*unit.Unit{}

	for i, t := range types {
		u := &unit.Unit{
			ID:    id(pkg, t.Value),
			Kind:  unit.KindType,
			Name:  t.Value,
			Files: []string{path},
		}
		if isInterface[t.Value] {
			u.Markers = append(u.Markers, unit.Marker{Source: unit.SourceSupertype, Key: "interface"})
		}
		// Everything a struct embeds, which is the nearest thing Go has to
		// extends. Attributed by position: an embedded field sits inside the
		// type declared above it.
		for _, e := range embeds {
			if ownerIndex(e, types) == i {
				u.Markers = append(u.Markers, unit.Marker{Source: unit.SourceSupertype, Key: e.Value})
			}
		}
		for _, tag := range tags {
			if ownerIndex(tag, types) != i {
				continue
			}
			for _, m := range tagPair.FindAllStringSubmatch(tag.Value, -1) {
				u.Markers = append(u.Markers, unit.Marker{Source: unit.SourceStructTag, Key: m[1], Value: m[2]})
			}
		}
		out = append(out, u)
		declaredBy = append(declaredBy, t)
		byName[t.Value] = u
	}

	// A method belongs to its receiver and is written outside it, so the two
	// captures come from one match and are paired back up by position: the
	// receiver is the one immediately before the name.
	for _, m := range methods {
		owner := nearestBefore(m, receivers)
		ownerID := ""
		name := m.Value
		if owner != "" {
			ownerID = id(pkg, owner)
			name = owner + "." + m.Value
		}
		out = append(out, &unit.Unit{
			ID:    id(pkg, name),
			Kind:  unit.KindFunction,
			Name:  m.Value,
			Files: []string{path},
			Owner: ownerID,
		})
		declaredBy = append(declaredBy, m)
	}

	for _, f := range funcs {
		unitID := id(pkg, f.Value)
		// A package may declare init (and _) once per file. Sharing one id
		// folded them into a single unit whose references came from every
		// file's init at once.
		if f.Value == "init" || f.Value == "_" {
			unitID = id(pkg, f.Value) + "@" + filepath.Base(path)
		}
		out = append(out, &unit.Unit{
			ID:    unitID,
			Kind:  unit.KindFunction,
			Name:  f.Value,
			Files: []string{path},
		})
		declaredBy = append(declaredBy, f)
	}

	// A directive is about the file rather than any one declaration, so it
	// marks whatever the file declares. Nothing to attach it to means
	// nothing to record.
	for _, d := range directives {
		m := directive.FindStringSubmatch(strings.TrimSpace(d.Value))
		if m == nil {
			continue
		}
		for _, u := range out {
			u.Markers = append(u.Markers, unit.Marker{Source: unit.SourceDirective, Key: m[1], Value: m[2]})
		}
	}

	attachGoRefs(out, declaredBy, pkg, aliases, importPaths, selPkgs, selNames, declSpans, locals)
	return out
}

// Which unit used which package, and what it took from it.
//
// Go imports a package rather than a name, and writes the use as
// `pkg.Symbol`, so the edge only exists once the alias in a selector is
// paired with the import path it stands for. An import with no alias is
// known by the last element of its path, which is what the code writes.
//
// No textual scan is needed here, unlike the languages where an import
// clause names what it took: the selector says the package and the symbol
// together, and its position says which declaration used it.
func attachGoRefs(units []*unit.Unit, declaredBy []*file.Snippet, pkg string, aliases, importPaths, selPkgs, selNames, declSpans, locals []*file.Snippet) {
	if len(units) == 0 {
		return
	}
	byOffset(importPaths)
	byOffset(aliases)
	byOffset(selPkgs)
	byOffset(selNames)

	// alias -> import path. An explicit alias sits immediately before the
	// path it renames.
	pathFor := map[string]string{}
	for _, p := range importPaths {
		last := p.Value
		if i := strings.LastIndex(last, "/"); i != -1 {
			last = last[i+1:]
		}
		if last != "" {
			pathFor[last] = p.Value
		}
	}
	for _, a := range aliases {
		for _, p := range importPaths {
			if p.Begin.Offset > a.Begin.Offset {
				pathFor[a.Value] = p.Value
				break
			}
		}
	}

	// Which unit owns each declaration span: the one whose own declaring
	// name sits inside it.
	type span struct{ begin, end, unitIdx int }
	var owned []span
	for _, sp := range declSpans {
		best, bestOffset := -1, 0
		for i, d := range declaredBy {
			off := d.Begin.Offset
			if off < sp.Begin.Offset || off > sp.End.Offset {
				continue
			}
			if best == -1 || off < bestOffset {
				best, bestOffset = i, off
			}
		}
		if best >= 0 {
			owned = append(owned, span{sp.Begin.Offset, sp.End.Offset, best})
		}
	}
	sort.SliceStable(owned, func(i, j int) bool {
		return (owned[i].end - owned[i].begin) < (owned[j].end - owned[j].begin)
	})

	for i, sel := range selPkgs {
		path, known := pathFor[sel.Value]
		if !known || i >= len(selNames) {
			continue
		}
		name := selNames[i]
		for _, sp := range owned {
			if sel.Begin.Offset >= sp.begin && sel.Begin.Offset <= sp.end {
				u := units[sp.unitIdx]
				u.Refs = appendGoRef(u.Refs, unit.Ref{Module: path, Name: name.Value})
				break
			}
		}
	}

	// Names used unqualified are this package's own declarations -- every
	// use of Context inside gin's root package, which is most of gin. They
	// resolve exactly in this package or not at all: locals, parameters and
	// builtins name nothing declared at package level and are dropped.
	for _, l := range locals {
		for _, sp := range owned {
			if l.Begin.Offset >= sp.begin && l.Begin.Offset <= sp.end {
				u := units[sp.unitIdx]
				if l.Value != u.Name {
					u.Refs = appendGoRef(u.Refs, unit.Ref{Module: pkg, Name: l.Value, Exact: true})
				}
				break
			}
		}
	}
}

func appendGoRef(list []unit.Ref, r unit.Ref) []unit.Ref {
	for _, existing := range list {
		if existing == r {
			return list
		}
	}
	return append(list, r)
}

func byOffset(snippets []*file.Snippet) {
	sort.SliceStable(snippets, func(i, j int) bool {
		return snippets[i].Begin.Offset < snippets[j].Begin.Offset
	})
}

// ownerIndex is which declaration a snippet sits inside: the last one that
// starts before it.
func ownerIndex(s *file.Snippet, declarations []*file.Snippet) int {
	last := -1
	for i, d := range declarations {
		if d.Begin.Offset <= s.Begin.Offset {
			last = i
		}
	}
	return last
}

// nearestBefore is the value of the closest snippet starting before this one.
func nearestBefore(s *file.Snippet, candidates []*file.Snippet) string {
	best := ""
	for _, c := range candidates {
		if c.Begin.Offset < s.Begin.Offset {
			best = c.Value
		}
	}
	return best
}
