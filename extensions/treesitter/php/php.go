package php

import (
	"strings"

	"github.com/archstats/archstats/core"
	"github.com/archstats/archstats/core/file"
	"github.com/archstats/archstats/extensions/treesitter/common"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	tsphp "github.com/tree-sitter/tree-sitter-php/bindings/go"
)

// PHP, read with tree-sitter. It used to be read by four regular
// expressions, which saw a namespace, a `use` line and the word class --
// no units, no references, nothing a Laravel or Symfony profile could
// classify -- so a PHP codebase had components and nothing inside them.
type Extension struct{}

func (e *Extension) Init(settings core.Analyzer) error {
	settings.RegisterFileAnalyzer(&phpAnalyzer{lp: createPHPLanguagePack()})
	return nil
}

// The capture for a `use` clause naming a class: on its way to becoming the
// namespace that holds it, which is what a component is.
const usedClass = "php__use__class"

func createPHPLanguagePack() *common.LanguagePack {
	template := &common.LanguagePackTemplate{
		FileGlob: "**.php",
		// The grammar that reads PHP inside HTML as well as pure PHP:
		// templates carry `<?php ... ?>` islands, and a pure-PHP grammar
		// reads everything before the first tag as an error.
		Language: tree_sitter.NewLanguage(tsphp.LanguagePHP()),
		QueriesForStats: []string{
			`(namespace_definition name: (namespace_name) @modularity__component__declarations)`,

			// What a file imports. `use Acme\Order\Model\Order;` depends on
			// the namespace Acme\Order\Model; a group use,
			// `use Acme\Core\{Clock, Money};`, names that namespace itself.
			`(namespace_use_clause (qualified_name) @modularity__import__raw)`,
			`(namespace_use_clause (qualified_name) @` + usedClass + `)`,
			`(namespace_use_declaration (namespace_name) @modularity__import__raw body: (namespace_use_group))`,
			`(namespace_use_declaration (namespace_name) @modularity__component__imports body: (namespace_use_group))`,

			`
((class_declaration name: (name) @modularity__types__total))
((interface_declaration name: (name) @modularity__types__total))
((trait_declaration name: (name) @modularity__types__total))
((enum_declaration name: (name) @modularity__types__total))
`,
			`
((interface_declaration name: (name) @modularity__types__abstract))
((class_declaration (abstract_modifier) name: (name) @modularity__types__abstract))
`,
		},
		QueriesForSnippets: unitQueries(),
		SnippetTransformers: map[string]func(*file.Snippet) *file.Snippet{
			usedClass: toNamespace,
		},
	}
	pack, err := common.PackFromTemplate(template)
	if err != nil {
		panic(err)
	}
	return pack
}

// toNamespace turns an imported class into the namespace that holds it.
func toNamespace(s *file.Snippet) *file.Snippet {
	name := strings.TrimPrefix(s.Value, `\`)
	if i := strings.LastIndex(name, `\`); i > 0 {
		name = name[:i]
	}
	s.Value = name
	s.Type = file.ComponentImport
	return s
}
