package java

import (
	"github.com/archstats/archstats/core"
	"github.com/archstats/archstats/extensions/treesitter/common"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	java "github.com/tree-sitter/tree-sitter-java/bindings/go"
)

type Extension struct {
}

func (e *Extension) Init(settings core.Analyzer) error {
	settings.RegisterFileAnalyzer(createJavaLanguagePack())
	return nil
}

func createJavaLanguagePack() *common.LanguagePack {

	language := tree_sitter.NewLanguage(java.Language())

	lp := &common.LanguagePackTemplate{
		FileGlob: "**.java",
		Language: language,
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
    ((scoped_identifier scope: (scoped_identifier) @modularity__component__imports))) @_import ) 
    (#not-match? @_import "(^import static)|[*]")
)
(
  ((import_declaration 
    ((scoped_identifier) @modularity__component__imports) (asterisk)) @_import)
    (#not-match? @_import "(^import static)")
    (#match? @_import "[*]")
)
(
  ((import_declaration
    ((scoped_identifier scope: (scoped_identifier) @modularity__component__imports))) @_import ) 
      (#match? @_import "(^import static)")
      (#not-match? @_import "[*]")
)
(
  ((import_declaration
      ((scoped_identifier) @modularity__component__imports) (asterisk)) @_import)
      (#match? @_import "(^import static)")
      (#match? @_import "[*]")
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
