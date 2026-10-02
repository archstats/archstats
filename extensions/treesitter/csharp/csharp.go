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
		FileGlob:   "**.cs",
		Complexity: complexity,
		Language:   language,
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

			// Where a type is named rather than declared. C# imports a
			// namespace rather than a type -- `using Acme.Core` says nothing
			// about which of its types are used -- so the usages have to be
			// read from the places a type can appear. Narrow on purpose: a
			// base list, a field or parameter type, a return type and a
			// `new`. Capturing every identifier would name every local
			// variable too.
			`(base_list (identifier) @csharp__type__use)`,
			`(variable_declaration type: (identifier) @csharp__type__use)`,
			`(parameter type: (identifier) @csharp__type__use)`,
			`(method_declaration returns: (identifier) @csharp__type__use)`,
			`(object_creation_expression type: (identifier) @csharp__type__use)`,
			// Those five saw 55% of the dependencies the source names. The
			// rest were a static class, an enum or a constant reached through
			// a dot -- nopCommerce names StandardPermission in 129 files and
			// had no edge to it -- a generic argument (`IRepository<Product>`
			// in every service constructor), a property type, and the nullable,
			// array, typeof, cast and pattern forms. A name that is not a type
			// of this codebase in a namespace the file can see resolves to
			// nothing, so a local variable caught here costs nothing.
			`(member_access_expression expression: (identifier) @csharp__type__use)`,
			`(generic_name (identifier) @csharp__type__use)`,
			`(type_argument_list (identifier) @csharp__type__use)`,
			`(property_declaration type: (identifier) @csharp__type__use)`,
			`(nullable_type (identifier) @csharp__type__use)`,
			`(array_type (identifier) @csharp__type__use)`,
			`(typeof_expression (identifier) @csharp__type__use)`,
			`(cast_expression type: (identifier) @csharp__type__use)`,
			`(as_expression right: (identifier) @csharp__type__use)`,
			`(declaration_pattern type: (identifier) @csharp__type__use)`,
			`(type_pattern (identifier) @csharp__type__use)`,
			`(default_expression (identifier) @csharp__type__use)`,
			`(class_declaration) @csharp__declaration__span`,
			`(struct_declaration) @csharp__declaration__span`,
			`(interface_declaration) @csharp__declaration__span`,
			`(record_declaration) @csharp__declaration__span`,
			`(enum_declaration) @csharp__declaration__span`,
			// The last segment of a qualified base: `: Mvc.Controller` is a
			// Controller, and matching every identifier in it made it an
			// `Mvc` as well.
			`(base_list [
				(identifier) @csharp__class__base
				(generic_name (identifier) @csharp__class__base)
				(qualified_name name: (identifier) @csharp__class__base)
				(qualified_name name: (generic_name (identifier) @csharp__class__base))
			])`,
			// What a type was declared as. Folded into a marker by the
			// analyzer and not stored.
			`(record_declaration name: (identifier) @csharp__record__name)`,
			`(struct_declaration name: (identifier) @csharp__struct__name)`,
			`(enum_declaration name: (identifier) @csharp__enum__name)`,
			`(interface_declaration name: (identifier) @csharp__interface__name)`,
			`
(using_directive (qualified_name) @modularity__import__raw)
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
