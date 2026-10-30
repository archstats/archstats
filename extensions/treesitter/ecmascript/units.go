package ecmascript

import (
	"path"
	"sort"
	"strings"

	"github.com/archstats/archstats/core/file"
	"github.com/archstats/archstats/core/unit"
	"github.com/archstats/archstats/extensions/treesitter/common"
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
	// What a class extends or implements, by its last name: `Repository` in
	// `extends Repository<User>`, `Component` in `extends React.Component`.
	// Every other pack records these, and a profile reads them before
	// anything else.
	CaptureJSSupertype = "js__class__supertype"
	// The shapes TypeScript declares besides an interface. A `type` alias
	// is at least as common as an interface in a models file, and without
	// it the models layer of a React or Vue app was mostly missing.
	CaptureJSTypeAlias = "js__type__alias"
	CaptureJSEnum      = "js__enum__declaration"
	// The call a function is written inside of: `forwardRef` in `const Input
	// = forwardRef((props, ref) => …)`. The function is the unit, the
	// wrapper its marker.
	CaptureJSWrapper = "js__wrapper__call"
)

// jsFactories make the things an architect names that are neither a class nor
// a function: a Pinia store is `export const useCartStore = defineStore(...)`,
// a Redux Toolkit slice `const cartSlice = createSlice(...)`. Their actions
// are methods of an object literal or functions inside the call, and without
// a unit to hold them they were loose functions of the file, so a store read
// as its interfaces and nothing else. An allowlist, not every `define…`:
// Vue's `const props = defineProps()` is a compiler macro, not a unit.
const jsFactories = `^(defineStore|defineComponent|createSlice|createApi)$`

// jsWrappers are the calls a function is written inside of and still is that
// function: `const Input = forwardRef((props, ref) => …)` and `const Row =
// memo(function Row() {…})` are React components, `observer(() => …)` a MobX
// one, `createAsyncThunk('users/fetch', async () => …)` a Redux thunk whose
// body is where the client is called, `createSelector(…, (s) => …)` a
// selector. An allowlist again: `const doubled = computed(() => …)` is a
// value.
const jsWrappers = `^(forwardRef|memo|observer|createAsyncThunk|createSelector)$`

