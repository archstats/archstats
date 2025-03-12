package kotlin

import (
	"github.com/archstats/archstats/extensions/treesitter/common"
	kotlin "github.com/fwcd/tree-sitter-kotlin/bindings/go"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

func createKotlinPack() *common.LanguagePack {
	language := tree_sitter.NewLanguage(kotlin.Language())
	template := &common.LanguagePackTemplate{
		FileGlob: "**.kt",
		Language: language,
		QueriesForStats: []string{
			`(import_header
	(identifier (((simple_identifier) ("\." (simple_identifier))*))@import )  @_import_no_wildcard .
)


`,
		},
	}

	pack, err := common.PackFromTemplate(template)
	if err != nil {
		// print error panic
		panic(err)
	}
	return pack
}
