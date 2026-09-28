// Package objc analyses Objective-C: .m, .mm and the headers beside them.
//
// Objective-C has one namespace per target, like Swift, and the two share it
// in a mixed app: WordPress-iOS and Signal-iOS carry hundreds of Objective-C
// files among thousands of Swift ones, and the Swift calls into them through
// a bridging header. So the units are named the Swift way,
// `<target>#SDWebImageManager`, and resolved by the same linker. A class's
// @interface, its class extension and its @implementation are one unit; a
// category (`@interface UIView (WebCache)`) is more of the class it extends.
package objc

import (
	"sort"
	"strings"
	"unicode"

	"github.com/archstats/archstats/core"
	"github.com/archstats/archstats/core/file"
	"github.com/archstats/archstats/core/unit"
	"github.com/archstats/archstats/extensions/treesitter/apple"
	"github.com/archstats/archstats/extensions/treesitter/common"
	objc "github.com/tree-sitter-grammars/tree-sitter-objc/bindings/go"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

type Extension struct{}

func (e *Extension) Init(settings core.Analyzer) error {
	settings.RegisterFileAnalyzer(&analyzer{lp: createPack()})
	settings.RegisterFileResultsEditor(&apple.Linker{Root: settings.RootPath()})
	return nil
}

const (
	captureClass    = "objc__class"
	captureCategory = "objc__category"
	captureProtocol = "objc__protocol"
	captureSpan     = "objc__span"
	captureMethod   = "objc__method"
	captureMSpan    = "objc__method_span"
	captureSuper    = "objc__super"
	captureConform  = "objc__conform"
)

func createPack() *common.LanguagePack {
	template := &common.LanguagePackTemplate{
		FileGlob: "**.{m,mm,h}",
		Language: tree_sitter.NewLanguage(objc.Language()),
		QueriesForStats: []string{
			// `#import <UIKit/UIKit.h>` and `@import UIKit;` name a framework;
			// kept as the framework's name for detection.
			`(preproc_include path: (system_lib_string) @` + file.ImportRaw + `)`,
			`(module_import path: (identifier) @` + file.ImportRaw + `)`,
			`(class_interface . (identifier) @` + file.Type + ` !category)`,
			`(protocol_declaration . (identifier) @` + file.Type + `)`,
			`(protocol_declaration . (identifier) @` + file.AbstractType + `)`,
		},
		ComponentResolution: common.DirectoryBasedComponentResolution,
		QueriesForSnippets: []string{
			`(class_interface . (identifier) @` + captureClass + ` !category)`,
			`(class_implementation . (identifier) @` + captureClass + ` !category)`,
			`(class_interface . (identifier) @` + captureCategory + ` category: (_))`,
			`(class_implementation . (identifier) @` + captureCategory + ` category: (_))`,
			`(protocol_declaration . (identifier) @` + captureProtocol + `)`,
			`(class_interface) @` + captureSpan,
			`(class_implementation) @` + captureSpan,
			`(protocol_declaration) @` + captureSpan,
			`(method_declaration (identifier) @` + captureMethod + `)`,
			`(method_definition (identifier) @` + captureMethod + `)`,
			`(method_declaration) @` + captureMSpan,
			`(method_definition) @` + captureMSpan,
			`(class_interface superclass: (identifier) @` + captureSuper + `)`,
			`(class_interface (parameterized_arguments (type_name (type_identifier) @` + captureConform + `)))`,
			`(protocol_declaration (protocol_reference_list (identifier) @` + captureConform + `))`,
			`(preproc_include path: (string_literal (string_content) @` + apple.CaptureInclude + `))`,
			`(type_identifier) @` + apple.CaptureRef,
			`(identifier) @` + apple.CaptureRef,
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
	res.Units = unitsFrom(path, res.Snippets)
	kept := res.Snippets[:0]
	for _, sn := range res.Snippets {
		switch sn.Type {
		case captureClass, captureCategory, captureProtocol, captureSpan, captureMethod, captureMSpan, captureSuper, captureConform:
			continue
		case apple.CaptureRef:
			if !capitalised(sn.Value) {
				continue
			}
		case file.ImportRaw:
			sn.Value = framework(sn.Value)
		}
		kept = append(kept, sn)
	}
	res.Snippets = kept
	return res
}

// framework turns `<UIKit/UIKit.h>` into UIKit, the name Swift would import.
func framework(v string) string {
	v = strings.Trim(v, "<>\" ")
	if i := strings.Index(v, "/"); i > 0 {
		return v[:i]
	}
	return strings.TrimSuffix(v, ".h")
}

func capitalised(s string) bool {
	for _, r := range s {
		return unicode.IsUpper(r)
	}
	return false
}

func unitsFrom(path string, snippets []*file.Snippet) []*unit.Unit {
	var types, spans, methods, mspans, supers, conforms, refs []*file.Snippet
	for _, s := range snippets {
		switch s.Type {
		case captureClass, captureCategory, captureProtocol:
			types = append(types, s)
		case captureSpan:
			spans = append(spans, s)
		case captureMethod:
			methods = append(methods, s)
		case captureMSpan:
			mspans = append(mspans, s)
		case captureSuper:
			supers = append(supers, s)
		case captureConform:
			conforms = append(conforms, s)
		case apple.CaptureRef:
			refs = append(refs, s)
		}
	}
	sort.SliceStable(types, func(i, j int) bool { return types[i].Begin.Offset < types[j].Begin.Offset })
	id := func(name string) string { return apple.Placeholder + "#" + name }

	// One unit per type name in the file: a class's @interface and
	// @implementation in one .m are the same unit.
	var out []*unit.Unit
	byName := map[string]int{}
	var declaredBy []*file.Snippet
	for _, t := range types {
		keyword := map[string]string{captureClass: "class", captureCategory: "extension", captureProtocol: "protocol"}[t.Type]
		if i, ok := byName[t.Value]; ok {
			// A class declared here and also given a category here is the
			// class.
			if keyword != "extension" {
				setKeyword(out[i], keyword)
			}
			declaredBy = append(declaredBy, t)
			continue
		}
		u := &unit.Unit{ID: id(t.Value), Kind: unit.KindType, Name: t.Value, Files: []string{path}}
		setKeyword(u, keyword)
		if keyword == "protocol" {
			u.Markers = append(u.Markers, unit.Marker{Source: unit.SourceSupertype, Key: "interface"})
		}
		byName[t.Value] = len(out)
		out = append(out, u)
		declaredBy = append(declaredBy, t)
	}
	// declaredBy may hold a name twice; spans map to the unit by name.
	unitOf := func(d *file.Snippet) int { return byName[d.Value] }
	type owned struct{ begin, end, unit int }
	var typeOwned []owned
	for _, sp := range spans {
		for _, d := range declaredBy {
			if d.Begin.Offset >= sp.Begin.Offset && d.Begin.Offset <= sp.End.Offset {
				typeOwned = append(typeOwned, owned{sp.Begin.Offset, sp.End.Offset, unitOf(d)})
				break
			}
		}
	}
	typeAt := func(offset int) int {
		for _, o := range typeOwned {
			if offset >= o.begin && offset <= o.end {
				return o.unit
			}
		}
		return -1
	}
	for _, s := range supers {
		if i := typeAt(s.Begin.Offset); i >= 0 {
			addMarker(out[i], unit.SourceSupertype, s.Value)
		}
	}
	for _, c := range conforms {
		if i := typeAt(c.Begin.Offset); i >= 0 {
			addMarker(out[i], unit.SourceSupertype, c.Value)
		}
	}

	// Methods, by their selector's first part, belong to the type around
	// them. A method declared in the @interface and defined in the
	// @implementation is one unit.
	typeCount := len(out)
	methodIndex := map[string]int{}
	var methodOwned []owned
	for _, m := range methods {
		owner := typeAt(m.Begin.Offset)
		if owner < 0 || owner >= typeCount {
			continue
		}
		mid := out[owner].ID + "." + m.Value
		i, seen := methodIndex[mid]
		if !seen {
			i = len(out)
			methodIndex[mid] = i
			out = append(out, &unit.Unit{ID: mid, Kind: unit.KindFunction, Name: m.Value, Files: []string{path}, Owner: out[owner].ID})
		}
		for _, sp := range mspans {
			if m.Begin.Offset >= sp.Begin.Offset && m.Begin.Offset <= sp.End.Offset {
				methodOwned = append(methodOwned, owned{sp.Begin.Offset, sp.End.Offset, i})
				break
			}
		}
	}

	for _, r := range refs {
		if !capitalised(r.Value) {
			continue
		}
		at := -1
		for _, o := range methodOwned {
			if r.Begin.Offset >= o.begin && r.Begin.Offset <= o.end {
				at = o.unit
				break
			}
		}
		if at < 0 {
			at = typeAt(r.Begin.Offset)
		}
		if at < 0 || out[at].Name == r.Value {
			continue
		}
		ref := unit.Ref{Module: apple.Placeholder, Name: r.Value, Exact: true}
		dup := false
		for _, x := range out[at].Refs {
			if x == ref {
				dup = true
				break
			}
		}
		if !dup {
			out[at].Refs = append(out[at].Refs, ref)
		}
	}
	return out
}

// setKeyword records what a type was declared as, a real declaration
// outranking a category.
func setKeyword(u *unit.Unit, key string) {
	for i, m := range u.Markers {
		if m.Source == apple.SourceKeyword {
			if m.Key == "extension" {
				u.Markers[i].Key = key
			}
			return
		}
	}
	u.Markers = append(u.Markers, unit.Marker{Source: apple.SourceKeyword, Key: key})
}

func addMarker(u *unit.Unit, source, key string) {
	for _, m := range u.Markers {
		if m.Source == source && m.Key == key {
			return
		}
	}
	u.Markers = append(u.Markers, unit.Marker{Source: source, Key: key})
}