// UnitQueries are the snippet queries for a JavaScript-family grammar.
//
// `typeIdentifier` differs between them: TypeScript names a class with a
// type_identifier and JavaScript with a plain identifier, and a query naming
// the wrong node compiles to nothing rather than failing.
func UnitQueries(typeIdentifier string, withTypes bool) []string {
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

		// A function written inside a wrapper call: `forwardRef((p, ref) =>
		// …)`, `React.memo(function Row() {})`.
		`((lexical_declaration (variable_declarator
			name: (identifier) @` + CaptureJSFunction + `
			value: (call_expression
				function: [(identifier) @` + CaptureJSWrapper + ` (member_expression property: (property_identifier) @` + CaptureJSWrapper + `)]
				arguments: (arguments [(arrow_function) (function_expression)]))))
		  (#match? @` + CaptureJSWrapper + ` "` + jsWrappers + `"))`,
		`((lexical_declaration (variable_declarator
			value: (call_expression
				function: [(identifier) @_wrapper2 (member_expression property: (property_identifier) @_wrapper2)]
				arguments: (arguments [(arrow_function) (function_expression)])))) @` + CaptureJSSpan + `
		  (#match? @_wrapper2 "` + jsWrappers + `"))`,

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
		// The decorators of an exported class hang off the export statement,
		// not the class, and what they name is the class's own use: a NestJS
		// `@Module({ providers: [UsersService] })` is the wiring an architect
		// wants to see from the module, and it used to be the file's.
		`(export_statement (decorator) declaration: (class_declaration)) @` + CaptureJSSpan,
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
			`(abstract_class_declaration) @`+CaptureJSSpan,
			`(export_statement (decorator) declaration: (abstract_class_declaration)) @`+CaptureJSSpan,
			`(interface_declaration name: (`+typeIdentifier+`) @`+CaptureJSInterface+`)`,
			`(interface_declaration) @`+CaptureJSSpan,
			`(type_alias_declaration name: (`+typeIdentifier+`) @`+CaptureJSTypeAlias+`)`,
			`(type_alias_declaration) @`+CaptureJSSpan,
			`(enum_declaration name: (identifier) @`+CaptureJSEnum+`)`,
			`(enum_declaration) @`+CaptureJSSpan,
			// TypeScript spells the heritage out in clauses; `extends
			// Repository<User>` names Repository, `implements OnInit` OnInit.
			`(class_heritage (extends_clause value: [
				(identifier) @`+CaptureJSSupertype+`
				(member_expression property: (property_identifier) @`+CaptureJSSupertype+`)
			]))`,
			`(class_heritage (implements_clause [
				(type_identifier) @`+CaptureJSSupertype+`
				(generic_type name: (type_identifier) @`+CaptureJSSupertype+`)
			]))`,
		)
	} else {
		// JavaScript's heritage is the bare expression after `extends`.
		queries = append(queries,
			`(class_heritage [
				(identifier) @`+CaptureJSSupertype+`
				(member_expression property: (property_identifier) @`+CaptureJSSupertype+`)
			])`,
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

// UnitsFrom builds units from what the shared queries captured.
//
// content is the file, used to find where an imported name is actually used.
// Nothing in a tree-sitter query can ask "where is this identifier
// referenced", and capturing every identifier in a codebase would put
// millions of snippets in the database to throw almost all of them away, so
// the usages are found by scanning for the name and attributed to whichever
// declaration's span contains them.
func UnitsFrom(filePath string, content []byte, res *file.Results) []*unit.Unit {
	module := ModuleOf(filePath)

	var classes, shapes, functions, methods, decorators, factories, supertypes, wrappers []*file.Snippet
	var bindings, importSpans, spans, sources, classSpans []*file.Snippet
	for _, s := range res.Snippets {
		switch s.Type {
		case CaptureJSClass:
			classes = append(classes, s)
		case CaptureJSInterface, CaptureJSTypeAlias, CaptureJSEnum:
			shapes = append(shapes, s)
		case CaptureJSSupertype:
			supertypes = append(supertypes, s)
		case CaptureJSWrapper:
			wrappers = append(wrappers, s)
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
	byOffset(functions)

	var out []*unit.Unit
	// The snippet declaring each unit, index for index with out.
	var declaredBy []*file.Snippet
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
	for i, c := range classes {
		u := &unit.Unit{
			ID:    module + "#" + c.Value,
			Kind:  unit.KindType,
			Name:  c.Value,
			Files: []string{filePath},
		}
		// A decorator sits above the class it is about. One inside a class
		// body, after the class's own name, is about a member -- `@Get()`
		// on a NestJS handler, `@IsString()` on a DTO field, `@Input()` on
		// an Angular property -- and used to land on the next class in the
		// file, so UpdateUserDto carried CreateUserDto's validators.
		for _, d := range decorators {
			if idx := classAt(d.Begin.Offset); idx >= 0 && d.Begin.Offset > classes[idx].Begin.Offset {
				continue
			}
			if followingIndex(d, classes) == i {
				u.Markers = append(u.Markers, unit.Marker{Source: unit.SourceAnnotation, Key: d.Value})
			}
		}
		out = append(out, u)
		declaredBy = append(declaredBy, c)
	}
	// A factory call follows the name it is assigned to.
	for _, f := range factories {
		if i := precedingIndex(f, classes); i >= 0 {
			out[i].Markers = append(out[i].Markers, unit.Marker{Source: unit.SourceSupertype, Key: f.Value})
		}
	}
	// The heritage sits inside the class that declares it.
	for _, s := range supertypes {
		if i := classAt(s.Begin.Offset); i >= 0 {
			out[i].Markers = append(out[i].Markers, unit.Marker{Source: unit.SourceSupertype, Key: s.Value})
		}
	}
	for _, s := range shapes {
		out = append(out, &unit.Unit{
			ID:      module + "#" + s.Value,
			Kind:    unit.KindType,
			Name:    s.Value,
			Files:   []string{filePath},
			Markers: []unit.Marker{{Source: unit.SourceSupertype, Key: shapeKind(s.Type)}},
		})
		declaredBy = append(declaredBy, s)
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
	// A wrapper call follows the name it is assigned to.
	wrapped := map[int][]unit.Marker{}
	for _, w := range wrappers {
		if i := precedingIndex(w, functions); i >= 0 {
			wrapped[i] = append(wrapped[i], unit.Marker{Source: unit.SourceSupertype, Key: w.Value})
		}
	}
	for i, f := range functions {
		out = append(out, &unit.Unit{
			ID:      module + "#" + f.Value,
			Kind:    unit.KindFunction,
			Name:    f.Value,
			Files:   []string{filePath},
			Markers: wrapped[i],
		})
		declaredBy = append(declaredBy, f)
	}
	ownNested(out, declaredBy, spans)

	moduleRefs := common.AttachRefs(out, declaredBy, content, bindings, sources, importSpans, spans, ImportedRef)
	if mu := common.ModuleUnit(module, filePath, moduleRefs); mu != nil {
		out = append(out, mu)
	}
	return out
}

// shapeKind is the marker a declared shape carries, so a profile can read
// "interface", "type_alias" or "enum" the way it reads a supertype.
func shapeKind(captureType string) string {
	switch captureType {
	case CaptureJSTypeAlias:
		return "type_alias"
	case CaptureJSEnum:
		return "enum"
	}
	return "interface"
}

// ownNested gives a function declared inside another declaration to the
// declaration that holds it: a callback inside a React component, a helper
// inside a method, an action inside a setup store's `defineStore(() => …)`.
//
// They used to be loose units of the file, which is what an architect saw:
// a page's `const onSearch = async () => axios.get(…)` stood beside the page
// as a unit of its own, and since it was the one that imported axios it was
// the one the lanes put in data access. Ownership is what the views roll
// members up by, so the callback's uses become the page's.
func ownNested(units []*unit.Unit, declaredBy []*file.Snippet, spans []*file.Snippet) {
	type owned struct {
		begin, end, unitIdx int
	}
	var all []owned
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
			all = append(all, owned{sp.Begin.Offset, sp.End.Offset, best})
		}
	}
	ownerOf := map[int]int{}
	for i, u := range units {
		if u.Owner != "" || u.Kind != unit.KindFunction || i >= len(declaredBy) || declaredBy[i] == nil {
			continue
		}
		at := declaredBy[i].Begin.Offset
		best, bestSize := -1, 0
		for _, o := range all {
			if o.unitIdx == i || at < o.begin || at > o.end {
				continue
			}
			if size := o.end - o.begin; best == -1 || size < bestSize {
				best, bestSize = o.unitIdx, size
			}
		}
		if best >= 0 {
			ownerOf[i] = best
		}
	}
	// An owner's own id may change first, so ids are settled from the
	// outside in.
	var idOf func(i int, depth int) string
	idOf = func(i int, depth int) string {
		if owner, nested := ownerOf[i]; nested && depth < 16 {
			return idOf(owner, depth+1) + "." + units[i].Name
		}
		return units[i].ID
	}
	for i, owner := range ownerOf {
		units[i].Owner = idOf(owner, 0)
		units[i].ID = idOf(i, 0)
	}
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
		if source := common.FirstSourceAfter(b, sources, importSpans); source != "" && b.Value != "" {
			out[b.Value] = source
		}
	}
	return out
}

// ModuleOf is the module a JavaScript or TypeScript file is: its path
// without the extension, and an index file named by its directory the way an
// import writes it.
func ModuleOf(filePath string) string {
	return common.ModuleOf(filePath, "index")
}

// ImportedRef is the reference a use of an imported name makes.
func ImportedRef(source, binding string) unit.Ref {
	ref := unit.Ref{Module: source, Name: importedName(source, binding)}
	p := withoutSourceExtension(source)
	if p != source {
		ref.Path = p
	}
	ref.Unaliased = withoutAliasPrefix(p)
	return ref
}

// withoutAliasPrefix strips a project's own import alias, leaving the path
// the file tree can be searched for. Returns "" when the module carries no
// alias, so a bare package name is never mangled into a false match.
//
// `~/` and `@/` are the two conventions nearly every TypeScript project
// configures; `@scope/pkg` is a published package unless the rest of it
// matches a real directory, which the resolver's suffix search decides
// rather than this.
//
// This is not a detail. LibreChat writes 3,485 of its 7,984 imports this
// way, against 1,766 relative ones: read without it, 81% of its units have
// no edge at all and the graph is empty for the language that needs it most.
func withoutAliasPrefix(module string) string {
	switch {
	case strings.HasPrefix(module, "~/"), strings.HasPrefix(module, "@/"):
		return module[2:]
	case strings.HasPrefix(module, "~"), strings.HasPrefix(module, "@"):
		// `@acme/shared/thing` keeps everything after the scope, because the
		// scope is the publisher and the rest is the path.
		if i := strings.Index(module, "/"); i > 0 && i+1 < len(module) {
			return module[i+1:]
		}
	}
	return ""
}

// withoutSourceExtension drops the file extension an import path spells out,
// because a module is named for its file without one. `./ChatPanel.vue` is
// the only way to import a Vue component, and `./reader.js` is how ESM
// TypeScript must write `./reader`; kept, neither names any module. Only a
// path is touched: a bare package name is not a file. An index file is named
// by its directory, so `./components/index.js` is `./components`.
func withoutSourceExtension(module string) string {
	if !strings.Contains(module, "/") {
		return module
	}
	switch path.Ext(module) {
	case ".vue", ".svelte", ".js", ".jsx", ".mjs", ".cjs", ".ts", ".tsx", ".mts", ".cts":
		module = strings.TrimSuffix(module, path.Ext(module))
		if path.Base(module) == "index" {
			module = path.Dir(module)
		}
	}
	return module
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
