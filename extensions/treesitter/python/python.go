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
			`(import_from_statement module_name: (dotted_name) @modularity__component__imports)`,
			`(import_from_statement module_name: (relative_import) @modularity__component__imports)`,
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
