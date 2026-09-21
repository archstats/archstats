package python

import (
	"github.com/archstats/archstats/core"
	"github.com/archstats/archstats/extensions/treesitter/common"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	python "github.com/tree-sitter/tree-sitter-python/bindings/go"
)

type Extension struct {
}

func (e *Extension) Init(settings core.Analyzer) error {
	settings.RegisterFileAnalyzer(createPythonLanguagePack())
	return nil
}

func createPythonLanguagePack() *common.LanguagePack {
	language := tree_sitter.NewLanguage(python.Language())
	template := &common.LanguagePackTemplate{
		FileGlob: "**/*.py",
		Language: language,
		QueriesForStats: []string{
			// Imports: capture dotted name or relative import
			`(import_statement name: (dotted_name) @modularity__component__imports)`,
			// `import acme.shared.log as slog`. The alias wraps the name, so
			// a query for a bare dotted_name under import_statement walks
			// straight past it, and every aliased import in a codebase went
			// unseen.
			`(import_statement name: (aliased_import name: (dotted_name) @modularity__component__imports))`,
			`(import_from_statement module_name: (dotted_name) @modularity__component__imports)`,
			`(import_from_statement module_name: (relative_import) @modularity__component__imports)`,
			// Dependencies named by a string, resolved when the program runs.
			//
			// django-oscar's entire extensibility model is `get_class`: every
			// app can be forked and overridden, so nothing may import another
			// app's classes directly. 746 of its dependency edges are written
			// this way against 1,645 static imports. An import graph that
			// reads only `import` statements is missing a third of that
			// codebase and reports no error.
			//
			// The first string argument names a module -- "catalogue.views",
			// "order.models" -- which is what the component linker already
			// knows how to resolve, so these join the ordinary imports rather
			// than needing a resolver of their own.
			`((call
				function: [(identifier) @_fn (attribute attribute: (identifier) @_fn)]
				arguments: (argument_list . (string (string_content) @modularity__component__imports__dynamic)))
			  (#match? @_fn "^(get_class|get_classes|get_model|import_module)$"))`,
			// Classes (total types)
			`(class_definition name: (identifier) @modularity__types__total)`,
			// Web decorator routes (FastAPI, Flask, Django)
			`((decorator (call function: (attribute attribute: (identifier) @python__web__routes))) (#match? @python__web__routes "^(get|post|put|delete|route)$"))`,
			`((decorator (call function: (identifier) @python__web__routes)) (#match? @python__web__routes "^(get|post|put|delete|route)$"))`,
			`((decorator (identifier) @python__web__routes) (#match? @python__web__routes "^(get|post|put|delete|route)$"))`,
		},
	}

	pack, err := common.PackFromTemplate(template)
	if err != nil {
		panic(err)
	}
	return pack
}
