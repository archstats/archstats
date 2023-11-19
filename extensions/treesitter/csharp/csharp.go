package csharp

import (
	"github.com/archstats/archstats/extensions/treesitter/common"
	"github.com/smacker/go-tree-sitter/csharp"
)

func createCSharpLanguagePack() *common.LanguagePack {
	lp := &common.LanguagePackTemplate{
		FileGlob: "",
		Language: csharp.GetLanguage(),
		Queries: []string{
			`(namespace_declaration 
				 name: (qualified_name) @modularity__component__declarations)`,
			`(using_directive [(qualified_name) (identifier)] @modularity__component__imports)`,
		},
	}
	template, err := common.PackFromTemplate(lp)
	if err != nil {
		panic(err)
	}
	return template
}
