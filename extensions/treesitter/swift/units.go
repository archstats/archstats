package swift

import (
	"regexp"
	"sort"
	"strings"
	"unicode"

	"github.com/archstats/archstats/core/file"
	"github.com/archstats/archstats/core/unit"
	"github.com/archstats/archstats/extensions/treesitter/apple"
	"github.com/archstats/archstats/extensions/treesitter/common"
)

// Units for Swift. A type's ID is `<target>#<Name>`, but a file does not know
// its target; the pack writes a placeholder and the linker fills it in once
// the targets are read.
//
// An extension is where much of a Swift type is written -- its conformances,
// its view builders, whole protocols' worth of methods -- usually in another
// file. It is read as more of the type it extends: its methods belong to
// that type, and its conformances are that type's supertypes.

const placeholder = apple.Placeholder

const (
	captureClass     = "swift__class"
	captureStruct    = "swift__struct"
	captureEnum      = "swift__enum"
	captureActor     = "swift__actor"
	captureProtocol  = "swift__protocol"
	captureExtension = "swift__extension"
	captureTypeSpan  = "swift__type_span"
	captureFunc      = "swift__func"
	captureFuncSpan  = "swift__func_span"
	captureSuper     = "swift__super"
	captureAttr      = "swift__attr"
	captureWrapper   = "swift__wrapper"
	captureSkip      = "swift__skip"
	// A generic parameter's name: `Content` in `struct Row<Content: View>`
	// is no type of the codebase's, whatever else is called Content.
	captureGeneric = "swift__generic"
	// A dotted name, `CounterFeature.State` as a type or as an expression.
	// A nested type answers only to its full name, so the bare parts alone
	// would never reach it.
	captureQualified = "swift__qualified"
	captureRef       = apple.CaptureRef
)

var qualifiedName = regexp.MustCompile(`^[A-Z]\w*(\.[A-Z]\w*)+$`)

var keywordOf = map[string]string{
	captureClass: "class", captureStruct: "struct", captureEnum: "enum", captureActor: "actor",
	captureProtocol: "protocol", captureExtension: "extension",
}

func unitQueries() []string {
	decl := func(kind, capture string) string {
		return `(class_declaration declaration_kind: "` + kind + `" name: (type_identifier) @` + capture + `)`
	}
	return []string{
		decl("class", captureClass),
		decl("struct", captureStruct),
		decl("enum", captureEnum),
		decl("actor", captureActor),
		`(protocol_declaration name: (type_identifier) @` + captureProtocol + `)`,
		`(class_declaration declaration_kind: "extension" name: (user_type) @` + captureExtension + `)`,
		`(class_declaration) @` + captureTypeSpan,
		`(protocol_declaration) @` + captureTypeSpan,
		`(function_declaration name: (simple_identifier) @` + captureFunc + `)`,
		`(function_declaration) @` + captureFuncSpan,
		// Superclass and conformances are written alike; the key is the
		// last name of the type, so `SwiftUI.App` is App.
		`(class_declaration (inheritance_specifier inherits_from: (user_type) @` + captureSuper + `))`,
		`(protocol_declaration (inheritance_specifier inherits_from: (user_type) @` + captureSuper + `))`,
		`(class_declaration (modifiers (attribute (user_type) @` + captureAttr + `)))`,
		`(protocol_declaration (modifiers (attribute (user_type) @` + captureAttr + `)))`,
		`(function_declaration (modifiers (attribute (user_type) @` + captureAttr + `)))`,
		// A property wrapper says what a type holds: @State and @Environment
		// make a SwiftUI view, @Published an observable model, @Dependency a
		// TCA feature. It marks the type the property is declared in.
		`(property_declaration (modifiers (attribute (user_type) @` + captureWrapper + `)))`,
		`(type_identifier) @` + captureRef,
		`(simple_identifier) @` + captureRef,
		`(user_type (type_identifier) (type_identifier)) @` + captureQualified,
		`(navigation_expression target: (simple_identifier) suffix: (navigation_suffix suffix: (simple_identifier))) @` + captureQualified,
		`(import_declaration) @` + captureSkip,
		`(type_parameter . (type_identifier) @` + captureGeneric + `)`,
	}
}

type swiftAnalyzer struct {
	lp *common.LanguagePack
}

func (a *swiftAnalyzer) ClaimsFile(path string) bool { return a.lp.ClaimsFile(path) }

func (a *swiftAnalyzer) AnalyzeFile(f file.File) *file.Results {
	return a.analyze(f.Path(), f.Content())
}

