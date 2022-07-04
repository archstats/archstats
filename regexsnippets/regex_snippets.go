package regexsnippets

import (
	"analyzer/core"
	"regexp"
)

type RegexBasedSnippetsProvider struct {
	Patterns []*regexp.Regexp
}

func (s *RegexBasedSnippetsProvider) GetSnippetsFromFile(file core.File) []*core.Snippet {
	var toReturn []*core.Snippet
	stringContent := string(file.Content())

	for _, pattern := range s.Patterns {
		matches := getMatches(pattern, stringContent)

		for _, match := range matches {
			theSnip := &core.Snippet{
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

func getMatches(regex *regexp.Regexp, content string) []*subexpMatch {
	var toReturn []*subexpMatch

	allMatches := regex.FindAllStringSubmatchIndex(content, 1000)
	names := regex.SubexpNames()
	for _, match := range allMatches {

		pairs := toTuples(match, 2)
		for i, pair := range pairs {
			nameOfGroup := names[i]

			if nameOfGroup != "" {
				toReturn = append(toReturn, &subexpMatch{
					name:  nameOfGroup,
					begin: pair[0],
					end:   pair[1],
				})
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
