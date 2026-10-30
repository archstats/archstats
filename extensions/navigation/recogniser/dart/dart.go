// Package dart reads Dart for navigation: a program's main.
package dart

import (
	"regexp"

	r "github.com/archstats/archstats/extensions/navigation/recogniser"
)

const dart = "dart"

var dartMain = regexp.MustCompile(`(?m)^(?:void|Future<void>|Future)\s+main\s*\(`)

// Plugin is Dart.
func Plugin() r.Plugin {
	return r.Plugin{
		Name: "dart",
		Languages: []*r.Language{{
			Name: dart, Extensions: []string{".dart"}, Kind: r.Code,
			Syntax: r.Syntax{
				LineComments:        []string{"//"},
				BlockComments:       [][2]string{{"/*", "*/"}},
				Strings:             true,
				TripleQuotes:        true,
				Marks:               r.AtMarks,
				TypeName:            r.CLikeTypeName,
				DollarInterpolation: true,
			},
		}},
		Recognisers: []*r.Recogniser{{
			Languages: []string{dart},
			MainEntries: func(s *r.Source) []r.Entry {
				if loc := dartMain.FindStringIndex(s.Code); loc != nil {
					return []r.Entry{r.MainEntry(s, loc[0], "")}
				}
				return nil
			},
			Declared: func(s *r.Source, sy *r.Symbols) { r.TypedNames(s, sy); r.ColonTypes(s, sy) },
			Constants: func(s *r.Source, sy *r.Symbols) {
				r.StaticConstants(s, sy)
				r.AssignedConstants(s, sy)
			},
		}},
	}
}
