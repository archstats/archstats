package regex

import (
	"github.com/RyanSusana/archstats/analysis"
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

func (s *Extension) Init(a analysis.Analyzer) error {
	a.RegisterFileAnalyzer(s)
	return nil
}

func (s *Extension) AnalyzeFile(file analysis.File) *analysis.FileResults {
	if s.Glob != nil && !s.Glob.Match(file.Path()) {
		return &analysis.FileResults{}
	}
	var toReturn []*analysis.Snippet
	stringContent := string(file.Content())

	for _, pattern := range s.Patterns {
		matches := getMatches(pattern, &stringContent)

		for _, match := range matches {

			if match.begin == -1 || match.end == -1 {
				continue
			}
			theSnip := &analysis.Snippet{
				Type:  match.name,
				File:  file.Path(),
				Begin: match.begin,
				End:   match.end,
				Value: stringContent[match.begin:match.end],
			}
			toReturn = append(toReturn, theSnip)
		}
	}
	if s.OnlyStats {
		return &analysis.FileResults{
			Stats: analysis.SnippetsToStats(toReturn),
		}
	} else {
		return &analysis.FileResults{
			Snippets: toReturn,
			Stats:    analysis.SnippetsToStats(toReturn),
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
