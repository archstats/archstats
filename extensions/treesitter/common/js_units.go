package common

import (
	"path"
	"sort"
	"strings"

	"github.com/archstats/archstats/core/file"
	"github.com/archstats/archstats/core/unit"
)

// Units for the two grammars that share a language.
//
// JavaScript and TypeScript are the case that makes a units view worth
// having at all: LibreChat is 997 functions to 14 classes, so a reading that
// looks only at classes sees fourteen things in a codebase of a thousand.
// Almost everything an architect points at here -- a React component, a
// hook, an Express handler, a utility -- is a function.
//
// The module is the file. There is no package clause to read, and two files
// may each export a `default` or a `handler`, so a unit is qualified by the
// module it lives in.

// Captures both packs emit. Shared so the two grammars cannot drift apart.
const (
	CaptureJSClass     = "js__class__declaration"
	CaptureJSInterface = "js__interface__declaration"
	CaptureJSFunction  = "js__function__declaration"
	CaptureJSMethod    = "js__method__definition"
	CaptureJSDecorator = "js__decorator"
	// What an import clause took, and where from. `import { parse } from
	// "./reader"` names both the unit and the module it lives in, which is
	// the only way most languages say which unit is depended on -- Java
	// imports the type itself, which is why it had a class graph and nothing
	// else did.
	CaptureJSBinding    = "js__import__binding"
	CaptureJSImportSpan = "js__import__span"
	// The whole declaration, so a usage can be attributed to the unit whose
	// body contains it rather than to every unit in the file.
	CaptureJSSpan = "js__declaration__span"
	// A class's whole body, so a method belongs to the class that holds it.
	CaptureJSClassSpan = "js__class__span"
	// `export default class extends Controller {}`: a class with no name of
	// its own, which the file's default export names.
	CaptureJSDefaultClass = "js__class__default"
	// The factory a constant is made by: `defineStore` in `const useCart =
	// defineStore(...)`. Such a constant is a unit, the factory its marker.
	CaptureJSFactory = "js__factory__call"
)

// jsFactories make the things an architect names that are neither a class nor
// a function: a Pinia store is `export const useCartStore = defineStore(...)`,
// a Redux Toolkit slice `const cartSlice = createSlice(...)`. Their actions
// are methods of an object literal or functions inside the call, and without
// a unit to hold them they were loose functions of the file, so a store read
// as its interfaces and nothing else. An allowlist, not every `define…`:
// Vue's `const props = defineProps()` is a compiler macro, not a unit.
const jsFactories = `^(defineStore|defineComponent|createSlice|createApi)$`

