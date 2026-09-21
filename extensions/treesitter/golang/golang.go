package golang

import (
	"github.com/archstats/archstats/core"
	"github.com/archstats/archstats/core/file"
	"github.com/archstats/archstats/extensions/treesitter/common"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	golang "github.com/tree-sitter/tree-sitter-go/bindings/go"
)

// Go, where the architecture is mostly not made of types.
//
// gin is 1,344 functions to 178 types, and archstats itself 556 to 106. A
// reading of this language that looks only at types sees a seventh of it,
// which is why units carry a kind rather than being classes by another name.
//
// Three things here have no analogue in the languages that came first:
//
//   - A method belongs to its receiver and is written outside it. `func (s
//     *Server) ServeHTTP` is Server's, and nothing in the file nests it
//     inside the type.
//   - A struct embeds rather than extends. gin's Engine embeds RouterGroup;
//     no import and no `implements` says so.
//   - There are no annotations at all. What Go has instead is struct tags --
//     150 `form:` tags in gin -- and `//go:` comment directives.

type Extension struct {
}

func (e *Extension) Init(settings core.Analyzer) error {
	settings.RegisterFileAnalyzer(&goAnalyzer{lp: createGoLanguagePack()})
	return nil
}

// Captures used to build units. They never become stats, and the analyzer
// consumes them by position, so they are named rather than underscored.
const (
	captureFunc      = "go__func__declaration"
	captureMethod    = "go__method__declaration"
	captureReceiver  = "go__method__receiver"
	captureTypeName  = "go__type__declaration"
	captureInterface = "go__interface__declaration"
	captureEmbeds    = "go__struct__embeds"
	captureTag       = "go__struct__tag"
	captureDirective = "go__directive"
)

func createGoLanguagePack() *common.LanguagePack {
	language := tree_sitter.NewLanguage(golang.Language())
	template := &common.LanguagePackTemplate{
		// `**.go` rather than `**/*.go`: the second needs a separator, and
		// Java's pattern shows the shape that matches a file wherever it is.
		FileGlob: "**.go",
		Language: language,
		QueriesForStats: []string{
			// The import path, without its quotes: the content node exists
			// precisely so nobody has to strip them afterwards.
			//
			// Go names a package by its last element only -- `package core`
			// says nothing about where it sits -- so there is deliberately
			// no component declaration here, and components come from the
			// directory tree, which is what an import path already points at.
			`(import_spec path: (interpreted_string_literal
				(interpreted_string_literal_content) @modularity__component__imports))`,

			`(type_declaration (type_spec name: (type_identifier) @modularity__types__total))`,
			// An interface is Go's only abstract type. A struct with no
			// methods is still concrete.
			`(type_declaration (type_spec
				name: (type_identifier) @modularity__types__abstract
				type: (interface_type)))`,
		},
		QueriesForSnippets: []string{
			`(type_declaration (type_spec name: (type_identifier) @` + captureTypeName + `))`,
			`(type_declaration (type_spec
				name: (type_identifier) @` + captureInterface + `
				type: (interface_type)))`,

			// A function standing on its own, which in Go is most of them.
			`(function_declaration name: (identifier) @` + captureFunc + `)`,

			// A method and the type it belongs to. Both captures come from
			// one match, but each arrives as its own snippet, so the
			// analyzer pairs them by position.
			`(method_declaration
				receiver: (parameter_list (parameter_declaration
					type: [
						(type_identifier) @` + captureReceiver + `
						(pointer_type (type_identifier) @` + captureReceiver + `)
					]))
				name: (field_identifier) @` + captureMethod + `)`,

			// Embedding: a field with a type and no name. This is how Go
			// composes, and it is the nearest thing it has to `extends`.
			// gin's Engine embeds RouterGroup exactly this way.
			`(field_declaration !name type: [
				(type_identifier) @` + captureEmbeds + `
				(pointer_type (type_identifier) @` + captureEmbeds + `)
				(qualified_type name: (type_identifier) @` + captureEmbeds + `)
				(pointer_type (qualified_type name: (type_identifier) @` + captureEmbeds + `))
			])`,

			// Struct tags and comment directives: what Go has instead of
			// annotations. gin carries 150 `form:` tags; archstats has 15
			// `//go:embed`.
			`(field_declaration tag: (raw_string_literal) @` + captureTag + `)`,
			`((comment) @` + captureDirective + ` (#match? @` + captureDirective + ` "^//go:"))`,
		},
	}

	pack, err := common.PackFromTemplate(template)
	if err != nil {
		panic(err)
	}
	return pack
}

type goAnalyzer struct {
	lp *common.LanguagePack
}

func (a *goAnalyzer) AnalyzeFile(f file.File) *file.Results {
	res := a.lp.AnalyzeFile(f)
	if res == nil {
		return nil
	}
	res.Units = unitsFrom(f.Path(), res)
	return res
}
