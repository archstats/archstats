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
		FileGlob: "**.java",
		Language: java.GetLanguage(),
		Queries: []string{
			`(package_declaration  (scoped_identifier) @modularity__component__declarations)`,
			`
((interface_declaration name: (identifier) @modularity__types__total))
((class_declaration  name: (identifier) @modularity__types__total))
((record_declaration name: (identifier) @modularity__types__total))
`,
			`
((interface_declaration name: (identifier) @modularity__types__abstract))
((class_declaration ((modifiers) @_modifiers) name: (identifier) @modularity__types__abstract) (#match? @_modifiers "abstract"))
`,
			`
(
  ((import_declaration 
    ((scoped_identifier scope: (scoped_identifier) @modularity__component__imports))) @import ) 
    (#not-match? @import "(^import static)|[*]")
)
(
  ((import_declaration 
    ((scoped_identifier) @modularity__component__imports) (asterisk)) @import)
    (#not-match? @import "(^import static)")
    (#match? @import "[*]")
)
(
  ((import_declaration
    ((scoped_identifier scope: (scoped_identifier) @modularity__component__imports))) @import ) 
      (#match? @import "(^import static)")
      (#not-match? @import "[*]")
)
(
  ((import_declaration
      ((scoped_identifier) @modularity__component__imports) (asterisk)) @import)
      (#match? @import "(^import static)")
      (#match? @import "[*]")
)
`,
		},
	}
	template, err := common.PackFromTemplate(lp)
	if err != nil {
		panic(err)
	}
	return template
}
