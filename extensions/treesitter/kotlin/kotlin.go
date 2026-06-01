package kotlin

import (
	"github.com/archstats/archstats/core"
	"github.com/archstats/archstats/extensions/treesitter/common"
	kotlin "github.com/fwcd/tree-sitter-kotlin/bindings/go"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

type Extension struct {
}

func (e *Extension) Init(settings core.Analyzer) error {
	settings.RegisterFileAnalyzer(createKotlinLanguagePack())
	return nil
}

func createKotlinLanguagePack() *common.LanguagePack {
	language := tree_sitter.NewLanguage(kotlin.Language())
	template := &common.LanguagePackTemplate{
		FileGlob: "**.kt",
		Language: language,
		QueriesForStats: []string{
			`(package_header (identifier) @modularity__component__declarations)`,
			`
(import_header
	(identifier (((simple_identifier) ("." (simple_identifier))*))@modularity__component__imports )  @_import_no_wildcard .
)
`,
			`
((class_declaration (type_identifier) @modularity__types__total))
((object_declaration (type_identifier) @modularity__types__total))
`,
			`
((class_declaration
  (modifiers) @_mods
  (type_identifier) @modularity__types__abstract)
  (#match? @_mods "abstract"))
`,
		},
	}

	pack, err := common.PackFromTemplate(template)
	if err != nil {
		panic(err)
	}
	return pack
}
