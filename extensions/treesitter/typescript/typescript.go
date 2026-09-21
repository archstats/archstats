package typescript

import (
	"github.com/archstats/archstats/core"
	"github.com/archstats/archstats/core/file"
	"github.com/archstats/archstats/extensions/treesitter/common"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	typescript "github.com/tree-sitter/tree-sitter-typescript/bindings/go"
)

type Extension struct {
}

func (e *Extension) Init(settings core.Analyzer) error {
	settings.RegisterFileAnalyzer(createTypeScriptLanguagePack(false)) // regular typescript
	settings.RegisterFileAnalyzer(createTypeScriptLanguagePack(true))  // TSX
	return nil
}

func createTypeScriptLanguagePack(isTsx bool) *common.LanguagePack {
	var language *tree_sitter.Language
	var globPattern string

	if isTsx {
		language = tree_sitter.NewLanguage(typescript.LanguageTSX())
		globPattern = "**/*.tsx"
	} else {
		language = tree_sitter.NewLanguage(typescript.LanguageTypescript())
		globPattern = "**/*.{ts,mts,cts}"
	}

	template := &common.LanguagePackTemplate{
		FileGlob: globPattern,
		Language: language,
		QueriesForStats: []string{
			// Imports, split by whether the compiler keeps them.
			//
			// `import type { Foo } from "./foo"` is erased: it is a real
			// dependency on a shape and none at all once the program runs.
			// LibreChat writes 681 of its 5,100 imports this way, so counting
			// them as coupling inflates its graph by an eighth. The whole
			// statement is matched rather than a grammar node, the same way
			// the Java queries tell a static import from an ordinary one.
			//
			// An inline `import { type Foo, bar }` stays an ordinary import,
			// correctly: the statement still pulls `bar` in at runtime.
			`((import_statement source: (string) @modularity__component__imports__type) @_imp
			  (#match? @_imp "^import[ \t]+type[ \t{*]"))`,
			`((import_statement source: (string) @modularity__component__imports) @_imp
			  (#not-match? @_imp "^import[ \t]+type[ \t{*]"))`,
			`((export_statement source: (string) @modularity__component__imports__type) @_exp
			  (#match? @_exp "^export[ \t]+type[ \t{*]"))`,
			`((export_statement source: (string) @modularity__component__imports) @_exp
			  (#not-match? @_exp "^export[ \t]+type[ \t{*]"))`,
			// CommonJS and dynamic imports. An enormous amount of real
			// JavaScript never writes the word `import`: express, at the
			// commit these tests pin, has 66 `require()` calls and zero ESM
			// imports, so matching only `import_statement` reported it as a
			// codebase with no dependencies at all.
			`((call_expression
				function: (identifier) @_require
				arguments: (arguments (string) @modularity__component__imports))
			  (#eq? @_require "require"))`,
			`(call_expression
				function: (import)
				arguments: (arguments (string) @modularity__component__imports))`,
			// Classes (total types)
			`(class_declaration name: (type_identifier) @modularity__types__total)`,
			`(abstract_class_declaration name: (type_identifier) @modularity__types__total)`,
			// Interfaces (total types & abstract types)
			`(interface_declaration name: (type_identifier) @modularity__types__total)`,
			`(interface_declaration name: (type_identifier) @modularity__types__abstract)`,
			// Abstract class declarations
			`(abstract_class_declaration name: (type_identifier) @modularity__types__abstract)`,
			// React Functional Components: functions starting with uppercase
			`(function_declaration name: (identifier) @ts__react__components (#match? @ts__react__components "^[A-Z]"))`,
			// React Arrow Components
			`(lexical_declaration (variable_declarator name: (identifier) @ts__react__components value: (arrow_function)) (#match? @ts__react__components "^[A-Z]"))`,
			// React Hooks
			`(call_expression function: (identifier) @ts__react__hooks (#match? @ts__react__hooks "^use[A-Z]"))`,
			// Angular Decorators
			`((decorator [ (identifier) @ts__angular__components (call_expression function: (identifier) @ts__angular__components) ]) (#match? @ts__angular__components "^Component$"))`,
			`((decorator [ (identifier) @ts__angular__services (call_expression function: (identifier) @ts__angular__services) ]) (#match? @ts__angular__services "^Injectable$"))`,
			`((decorator [ (identifier) @ts__angular__directives (call_expression function: (identifier) @ts__angular__directives) ]) (#match? @ts__angular__directives "^Directive$"))`,
			`((decorator [ (identifier) @ts__angular__pipes (call_expression function: (identifier) @ts__angular__pipes) ]) (#match? @ts__angular__pipes "^Pipe$"))`,
		},
		SnippetTransformers: map[string]func(*file.Snippet) *file.Snippet{
			file.ComponentImport:         stripQuotes,
			file.ComponentImportTypeOnly: stripQuotes,
		},
	}

	pack, err := common.PackFromTemplate(template)
	if err != nil {
		panic(err)
	}
	return pack
}

func stripQuotes(s *file.Snippet) *file.Snippet {
	val := s.Value
	if len(val) >= 2 && ((val[0] == '"' && val[len(val)-1] == '"') || (val[0] == '\'' && val[len(val)-1] == '\'') || (val[0] == '`' && val[len(val)-1] == '`')) {
		s.Value = val[1 : len(val)-1]
	}
	return s
}