// JSUnitQueries are the snippet queries for a JavaScript-family grammar.
//
// `typeIdentifier` differs between them: TypeScript names a class with a
// type_identifier and JavaScript with a plain identifier, and a query naming
// the wrong node compiles to nothing rather than failing.
func JSUnitQueries(typeIdentifier string, withTypes bool) []string {
	queries := []string{
		`(class_declaration name: (` + typeIdentifier + `) @` + CaptureJSClass + `)`,

		// A function standing on its own, declared either way. `export
		// default function` and `const Chat = () => …` are the same kind of
		// thing to everybody except the parser.
		`(function_declaration name: (identifier) @` + CaptureJSFunction + `)`,
		`(generator_function_declaration name: (identifier) @` + CaptureJSFunction + `)`,
		`(lexical_declaration (variable_declarator
			name: (identifier) @` + CaptureJSFunction + `
			value: [(arrow_function) (function_expression)]))`,

		// A constant made by a factory: a store, a slice.
		`((lexical_declaration (variable_declarator
			name: (identifier) @` + CaptureJSClass + `
			value: (call_expression function: (identifier) @` + CaptureJSFactory + `)))
		  (#match? @` + CaptureJSFactory + ` "` + jsFactories + `"))`,
		`((lexical_declaration (variable_declarator
			value: (call_expression function: (identifier) @_factory))) @` + CaptureJSClassSpan + `
		  (#match? @_factory "` + jsFactories + `"))`,
		`((lexical_declaration (variable_declarator
			value: (call_expression function: (identifier) @_factory2))) @` + CaptureJSSpan + `
		  (#match? @_factory2 "` + jsFactories + `"))`,

		// A method belongs to the class above it, paired up by position.
		`(method_definition name: (property_identifier) @` + CaptureJSMethod + `)`,

		// The only thing in this family that looks like an annotation, and
		// it exists only in Angular and NestJS.
		`(decorator [
			(identifier) @` + CaptureJSDecorator + `
			(call_expression function: (identifier) @` + CaptureJSDecorator + `)
		])`,

		`(import_statement (import_clause (named_imports
			(import_specifier name: (identifier) @` + CaptureJSBinding + `))))`,
		`(import_statement (import_clause (identifier) @` + CaptureJSBinding + `))`,
		`(import_statement (import_clause (namespace_import (identifier) @` + CaptureJSBinding + `)))`,
		`(import_statement) @` + CaptureJSImportSpan,

		// CommonJS takes its names by destructuring instead. An enormous
		// amount of real JavaScript never writes the word `import`: express,
		// at the commit the fixtures pin, has 66 require() calls and no ESM
		// imports at all.
		`((lexical_declaration (variable_declarator
			name: (object_pattern (shorthand_property_identifier_pattern) @` + CaptureJSBinding + `)
			value: (call_expression
				function: (identifier) @_cjsreq
				arguments: (arguments (string)))))
		  (#eq? @_cjsreq "require"))`,
		`((lexical_declaration (variable_declarator
			name: (identifier) @` + CaptureJSBinding + `
			value: (call_expression
				function: (identifier) @_cjsreq2
				arguments: (arguments (string)))))
		  (#eq? @_cjsreq2 "require"))`,
		`((variable_declaration (variable_declarator
			name: (object_pattern (shorthand_property_identifier_pattern) @` + CaptureJSBinding + `)
			value: (call_expression
				function: (identifier) @_cjsreq3
				arguments: (arguments (string)))))
		  (#eq? @_cjsreq3 "require"))`,
		`((variable_declaration (variable_declarator
			name: (identifier) @` + CaptureJSBinding + `
			value: (call_expression
				function: (identifier) @_cjsreq4
				arguments: (arguments (string)))))
		  (#eq? @_cjsreq4 "require"))`,
		// The statement, so the name and the module it came from can be
		// paired: both sit inside it.
		`((lexical_declaration (variable_declarator
			value: (call_expression function: (identifier) @_cjsspan
				arguments: (arguments (string))))) @` + CaptureJSImportSpan + `
		  (#eq? @_cjsspan "require"))`,
		`((variable_declaration (variable_declarator
			value: (call_expression function: (identifier) @_cjsspan2
				arguments: (arguments (string))))) @` + CaptureJSImportSpan + `
		  (#eq? @_cjsspan2 "require"))`,

		`(function_declaration) @` + CaptureJSSpan,
		`(generator_function_declaration) @` + CaptureJSSpan,
		`(class_declaration) @` + CaptureJSSpan,
		`(class_declaration) @` + CaptureJSClassSpan,
		`(class) @` + CaptureJSClassSpan,
		`(export_statement value: (class) @` + CaptureJSDefaultClass + `)`,
		`(method_definition) @` + CaptureJSSpan,
		`(lexical_declaration (variable_declarator
			value: [(arrow_function) (function_expression)])) @` + CaptureJSSpan,
	}
	if withTypes {
		queries = append(queries,
			`(abstract_class_declaration name: (`+typeIdentifier+`) @`+CaptureJSClass+`)`,
			`(abstract_class_declaration) @`+CaptureJSClassSpan,
			`(interface_declaration name: (`+typeIdentifier+`) @`+CaptureJSInterface+`)`,
		)
	}
	return queries
}

// DefaultExportName is what an anonymous default export is called: its file,
// which is what an importer of the default almost always names it. The name
// ends at the first dot, so `Panel.client.vue` is Panel, except inside the
// brackets of a route parameter: `[...slug].vue` is [...slug], not "[".
func DefaultExportName(filePath string) string {
	base := path.Base(filePath)
	from := 0
	if strings.HasPrefix(base, "[") {
		if end := strings.LastIndex(base, "]"); end > 0 {
			from = end
		}
	}
	if i := strings.Index(base[from:], "."); from+i > 0 && i >= 0 {
		base = base[:from+i]
	}
	return base
}

// ModuleOf is the module a file is, which is its path without the extension.
// `client/src/utils/rules.ts` is the module `client/src/utils/rules`, and an
// index file is named by its directory the way an import writes it.
func ModuleOf(filePath string) string {
	p := path.Clean(strings.TrimPrefix(filePath, "./"))
	ext := path.Ext(p)
	p = strings.TrimSuffix(p, ext)
	// A package's own file is named by its directory: `index.ts` is what
	// `import "./components"` means, and Python's `__init__.py` is what
	// `from oscar import get_version` means.
	if base := path.Base(p); base == "index" || (base == "__init__" && ext == ".py") {
		if dir := path.Dir(p); dir != "." && dir != "/" {
			return dir
		}
	}
	return p
}

