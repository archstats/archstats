package javascript

import (
	"github.com/archstats/archstats/core"
	"github.com/archstats/archstats/core/file"
	"github.com/archstats/archstats/extensions/treesitter/common"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	javascript "github.com/tree-sitter/tree-sitter-javascript/bindings/go"
)

type Extension struct {
}

func (e *Extension) Init(settings core.Analyzer) error {
	settings.RegisterFileAnalyzer(createJavaScriptLanguagePack())
	return nil
}

func createJavaScriptLanguagePack() *common.LanguagePack {
	language := tree_sitter.NewLanguage(javascript.Language())
	template := &common.LanguagePackTemplate{
		FileGlob: "**/*.{js,jsx,mjs,cjs}",
		Language: language,
		QueriesForStats: []string{
			// Imports: capture string inside import/export statements
			`(import_statement source: (string) @modularity__component__imports)`,
			`(export_statement source: (string) @modularity__component__imports)`,
			// Classes (total types)
			`(class_declaration name: (identifier) @modularity__types__total)`,
			// React Functional Components: functions starting with uppercase
			`(function_declaration name: (identifier) @js__react__components (#match? @js__react__components "^[A-Z]"))`,
			// React Arrow Components: const Title = () => JSX
			`(lexical_declaration (variable_declarator name: (identifier) @js__react__components value: (arrow_function)) (#match? @js__react__components "^[A-Z]"))`,
			// React Hooks: useQuery, useEffect, etc. (call expressions starting with "use" followed by uppercase)
			`(call_expression function: (identifier) @js__react__hooks (#match? @js__react__hooks "^use[A-Z]"))`,
		},
		SnippetTransformers: map[string]func(*file.Snippet) *file.Snippet{
			"modularity__component__imports": stripQuotes,
		},
	}

	pack, err := common.PackFromTemplate(template)
	if err != nil {
		panic(err)
	}
	return pack
}

func stripQuotes(s *file.Snippet) *file.Snippet {
	val := s.Value
	if len(val) >= 2 && ((val[0] == '"' && val[len(val)-1] == '"') || (val[0] == '\'' && val[len(val)-1] == '\'') || (val[0] == '`' && val[len(val)-1] == '`')) {
		s.Value = val[1 : len(val)-1]
	}
	return s
}
