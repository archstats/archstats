package archstats

import (
	"regexp"
)

type RegexStatPattern struct {
	Name   string
	Regexp *regexp.Regexp
}
type RegexBasedStats struct {
	Patterns []RegexStatPattern
}

func (r *RegexBasedStats) AfterFileProcessing(results *AfterFileProcessingResults) {
	for _, directory := range results.Directories {
		directory.Stats()
	}
}

func (r *RegexBasedStats) VisitFile(file File, content []byte) {
	stringContent := string(content)
	for _, pattern := range r.Patterns {
		matches := getMatches(pattern.Name, pattern.Regexp, stringContent)
		for _, match := range matches {
			file.RecordSnippet(match)
		}
	}
}

type SubexpMatch struct {
	name  string
	begin int
	end   int
}

func (s *SubexpMatch) Name() string {
	return s.name
}

func (s *SubexpMatch) Begin() int {
	return s.begin
}

func (s *SubexpMatch) End() int {
	return s.end
}

func getMatches(name string, regex *regexp.Regexp, content string) []*SubexpMatch {
	var toReturn []*SubexpMatch

	allMatches := regex.FindAllStringSubmatchIndex(content, 1000)
	names := regex.SubexpNames()
	for _, match := range allMatches {

		toReturn = append(toReturn, &SubexpMatch{
			name:  name,
			begin: match[0],
			end:   match[1],
		})
		pairs := toTuples(match, 2)
		for i, pair := range pairs {
			nameOfGroup := names[i]

			if nameOfGroup != "" {
				toReturn = append(toReturn, &SubexpMatch{
					name:  nameOfGroup,
					begin: pair[0],
					end:   pair[1],
				})
			}
		}
	}
	return toReturn
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
