// Package swift reads Swift for navigation: an app's @main.
package swift

import (
	r "github.com/archstats/archstats/extensions/navigation/recogniser"
)

const swift = "swift"

// Plugin is Swift.
func Plugin() r.Plugin {
	return r.Plugin{
		Name: "swift",
		Languages: []*r.Language{{
			Name: swift, Extensions: []string{".swift"}, Kind: r.Code,
			Syntax: r.Syntax{
				LineComments:  []string{"//"},
				BlockComments: [][2]string{{"/*", "*/"}},
				Strings:       true,
				TripleQuotes:  true,
				CharLiterals:  true,
				Marks:         r.AtMarks,
				TypeName:      r.CLikeTypeName,
			},
		}},
		Recognisers: []*r.Recogniser{{
			Languages: []string{swift},
			ClassEntries: func(s *r.Source, d r.Decl) []r.Entry {
				if d.Has("main") == nil {
					return nil
				}
				return []r.Entry{{Kind: r.KindMain, Framework: "swift", Class: d.Name, Line: d.Line, DeclLine: d.Line}}
			},
			Declared:  r.ColonTypes,
			Constants: r.AssignedConstants,
		}},
	}
}
