// Package dart analyses Dart, and so Flutter.
//
// A Dart library is a file. `import 'package:wonders/logic/app_logic.dart'`
// names a file through the package's pubspec name, and makes every public
// name in it -- and in whatever it exports -- visible. So units are named by
// their file, `lib/logic/app_logic#AppLogic`, imports resolve through the
// pubspec to a file, and a name a file uses resolves to the library it
// imported that declares it. The first two need every pubspec and every file
// in hand, so they are done by the linker once parsing is over.
//
// Before this pack, Wonderous' 192 files, immich's 816 and AppFlowy's 1,976
// were in no component at all.
package dart

import (
	"sort"
	"strings"
	"unicode"

	dart "github.com/UserNobody14/tree-sitter-dart/bindings/go"
	"github.com/archstats/archstats/core"
	"github.com/archstats/archstats/core/file"
	"github.com/archstats/archstats/core/unit"
	"github.com/archstats/archstats/extensions/treesitter/common"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

type Extension struct{}

func (e *Extension) Init(settings core.Analyzer) error {
	settings.RegisterFileAnalyzer(&analyzer{lp: createPack()})
	settings.RegisterFileResultsEditor(&linker{root: settings.RootPath()})
	return nil
}

// placeholder stands for the file's library path in unit IDs until the
// linker writes it; a file does know its own path, but the pack keeps the
// ID scheme in one place.
const placeholder = "\x00dart"

const (
	captureClass     = "dart__class"
	captureMixin     = "dart__mixin"
	captureEnum      = "dart__enum"
	captureExtension = "dart__extension"
	captureExtended  = "dart__extended"
	captureTypeSpan  = "dart__type_span"
	captureFunc      = "dart__func"
	captureMethod    = "dart__method"
	captureSuper     = "dart__super"
	captureAnnot     = "dart__annotation"
	captureTopAnnot  = "dart__top_annotation"
	captureImport    = "dart__import"
	captureExport    = "dart__export"
	capturePart      = "dart__part"
	captureRef       = "dart__ref"
)

func createPack() *common.LanguagePack {
	template := &common.LanguagePackTemplate{
		FileGlob: "**.dart",
		Language: tree_sitter.NewLanguage(dart.Language()),
		QueriesForStats: []string{
			`(library_import (import_specification (configurable_uri (uri (string_literal) @` + file.ImportRaw + `))))`,
			`(class_definition name: (identifier) @` + file.Type + `)`,
			`(mixin_declaration (identifier) @` + file.Type + `)`,
			`(enum_declaration name: (identifier) @` + file.Type + `)`,
			`(class_definition (abstract) name: (identifier) @` + file.AbstractType + `)`,
			`(class_definition (sealed) name: (identifier) @` + file.AbstractType + `)`,
			`(mixin_declaration (identifier) @` + file.AbstractType + `)`,
		},
		ComponentResolution: common.DirectoryBasedComponentResolution,
		QueriesForSnippets: []string{
			`(class_definition name: (identifier) @` + captureClass + `)`,
			`(mixin_declaration (identifier) @` + captureMixin + `)`,
			`(enum_declaration name: (identifier) @` + captureEnum + `)`,
			`(extension_declaration name: (identifier) @` + captureExtension + ` class: (_) @` + captureExtended + `)`,
			`(class_definition) @` + captureTypeSpan,
			`(mixin_declaration) @` + captureTypeSpan,
			`(enum_declaration) @` + captureTypeSpan,
			`(extension_declaration) @` + captureTypeSpan,
			`(program (function_signature name: (identifier) @` + captureFunc + `))`,
			`(method_signature (function_signature name: (identifier) @` + captureMethod + `))`,
			`(superclass . (type_identifier) @` + captureSuper + `)`,
			`(mixins (type_identifier) @` + captureSuper + `)`,
			`(interfaces (type_identifier) @` + captureSuper + `)`,
			`(class_definition (annotation name: (identifier) @` + captureAnnot + `))`,
			`(program (annotation name: (identifier) @` + captureTopAnnot + `))`,
			`(library_import (import_specification (configurable_uri (uri (string_literal) @` + captureImport + `))))`,
			`(library_export (configurable_uri (uri (string_literal) @` + captureExport + `)))`,
			`(part_directive (uri (string_literal) @` + capturePart + `))`,
			`(type_identifier) @` + captureRef,
			`(identifier) @` + captureRef,
		},
	}
	pack, err := common.PackFromTemplate(template)
	if err != nil {
		panic(err)
	}
	return pack
}

type analyzer struct {
	lp *common.LanguagePack
}

func (a *analyzer) AnalyzeFile(f file.File) *file.Results {
	return a.analyze(f.Path(), f.Content())
}

func (a *analyzer) analyze(path string, content []byte) *file.Results {
	res := a.lp.AnalyzeFileContent(path, content)
	if res == nil {
		return nil
	}
	res.Units = unitsFrom(path, res.Snippets, content)
	kept := res.Snippets[:0]
	for _, sn := range res.Snippets {
		switch sn.Type {
		case captureClass, captureMixin, captureEnum, captureExtension, captureExtended, captureTypeSpan,
			captureFunc, captureMethod, captureSuper, captureAnnot, captureTopAnnot:
			continue
		case captureRef:
			if !capitalised(sn.Value) {
				continue
			}
		case file.ImportRaw:
			// `flutter/material.dart`, `flutter_riverpod/flutter_riverpod.dart`:
			// the package first, as detection reads it.
			sn.Value = strings.TrimPrefix(unquote(sn.Value), "package:")
		case captureImport, captureExport, capturePart:
			// The linker needs the scheme to tell a package from a path.
			sn.Value = unquote(sn.Value)
		}
		kept = append(kept, sn)
	}
	res.Snippets = kept
	return res
}

func unquote(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(strings.TrimPrefix(s, "r"), "R")
	return strings.Trim(s, `'"`)
}

func capitalised(s string) bool {
	for _, r := range s {
		return unicode.IsUpper(r)
	}
	return false
}

func unitsFrom(path string, snippets []*file.Snippet, content []byte) []*unit.Unit {
	var types, extended, spans, funcs, methods, supers, annots, topAnnots, refs []*file.Snippet
	for _, s := range snippets {
		switch s.Type {
		case captureClass, captureMixin, captureEnum, captureExtension:
			types = append(types, s)
		case captureExtended:
			extended = append(extended, s)
		case captureTypeSpan:
			spans = append(spans, s)
		case captureFunc:
			funcs = append(funcs, s)
		case captureMethod:
			methods = append(methods, s)
		case captureSuper:
			supers = append(supers, s)
		case captureAnnot:
			annots = append(annots, s)
		case captureTopAnnot:
			topAnnots = append(topAnnots, s)
		case captureRef:
			refs = append(refs, s)
		}
	}
	sortByOffset(types, funcs, methods, spans)
	id := func(name string) string { return placeholder + "#" + name }

	var out []*unit.Unit
	type owned struct{ begin, end, unit int }
	var typeSpans []owned
	for _, t := range types {
		u := &unit.Unit{ID: id(t.Value), Kind: unit.KindType, Name: t.Value, Files: []string{path}}
		keyword := map[string]string{captureClass: "class", captureMixin: "mixin", captureEnum: "enum", captureExtension: "extension"}[t.Type]
		for _, sp := range spans {
			if t.Begin.Offset < sp.Begin.Offset || t.Begin.Offset > sp.End.Offset {
				continue
			}
			typeSpans = append(typeSpans, owned{sp.Begin.Offset, sp.End.Offset, len(out)})
			// The modifiers are the words before the name: `abstract
			// interface class`, `sealed class`.
			head := string(content[sp.Begin.Offset:t.Begin.Offset])
			for _, mod := range []string{"abstract", "sealed", "interface", "base", "final"} {
				if strings.Contains(" "+head+" ", " "+mod+" ") {
					u.Markers = append(u.Markers, unit.Marker{Source: sourceKeyword, Key: mod})
				}
			}
			if strings.Contains(" "+head+" ", " abstract ") && strings.Contains(" "+head+" ", " interface ") {
				u.Markers = append(u.Markers, unit.Marker{Source: unit.SourceSupertype, Key: "interface"})
			}
			break
		}
		u.Markers = append(u.Markers, unit.Marker{Source: sourceKeyword, Key: keyword})
		if t.Type == captureExtension {
			for _, e := range extended {
				if e.Begin.Offset > t.Begin.Offset {
					u.Markers = append(u.Markers, unit.Marker{Source: unit.SourceSupertype, Key: typeName(e.Value)})
					break
				}
			}
		}
		out = append(out, u)
	}
	typeAt := func(offset int) int {
		best, size := -1, 0
		for _, sp := range typeSpans {
			if offset >= sp.begin && offset <= sp.end && (best < 0 || sp.end-sp.begin < size) {
				best, size = sp.unit, sp.end-sp.begin
			}
		}
		return best
	}
	for _, s := range supers {
		if i := typeAt(s.Begin.Offset); i >= 0 {
			addMarker(out[i], unit.SourceSupertype, s.Value)
		}
	}
	for _, a := range annots {
		if i := typeAt(a.Begin.Offset); i >= 0 {
			addMarker(out[i], unit.SourceAnnotation, a.Value)
		}
	}

	// Methods belong to the type around them; top-level functions to nothing.
	// A body is the signature's sibling, not its child, so what a function
	// mentions is read as everything from its signature to the next one.
	type decl struct{ at, unit int }
	var methodStarts, funcStarts []decl
	for _, m := range methods {
		owner := typeAt(m.Begin.Offset)
		if owner < 0 {
			continue
		}
		mid := out[owner].ID + "." + m.Value
		out = append(out, &unit.Unit{ID: mid, Kind: unit.KindFunction, Name: m.Value, Files: []string{path}, Owner: out[owner].ID})
		methodStarts = append(methodStarts, decl{m.Begin.Offset, len(out) - 1})
	}
	for _, f := range funcs {
		out = append(out, &unit.Unit{ID: id(f.Value), Kind: unit.KindFunction, Name: f.Value, Files: []string{path}})
		funcStarts = append(funcStarts, decl{f.Begin.Offset, len(out) - 1})
	}

	// A top-level annotation is written before the declaration it marks, as
	// its sibling: `@riverpod` on a provider function.
	var starts []decl
	for i, t := range types {
		starts = append(starts, decl{t.Begin.Offset, i})
	}
	starts = append(starts, funcStarts...)
	sort.Slice(starts, func(i, j int) bool { return starts[i].at < starts[j].at })
	for _, a := range topAnnots {
		for _, d := range starts {
			if d.at > a.Begin.Offset {
				addMarker(out[d.unit], unit.SourceAnnotation, a.Value)
				break
			}
		}
	}

	for _, r := range refs {
		if !capitalised(r.Value) {
			continue
		}
		at := -1
		if owner := typeAt(r.Begin.Offset); owner >= 0 {
			at = owner
			for _, m := range methodStarts {
				if m.at <= r.Begin.Offset && out[m.unit].Owner == out[owner].ID {
					at = m.unit
				}
			}
		} else {
			for _, f := range funcStarts {
				if f.at <= r.Begin.Offset {
					at = f.unit
				}
			}
		}
		if at < 0 || out[at].Name == r.Value {
			continue
		}
		ref := unit.Ref{Module: placeholder, Name: r.Value, Exact: true}
		if !hasRef(out[at].Refs, ref) {
			out[at].Refs = append(out[at].Refs, ref)
		}
	}
	return out
}

const sourceKeyword = "keyword"

func typeName(s string) string {
	if i := strings.IndexAny(s, "<?"); i >= 0 {
		s = s[:i]
	}
	return strings.TrimSpace(s)
}

func sortByOffset(lists ...[]*file.Snippet) {
	for _, l := range lists {
		sort.SliceStable(l, func(i, j int) bool { return l[i].Begin.Offset < l[j].Begin.Offset })
	}
}

func addMarker(u *unit.Unit, source, key string) {
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
