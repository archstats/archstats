// Package swift analyses Swift.
//
// Swift has no packages. A target is a module: every file in it sees every
// other file's types without an import, and `import Timeline` names a target
// or a framework, never a directory. So a file's imports say which modules it
// uses, and nothing about which of the directories around it. Directory
// edges inside a target come from the types a file mentions, resolved within
// that target once every file has been read (link.go). Read the way Java or
// Go are, IceCubesApp's 428 files, isowords' 388 and Kickstarter's 2,080 came
// out as components with no edges -- or, before this pack, as nothing.
package swift

import (
	"github.com/archstats/archstats/core"
	"github.com/archstats/archstats/core/file"
	"github.com/archstats/archstats/extensions/treesitter/apple"
	"github.com/archstats/archstats/extensions/treesitter/common"
	swift "github.com/archstats/archstats/extensions/treesitter/swift/grammar"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

type Extension struct{}

func (e *Extension) Init(settings core.Analyzer) error {
	settings.RegisterFileAnalyzer(&swiftAnalyzer{lp: createSwiftLanguagePack()})
	settings.RegisterFileResultsEditor(&apple.Linker{Root: settings.RootPath()})
	return nil
}

func createSwiftLanguagePack() *common.LanguagePack {
	template := &common.LanguagePackTemplate{
		FileGlob: "**.swift",
		Language: tree_sitter.NewLanguage(swift.Language()),
		QueriesForStats: []string{
			// The module, as written: SwiftUI, UIKit, ComposableArchitecture,
			// or one of the codebase's own targets. Framework detection reads
			// these; the component graph does not, because a module is not a
			// directory.
			`(import_declaration (identifier) @` + file.ImportRaw + `)`,
			`(class_declaration declaration_kind: ["class" "struct" "enum" "actor"] name: (type_identifier) @` + file.Type + `)`,
			`(protocol_declaration name: (type_identifier) @` + file.Type + `)`,
			`(protocol_declaration name: (type_identifier) @` + file.AbstractType + `)`,
		},
		ComponentResolution: common.DirectoryBasedComponentResolution,
		QueriesForSnippets:  unitQueries(),
	}
	pack, err := common.PackFromTemplate(template)
	if err != nil {
		panic(err)
	}
	return pack
}
