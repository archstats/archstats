package kotlin

import (
	"strings"
	"unicode"

	"github.com/archstats/archstats/core"
	"github.com/archstats/archstats/core/file"
	"github.com/archstats/archstats/extensions/treesitter/common"
	kotlin "github.com/fwcd/tree-sitter-kotlin/bindings/go"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

type Extension struct {
}

func (e *Extension) Init(settings core.Analyzer) error {
	settings.RegisterFileAnalyzer(createKotlinLanguagePack())
	return nil
}

func createKotlinLanguagePack() *common.LanguagePack {
	language := tree_sitter.NewLanguage(kotlin.Language())
	template := &common.LanguagePackTemplate{
		FileGlob: "**.kt",
		Language: language,
		QueriesForStats: []string{
			`(package_header (identifier) @modularity__component__declarations)`,
			// The whole dotted name, in one capture. Written as a sequence of
			// `simple_identifier`s this matched each segment on its own, so
			// `import kotlinx.datetime.internal.JSJoda` arrived as four
			// snippets — "kotlinx", "datetime", "internal", "JSJoda" — none
			// of which is the name of any component. Every Kotlin project
			// analysed as a set of components with no edges between them.
			//
			// Split on the wildcard, the same way the Java queries are: a
			// star import already names the package and keeps every segment,
			// while any other import ends in the member it names — a class, a
			// top-level function, a constant — and that segment is not part
			// of the component.
			`((import_header (identifier) @modularity__component__imports) @_import
			  (#match? @_import "[*]"))`,
			`((import_header (identifier) @kotlin__import__member) @_import
			  (#not-match? @_import "[*]"))`,
			`
((class_declaration (type_identifier) @modularity__types__total))
((object_declaration (type_identifier) @modularity__types__total))
`,
			`
((class_declaration
  (modifiers) @_mods
  (type_identifier) @modularity__types__abstract)
  (#match? @_mods "abstract"))
`,
		},
	}

	template.SnippetTransformers = map[string]func(*file.Snippet) *file.Snippet{
		importedMember: toPackage,
	}

	pack, err := common.PackFromTemplate(template)
	if err != nil {
		panic(err)
	}
	return pack
}

// The capture for an import that names a member rather than a package. It is
// only ever seen here, on its way to becoming an ordinary import.
const importedMember = "kotlin__import__member"

// toPackage turns what a non-wildcard import names into the component that
// holds it: `kotlinx.datetime.internal.JSJoda` is imported from
// `kotlinx.datetime.internal`, and a component is the package, never the
// class. A nested class loses both of its trailing segments, since anything
// upper case is a type inside the package rather than part of its name.
func toPackage(s *file.Snippet) *file.Snippet {
	segments := strings.Split(s.Value, ".")
	if len(segments) > 1 {
		segments = segments[:len(segments)-1]
	}
	for len(segments) > 1 {
		last := []rune(segments[len(segments)-1])
		if len(last) == 0 || !unicode.IsUpper(last[0]) {
			break
		}
		segments = segments[:len(segments)-1]
	}
	s.Value = strings.Join(segments, ".")
	s.Type = file.ComponentImport
	return s
}
