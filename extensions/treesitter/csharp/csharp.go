package csharp

import (
	"github.com/archstats/archstats/core"
	"github.com/archstats/archstats/extensions/treesitter/common"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	csharp "github.com/tree-sitter/tree-sitter-c-sharp/bindings/go"
)

type Extension struct {
}

func (e *Extension) Init(settings core.Analyzer) error {
	settings.RegisterFileAnalyzer(createCSharpLanguagePack())
	return nil
}
func createCSharpLanguagePack() *common.LanguagePack {
	language := tree_sitter.NewLanguage(csharp.Language())
	lp := &common.LanguagePackTemplate{
		FileGlob: "**.cs",
		Language: language,
		QueriesForStats: []string{
			`(namespace_declaration
				 name: ([(qualified_name) (identifier)]) @modularity__component__declarations)`,
			`
((interface_declaration name: (identifier) @modularity__types__abstract))
((class_declaration (((modifier)@_mod ) (#match? @_mod "abstract" )) name: (identifier) @modularity__types__abstract))
`,
			`
((class_declaration name: (identifier) @modularity__types__total))
((struct_declaration name: (identifier) @modularity__types__total))
((interface_declaration name: (identifier) @modularity__types__total))
((record_declaration name: (identifier) @modularity__types__total))
`,

			`
(using_directive (qualified_name) @modularity__component__imports)
(using_directive (identifier)  @modularity__component__imports !name)
(using_directive name: (identifier) (identifier) @modularity__component__imports)`,
		},
	}
	template, err := common.PackFromTemplate(lp)
	if err != nil {
		panic(err)
	}
	return template
}
