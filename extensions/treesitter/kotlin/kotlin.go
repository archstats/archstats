package kotlin

import (
	"github.com/archstats/archstats/extensions/treesitter/common"
	"github.com/smacker/go-tree-sitter/kotlin"
)

func createKotlinPack() *common.LanguagePack {
	template := &common.LanguagePackTemplate{
		FileGlob: "*.kt",
		Language: kotlin.GetLanguage(),
		Queries: []string{
			`(import_header 
	(identifier
		((((simple_identifier) ".")+ @modularity__component__imports)) ) )`,

			`(package_header (identifier) @modularity__component__declarations)`,
		},
	}

	pack, err := common.PackFromTemplate(template)
	if err != nil {
		panic(err)
	}
	return pack
}