// JSUnitsFrom builds units from what the shared queries captured.
//
// content is the file, used to find where an imported name is actually used.
// Nothing in a tree-sitter query can ask "where is this identifier
// referenced", and capturing every identifier in a codebase would put
// millions of snippets in the database to throw almost all of them away, so
// the usages are found by scanning for the name and attributed to whichever
// declaration's span contains them.
func JSUnitsFrom(filePath string, content []byte, res *file.Results) []*unit.Unit {
	module := ModuleOf(filePath)

	var classes, interfaces, functions, methods, decorators, factories []*file.Snippet
	var bindings, importSpans, spans, sources, classSpans []*file.Snippet
	for _, s := range res.Snippets {
		switch s.Type {
		case CaptureJSClass:
			classes = append(classes, s)
		case CaptureJSInterface:
			interfaces = append(interfaces, s)
		case CaptureJSFunction:
			functions = append(functions, s)
		case CaptureJSMethod:
			methods = append(methods, s)
		case CaptureJSDecorator:
			decorators = append(decorators, s)
		case CaptureJSFactory:
			factories = append(factories, s)
		case CaptureJSBinding:
			bindings = append(bindings, s)
		case CaptureJSImportSpan:
			importSpans = append(importSpans, s)
		case CaptureJSClassSpan:
			classSpans = append(classSpans, s)
		case CaptureJSDefaultClass:
			// Named for the file, as whoever imports the default writes it.
			classes = append(classes, &file.Snippet{
				File: s.File, Type: CaptureJSClass, Component: s.Component,
				Value: DefaultExportName(filePath), Begin: s.Begin, End: s.Begin,
			})
		case CaptureJSSpan:
			spans = append(spans, s)
		case file.ImportRaw:
			sources = append(sources, s)
		}
	}
	byOffset(spans)
	byOffset(sources)
	byOffset(classes)
	byOffset(methods)
	byOffset(factories)

	var out []*unit.Unit
	// The snippet declaring each unit, index for index with out; nil where a
	// unit has no span of its own to own (an interface).
	var declaredBy []*file.Snippet
	for i, c := range classes {
		u := &unit.Unit{
			ID:    module + "#" + c.Value,
			Kind:  unit.KindType,
			Name:  c.Value,
			Files: []string{filePath},
		}
		// A decorator sits above the class it is about.
		for _, d := range decorators {
			if followingIndex(d, classes) == i {
				u.Markers = append(u.Markers, unit.Marker{Source: unit.SourceAnnotation, Key: d.Value})
			}
		}
		out = append(out, u)
		declaredBy = append(declaredBy, c)
	}
	// A factory call follows the name it is assigned to.
	factorySpan := map[int]bool{}
	for _, f := range factories {
		if i := precedingIndex(f, classes); i >= 0 {
			out[i].Markers = append(out[i].Markers, unit.Marker{Source: unit.SourceSupertype, Key: f.Value})
			factorySpan[i] = true
		}
	}
	for _, i := range interfaces {
		out = append(out, &unit.Unit{
			ID:      module + "#" + i.Value,
			Kind:    unit.KindType,
			Name:    i.Value,
			Files:   []string{filePath},
			Markers: []unit.Marker{{Source: unit.SourceSupertype, Key: "interface"}},
		})
		declaredBy = append(declaredBy, nil)
	}
	// A method belongs to the innermost class whose body holds it. It used
	// to belong to the class declared above it, so a method of an anonymous
	// class, or of an object literal below a class, joined whichever class
	// came before -- and Sylius's Stimulus controllers, all
	// `export default class extends Controller`, had every method loose at
	// the top of the file.
	classAt := func(at int) int {
		best, bestSize := -1, 0
		for _, sp := range classSpans {
			if at < sp.Begin.Offset || at > sp.End.Offset {
				continue
			}
			for i, c := range classes {
				if c.Begin.Offset >= sp.Begin.Offset && c.Begin.Offset <= sp.End.Offset {
					if size := sp.End.Offset - sp.Begin.Offset; best == -1 || size < bestSize {
						best, bestSize = i, size
					}
					break
				}
			}
		}
		return best
	}
	for _, m := range methods {
		owner := ""
		name := m.Value
		if idx := classAt(m.Begin.Offset); idx >= 0 {
			owner = module + "#" + classes[idx].Value
			name = classes[idx].Value + "." + m.Value
		}
		out = append(out, &unit.Unit{
			ID:    module + "#" + name,
			Kind:  unit.KindFunction,
			Name:  m.Value,
			Files: []string{filePath},
			Owner: owner,
		})
		declaredBy = append(declaredBy, m)
	}
	for _, f := range functions {
		// A function declared inside a class body is a method and has
		// already been recorded as one. One declared inside a factory call
		// -- a setup store's actions -- belongs to what the factory makes.
		u := &unit.Unit{
			ID:    module + "#" + f.Value,
			Kind:  unit.KindFunction,
			Name:  f.Value,
			Files: []string{filePath},
		}
		if idx := classAt(f.Begin.Offset); idx >= 0 && factorySpan[idx] {
			u.Owner = module + "#" + classes[idx].Value
			u.ID = module + "#" + classes[idx].Value + "." + f.Value
		}
		out = append(out, u)
		declaredBy = append(declaredBy, f)
	}

	moduleRefs := AttachRefs(out, declaredBy, content, bindings, sources, importSpans, spans)
	if mu := ModuleUnit(module, filePath, moduleRefs); mu != nil {
		out = append(out, mu)
	}
	return out
}