func (a *swiftAnalyzer) analyze(path string, content []byte) *file.Results {
	res := a.lp.AnalyzeFileContent(path, content)
	if res == nil {
		return nil
	}
	res.Units = unitsFrom(path, res.Snippets)
	generic := genericNames(res.Snippets)
	kept := res.Snippets[:0]
	for _, sn := range res.Snippets {
		switch sn.Type {
		case captureClass, captureStruct, captureEnum, captureActor, captureProtocol, captureExtension,
			captureTypeSpan, captureFunc, captureFuncSpan, captureSuper, captureAttr, captureWrapper, captureSkip, captureGeneric, captureQualified:
			continue
		case captureRef:
			if !capitalised(sn.Value) || generic[sn.Value] {
				continue
			}
		}
		kept = append(kept, sn)
	}
	res.Snippets = kept
	return res
}

func genericNames(snippets []*file.Snippet) map[string]bool {
	out := map[string]bool{}
	for _, s := range snippets {
		if s.Type == captureGeneric {
			out[s.Value] = true
		}
	}
	return out
}

func capitalised(s string) bool {
	for _, r := range s {
		return unicode.IsUpper(r)
	}
	return false
}

// typeName is the name a user_type spells: `SwiftUI.App` is App to a
// conformance, `Array<Int>` is Array, `Foo.Bar` extended is Foo.Bar.
func typeName(s string, qualified bool) string {
	if i := strings.IndexAny(s, "<("); i >= 0 {
		s = s[:i]
	}
	s = strings.Join(strings.Fields(s), "")
	if !qualified {
		if i := strings.LastIndex(s, "."); i >= 0 {
			s = s[i+1:]
		}
	}
	return s
}

