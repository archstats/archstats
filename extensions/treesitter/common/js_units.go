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
)

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

		// A method belongs to the class above it, paired up by position.
		`(method_definition name: (property_identifier) @` + CaptureJSMethod + `)`,

		// The only thing in this family that looks like an annotation, and
		// it exists only in Angular and NestJS.
		`(decorator [
			(identifier) @` + CaptureJSDecorator + `
			(call_expression function: (identifier) @` + CaptureJSDecorator + `)
		])`,
	}
	if withTypes {
		queries = append(queries,
			`(abstract_class_declaration name: (`+typeIdentifier+`) @`+CaptureJSClass+`)`,
			`(interface_declaration name: (`+typeIdentifier+`) @`+CaptureJSInterface+`)`,
		)
	}
	return queries
}

// ModuleOf is the module a file is, which is its path without the extension.
// `client/src/utils/rules.ts` is the module `client/src/utils/rules`, and an
// index file is named by its directory the way an import writes it.
func ModuleOf(filePath string) string {
	p := path.Clean(strings.TrimPrefix(filePath, "./"))
	ext := path.Ext(p)
	p = strings.TrimSuffix(p, ext)
	if base := path.Base(p); base == "index" {
		if dir := path.Dir(p); dir != "." && dir != "/" {
			return dir
		}
	}
	return p
}

// JSUnitsFrom builds units from what the shared queries captured.
func JSUnitsFrom(filePath string, res *file.Results) []*unit.Unit {
	module := ModuleOf(filePath)

	var classes, interfaces, functions, methods, decorators []*file.Snippet
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
		}
	}
	byOffset(classes)
	byOffset(methods)

	var out []*unit.Unit
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
	}
	for _, i := range interfaces {
		out = append(out, &unit.Unit{
			ID:      module + "#" + i.Value,
			Kind:    unit.KindType,
			Name:    i.Value,
			Files:   []string{filePath},
			Markers: []unit.Marker{{Source: unit.SourceSupertype, Key: "interface"}},
		})
	}
	for _, m := range methods {
		owner := ""
		name := m.Value
		if idx := precedingIndex(m, classes); idx >= 0 {
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
	}
	for _, f := range functions {
		// A function declared inside a class body is a method and has
		// already been recorded as one.
		out = append(out, &unit.Unit{
			ID:    module + "#" + f.Value,
			Kind:  unit.KindFunction,
			Name:  f.Value,
			Files: []string{filePath},
		})
	}
	return out
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
