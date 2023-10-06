package regex

import (
	"github.com/archstats/archstats/core"
	"github.com/archstats/archstats/core/file"
	"github.com/gobwas/glob"
	"regexp"
)

// Extension is a FileAnalyzer that uses regular expressions to find snippets.
// It can be configured to only match certain files using a glob.
// For a snippet to be recorded, it must be in a group with a name.
// For example, the following regex will match a function named "foo" and a function named "bar":
// (?P<function>func foo\(\) {.*?})|(?P<function>func bar\(\) {.*?})
//
// See https://www.regular-expressions.info/named.html for more information on named groups.
type Extension struct {
	OnlyStats bool
	Glob      glob.Glob
	Patterns  []*regexp.Regexp
}

func (s *Extension) Init(a core.Analyzer) error {
	a.RegisterFileAnalyzer(s)
	return nil
}

func (s *Extension) AnalyzeFile(theFile file.File) *file.Results {
	if s.Glob != nil && !s.Glob.Match(theFile.Path()) {
		return &file.Results{}
	}
	var toReturn []*file.Snippet
	stringContent := string(theFile.Content())

	for _, pattern := range s.Patterns {
		matches := getMatches(pattern, &stringContent)

		for _, match := range matches {

			if match.begin == -1 || match.end == -1 {
				continue
			}
			theSnip := &file.Snippet{
				Type: match.name,
				File: theFile.Path(),
				Begin: &file.Position{
					Offset: match.begin,
				},
				End: &file.Position{
					Offset: match.end,
				},
				Value: stringContent[match.begin:match.end],
			}
			toReturn = append(toReturn, theSnip)
		}
	}

	if s.OnlyStats {
		return &file.Results{
			Stats: file.SnippetsToStats(toReturn),
		}
	} else {
		return &file.Results{
			Snippets: toReturn,
			Stats:    file.SnippetsToStats(toReturn),
		}
	}
}

func getMatches(regex *regexp.Regexp, content *string) []*subexpMatch {
	var toReturn []*subexpMatch

	allMatches := regex.FindAllStringSubmatchIndex(*content, 1_000_000)
	names := regex.SubexpNames()
	for _, match := range allMatches {

		pairs := toTuples(match, 2)
		for i, pair := range pairs {
			nameOfGroup := names[i]

			if nameOfGroup != "" {
				subMatch := &subexpMatch{
					name:  nameOfGroup,
					begin: pair[0],
					end:   pair[1],
				}
				if !(subMatch.begin == -1 || subMatch.end == -1) {
					toReturn = append(toReturn, subMatch)
				}
			}
		}
	}
	return toReturn
}

type subexpMatch struct {
	name  string
	begin int
	end   int
}

func toTuples(input []int, size int) [][]int {
	returnSize := len(input) / size
	tuples := make([][]int, 0, returnSize)
	for i := 0; i < len(input); i += size {
		newTuple := make([]int, size, size)

		for j := 0; j < size; j++ {
			newTuple[j] = input[i+j]
		}
		tuples = append(tuples, newTuple)
	}
	return tuples
}