func unitsFrom(path string, snippets []*file.Snippet) []*unit.Unit {
	var names, typeSpans, funcs, funcSpans, supers, attrs, wrappers, refs, skips []*file.Snippet
	for _, s := range snippets {
		switch s.Type {
		case captureQualified:
			// `CounterFeature.State`, as one reference to the nested type,
			// beside the references its parts make on their own.
			if v := typeName(s.Value, true); qualifiedName.MatchString(v) {
				c := *s
				c.Value = v
				refs = append(refs, &c)
			}
		case captureClass, captureStruct, captureEnum, captureActor, captureProtocol:
			names = append(names, s)
		case captureExtension:
			// Read as the name it extends, so what is nested inside is named
			// through it.
			c := *s
			c.Value = typeName(s.Value, true)
			names = append(names, &c)
		case captureTypeSpan:
			typeSpans = append(typeSpans, s)
		case captureFunc:
			funcs = append(funcs, s)
		case captureFuncSpan:
			funcSpans = append(funcSpans, s)
		case captureSuper:
			supers = append(supers, s)
		case captureAttr:
			attrs = append(attrs, s)
		case captureWrapper:
			wrappers = append(wrappers, s)
		case captureRef:
			refs = append(refs, s)
		case captureSkip:
			skips = append(skips, s)
		}
	}
	sort.SliceStable(names, func(i, j int) bool { return names[i].Begin.Offset < names[j].Begin.Offset })
	nested := common.NestedNames(names, typeSpans)
	id := func(name string) string { return placeholder + "#" + name }

	var out []*unit.Unit
	var declaredBy []*file.Snippet
	for _, n := range names {
		full := nested[n]
		u := &unit.Unit{ID: id(full), Kind: unit.KindType, Name: n.Value, Files: []string{path}}
		if n.Type == captureExtension {
			u.Name = typeName(n.Value, false)
		}
		if i := strings.LastIndex(full, "."); i > 0 && n.Type != captureExtension {
			u.Owner = id(full[:i])
		}
		u.Markers = append(u.Markers, unit.Marker{Source: sourceKeyword, Key: keywordOf[n.Type]})
		if n.Type == captureProtocol {
			// The convention every pack follows for an interface; the UI
			// reads it as one.
			u.Markers = append(u.Markers, unit.Marker{Source: unit.SourceSupertype, Key: "interface"})
		}
		out = append(out, u)
		declaredBy = append(declaredBy, n)
	}
	typeOwned := ownedSpans(declaredBy, typeSpans)

	// The innermost type or extension around an offset.
	typeAt := func(offset int) int {
		for _, sp := range typeOwned {
			if offset >= sp.begin && offset <= sp.end {
				return sp.unit
			}
		}
		return -1
	}

	for _, f := range funcs {
		owner := typeAt(f.Begin.Offset)
		u := &unit.Unit{Kind: unit.KindFunction, Name: f.Value, Files: []string{path}}
		if owner >= 0 {
			ownerName := strings.TrimPrefix(out[owner].ID, id(""))
			u.ID = id(ownerName + "." + f.Value)
			u.Owner = out[owner].ID
		} else {
			u.ID = id(f.Value)
		}
		out = append(out, u)
		declaredBy = append(declaredBy, f)
	}
	funcOwned := ownedSpans(declaredBy, funcSpans)

	for _, s := range supers {
		if i := typeAt(s.Begin.Offset); i >= 0 {
			addMarker(out[i], unit.SourceSupertype, typeName(s.Value, false))
		}
	}
	// An attribute belongs to the declaration whose modifiers hold it: the
	// innermost function or type around it.
	all := append(append([]ownedSpan{}, funcOwned...), typeOwned...)
	sort.SliceStable(all, func(i, j int) bool { return all[i].end-all[i].begin < all[j].end-all[j].begin })
	innermost := func(offset int) int {
		for _, sp := range all {
			if offset >= sp.begin && offset <= sp.end {
				return sp.unit
			}
		}
		return -1
	}
	for _, a := range attrs {
		if i := innermost(a.Begin.Offset); i >= 0 {
			addMarker(out[i], unit.SourceAnnotation, typeName(a.Value, false))
		}
	}
	for _, w := range wrappers {
		if i := typeAt(w.Begin.Offset); i >= 0 {
			addMarker(out[i], unit.SourceAnnotation, typeName(w.Value, false))
		}
	}

	// The types this file declares by their full names, for the names a
	// declaration uses from inside its own type: `State` inside
	// CounterFeature means CounterFeature.State, as it does to the compiler.
	declared := map[string]bool{}
	for _, n := range names {
		if n.Type != captureExtension {
			declared[nested[n]] = true
		}
	}
	inScope := func(from *unit.Unit, name string) string {
		scope := strings.TrimPrefix(from.ID, id(""))
		for scope != "" {
			if declared[scope+"."+name] {
				return scope + "." + name
			}
			if i := strings.LastIndex(scope, "."); i >= 0 {
				scope = scope[:i]
			} else {
				scope = ""
			}
		}
		return name
	}

	// References: every capitalised name, owned by the innermost declaration
	// around it. Resolved by the linker, which knows the targets.
	generic := genericNames(snippets)
	for _, r := range refs {
		if !capitalised(r.Value) || generic[r.Value] {
			continue
		}
		skipped := false
		for _, sk := range skips {
			if r.Begin.Offset >= sk.Begin.Offset && r.Begin.Offset <= sk.End.Offset {
				skipped = true
				break
			}
		}
		if skipped {
			continue
		}
		i := innermost(r.Begin.Offset)
		if i < 0 || out[i].Name == r.Value {
			continue
		}
		ref := unit.Ref{Module: placeholder, Name: inScope(out[i], r.Value), Exact: true}
		if !hasRef(out[i].Refs, ref) {
			out[i].Refs = append(out[i].Refs, ref)
		}
	}
	return out
}

const sourceKeyword = apple.SourceKeyword

func addMarker(u *unit.Unit, source, key string) {
	if key == "" {
		return
	}
	for _, m := range u.Markers {
		if m.Source == source && m.Key == key {
			return
		}
	}
	u.Markers = append(u.Markers, unit.Marker{Source: source, Key: key})
}

func hasRef(refs []unit.Ref, r unit.Ref) bool {
	for _, x := range refs {
		if x == r {
			return true
		}
	}
	return false
}

type ownedSpan struct{ begin, end, unit int }

// ownedSpans pairs each declaration span with the unit whose name is the
// first declared inside it, smallest first.
func ownedSpans(declaredBy, spans []*file.Snippet) []ownedSpan {
	var out []ownedSpan
	for _, sp := range spans {
		best, bestOffset := -1, 0
		for i, d := range declaredBy {
			if off := d.Begin.Offset; off >= sp.Begin.Offset && off <= sp.End.Offset && (best == -1 || off < bestOffset) {
				best, bestOffset = i, off
			}
		}
		if best >= 0 {
			out = append(out, ownedSpan{sp.Begin.Offset, sp.End.Offset, best})
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].end-out[i].begin < out[j].end-out[j].begin })
	return out
}