// ModuleUnit is the file itself, as the unit that makes the references its
// declarations do not: top-level code, a barrel's re-exported names, a test
// file's calls. Without it a file declaring nothing could never be an edge's
// source, so every barrel, context file and spec lost all of its imports --
// 191 of them in LibreChat alone. Returns nil when the module itself uses
// nothing, so a file is not given a unit only to stand for nothing.
//
// Its id ends in "#" with no name, so it can be a reference's source but
// never a target: nothing imports "the module" by name.
func ModuleUnit(module, filePath string, refs []unit.Ref) *unit.Unit {
	if len(refs) == 0 {
		return nil
	}
	return &unit.Unit{
		ID:    module + "#",
		Kind:  unit.KindModule,
		Name:  path.Base(module),
		Files: []string{filePath},
		Refs:  refs,
	}
}

// AttachRefs works out which unit used which imported name, and returns the
// references the module itself makes: uses outside every declaration.
//
// An import clause says what was taken and from where; the usages say by
// whom. declaredBy is the snippet that declares each unit, index for index
// (nil for a unit with no span of its own); a declaration span belongs to the
// unit whose own declaring name sits inside it. It used to be matched by
// name, which gave every method called render the first render's span.
//
// The usages are found by scanning the file for the name, because nothing in
// a tree-sitter query can ask "where is this identifier referenced" and
// capturing every identifier would put millions of snippets in the database
// to throw almost all of them away. Textual rather than semantic: a name in
// a comment or a string counts, which overstates a little and is far better
// than the alternative of attributing every import to every unit in the file.
func AttachRefs(units []*unit.Unit, declaredBy []*file.Snippet, content []byte, bindings, sources, importSpans, spans []*file.Snippet) []unit.Ref {
	if len(bindings) == 0 {
		return nil
	}
	type span struct {
		begin, end int
		unitIdx    int
	}
	var owned []span
	for _, sp := range spans {
		best, bestOffset := -1, 0
		for i, d := range declaredBy {
			if d == nil || i >= len(units) {
				continue
			}
			off := d.Begin.Offset
			if off < sp.Begin.Offset || off > sp.End.Offset {
				continue
			}
			if best == -1 || off < bestOffset {
				best, bestOffset = i, off
			}
		}
		if best >= 0 {
			owned = append(owned, span{begin: sp.Begin.Offset, end: sp.End.Offset, unitIdx: best})
		}
	}
	// Innermost first, so a method inside a class is preferred over the class.
	sort.SliceStable(owned, func(i, j int) bool {
		return (owned[i].end - owned[i].begin) < (owned[j].end - owned[j].begin)
	})

	var moduleRefs []unit.Ref
	text := string(content)
	for _, b := range bindings {
		source := firstSourceAfter(b, sources, importSpans)
		if source == "" || b.Value == "" {
			continue
		}
		ref := ImportedRef(source, b.Value)
		for _, at := range occurrencesOf(text, b.Value) {
			if insideAny(at, importSpans) {
				continue
			}
			attributed := false
			for _, sp := range owned {
				if at >= sp.begin && at <= sp.end {
					units[sp.unitIdx].Refs = appendRef(units[sp.unitIdx].Refs, ref)
					attributed = true
					break
				}
			}
			if !attributed {
				moduleRefs = appendRef(moduleRefs, ref)
			}
		}
	}
	return moduleRefs
}

