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
	settings.RegisterFileAnalyzer(&csharpAnalyzer{lp: createCSharpLanguagePack()})
	return nil
}
func createCSharpLanguagePack() *common.LanguagePack {
	language := tree_sitter.NewLanguage(csharp.Language())
	lp := &common.LanguagePackTemplate{
		FileGlob: "**.cs",
		Language: language,
		QueriesForStats: []string{
			// Both ways C# spells a namespace. The file-scoped form —
			// `namespace Nop.Services.Catalog;` with no block — is a
			// different node, not a variation of the first, and it is what
			// every .NET 6+ project template emits. Matching only the block
			// form filed a whole modern codebase under "Unknown".
			`(namespace_declaration
				 name: ([(qualified_name) (identifier)]) @modularity__component__declarations)
			 (file_scoped_namespace_declaration
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
; An enum is a type. Leaving it out undercounted nopCommerce by 143 types
; of 3,923 and, because abstractness is abstract over total, overstated
; abstractness everywhere enums are common.
((enum_declaration name: (identifier) @modularity__types__total))
`,

			// Attributes and base types, as neutral evidence about a type.
			// C# puts almost nothing architectural in its attributes --
			// nopCommerce's most common are NopResourceDisplayName (2,609)
			// and JsonProperty (1,064), which are display and serialization
			// -- but what is there is worth recording, and the base list is
			// where ASP.NET and EF actually say what something is.
			`(attribute_list (attribute name: [(identifier) (qualified_name)] @csharp__class__attribute))`,
			`(base_list [
				(identifier) @csharp__class__base
				(generic_name (identifier) @csharp__class__base)
				(qualified_name (identifier) @csharp__class__base)
			])`,
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
