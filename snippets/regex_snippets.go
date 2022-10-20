package snippets

import (
	"github.com/gobwas/glob"
	"regexp"
)

// RegexBasedSnippetsProvider is a SnippetProvider that uses regular expressions to find snippets.
// It can be configured to only match certain files using a glob.
// For a snippet to be recorded, it must be in a group with a name.
// For example, the following regex will match a function named "foo" and a function named "bar":
// (?P<function>func foo\(\) {.*?})|(?P<function>func bar\(\) {.*?})
//
// See https://www.regular-expressions.info/named.html for more information on named groups.
type RegexBasedSnippetsProvider struct {
	Glob     glob.Glob
	Patterns []*regexp.Regexp
}

func (s *RegexBasedSnippetsProvider) GetSnippetsFromFile(file File) []*Snippet {
	if s.Glob != nil && !s.Glob.Match(file.Path()) {
		return []*Snippet{}
	}
	var toReturn []*Snippet
	stringContent := string(file.Content())

	for _, pattern := range s.Patterns {
		matches := getMatches(pattern, &stringContent)

		for _, match := range matches {

			if match.begin == -1 || match.end == -1 {
				continue
			}
			theSnip := &Snippet{
				Type:  match.name,
				File:  file.Path(),
				Begin: match.begin,
				End:   match.end,
				Value: stringContent[match.begin:match.end],
			}
			toReturn = append(toReturn, theSnip)
		}
	}
	return toReturn
}

func getMatches(regex *regexp.Regexp, content *string) []*subexpMatch {
	var toReturn []*subexpMatch

	allMatches := regex.FindAllStringSubmatchIndex(*content, 1000)
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