// importedName is the unit an import clause takes. A single-file component
// exports one thing, the component, whatever the importer calls it: `import
// Panel from "./ChatPanel.vue"` takes ChatPanel, named for its file the way
// the component's own unit is.
func importedName(source, binding string) string {
	if isSingleFileComponent(source) {
		return DefaultExportName(source)
	}
	return binding
}

// isSingleFileComponent reports whether a path is a Vue or Svelte component,
// whose code sits in script blocks inside markup.
func isSingleFileComponent(p string) bool {
	switch strings.ToLower(path.Ext(p)) {
	case ".vue", ".svelte":
		return true
	}
	return false
}

// ImportBindings maps each name the file's import clauses took to the module
// it came from, as written.
func ImportBindings(res *file.Results) map[string]string {
	var bindings, sources, importSpans []*file.Snippet
	for _, s := range res.Snippets {
		switch s.Type {
		case CaptureJSBinding:
			bindings = append(bindings, s)
		case file.ImportRaw:
			sources = append(sources, s)
		case CaptureJSImportSpan:
			importSpans = append(importSpans, s)
		}
	}
	out := map[string]string{}
	for _, b := range bindings {
		if source := firstSourceAfter(b, sources, importSpans); source != "" && b.Value != "" {
			out[b.Value] = source
		}
	}
	return out
}

// ImportedRef is the reference a use of an imported name makes.
func ImportedRef(source, binding string) unit.Ref {
	return unit.Ref{Module: source, Name: importedName(source, binding)}
}

func appendRef(list []unit.Ref, r unit.Ref) []unit.Ref {
	for _, existing := range list {
		if existing == r {
			return list
		}
	}
	return append(list, r)
}

// The snippet that named a unit, so a declaration span can be matched to it.
func nameSnippetOf(u *unit.Unit, nameSnippets []*file.Snippet) *file.Snippet {
	for _, s := range nameSnippets {
		if s.Value == u.Name {
			return s
		}
	}
	return nil
}

// The module an import clause took its names from: the first source string
// inside the same import statement.
func firstSourceAfter(binding *file.Snippet, sources, importSpans []*file.Snippet) string {
	for _, span := range importSpans {
		if binding.Begin.Offset < span.Begin.Offset || binding.Begin.Offset > span.End.Offset {
			continue
		}
		for _, src := range sources {
			if src.Begin.Offset >= span.Begin.Offset && src.End.Offset <= span.End.Offset {
				return src.Value
			}
		}
	}
	return ""
}

func insideAny(offset int, spans []*file.Snippet) bool {
	for _, s := range spans {
		if offset >= s.Begin.Offset && offset <= s.End.Offset {
			return true
		}
	}
	return false
}

// Whole-word occurrences of a name. Textual rather than semantic: a name in
// a comment or a string counts, which overstates a little and is far better
// than the alternative of attributing every import to every unit in the file.
func occurrencesOf(text, name string) []int {
	var out []int
	for from := 0; ; {
		idx := strings.Index(text[from:], name)
		if idx < 0 {
			return out
		}
		at := from + idx
		from = at + len(name)
		if at > 0 && isWordByte(text[at-1]) {
			continue
		}
		if end := at + len(name); end < len(text) && isWordByte(text[end]) {
			continue
		}
		out = append(out, at)
	}
}

func isWordByte(b byte) bool {
	return b == '_' || b == '$' || (b >= '0' && b <= '9') || (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}

func byOffset(snippets []*file.Snippet) {
	sort.SliceStable(snippets, func(i, j int) bool {
		return snippets[i].Begin.Offset < snippets[j].Begin.Offset
	})
}

// precedingIndex is the last declaration starting before this snippet.
func precedingIndex(s *file.Snippet, declarations []*file.Snippet) int {
	last := -1
	for i, d := range declarations {
		if d.Begin.Offset < s.Begin.Offset {
			last = i
		}
	}
	return last
}

// followingIndex is the first declaration starting at or after this snippet,
// which is what a decorator sits above.
func followingIndex(s *file.Snippet, declarations []*file.Snippet) int {
	for i, d := range declarations {
		if d.Begin.Offset >= s.Begin.Offset {
			return i
		}
	}
	return -1
}
