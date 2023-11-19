package java

import (
	"github.com/archstats/archstats/core"
	"github.com/archstats/archstats/extensions/treesitter/common"
	"github.com/smacker/go-tree-sitter/java"
)

type Extension struct {
}

func (e *Extension) Init(settings core.Analyzer) error {
	settings.RegisterFileAnalyzer(createJavaLanguagePack())
	return nil
}

func createJavaLanguagePack() *common.LanguagePack {
	lp := &common.LanguagePackTemplate{
		FileGlob: "**/*.java",
		Language: java.GetLanguage(),
		Queries: []string{
			`(package_declaration  (scoped_identifier) @modularity__component__declarations)`,
			`((import_declaration 
				[(
					 (scoped_identifier) (asterisk)) @modularity__component__declarations 
				  (scoped_identifier scope: (scoped_identifier) @modularity__component__declarations) 
					  (#match? @modularity__component__declarations  \"^[a-z0-9.]+$\") ]))`,
		},
	}
	template, err := common.PackFromTemplate(lp)
	if err != nil {
		panic(err)
	}
	return template
}
