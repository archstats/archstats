package main

import (
	"fmt"
	"regexp"
)

const ok = `import(
	"fmt"
	"regexp"
)
`

func main() {

	m := regexp.MustCompile(`import\(([\n\r\s]+\W*"(?P<componentImport>.*)"\W*)*`)

	//x := getRegexes("import", m, ok)

	fmt.Println(m.FindAllStringSubmatchIndex(ok, 1000))
}

type SubexpMatch struct {
	name  string
	begin int
	end   int
}

func getRegexes(name string, regex *regexp.Regexp, content string) []SubexpMatch {
	var toReturn []SubexpMatch

	allMatches := regex.FindAllStringSubmatchIndex(content, 1000)
	names := regex.SubexpNames()
	for _, match := range allMatches {

		toReturn = append(toReturn, SubexpMatch{
			name:  name,
			begin: match[0],
			end:   match[1],
		})
		pairs := toTuples(match, 2)
		for i, pair := range pairs {
			nameOfGroup := names[i]

			if nameOfGroup != "" {
				toReturn = append(toReturn, SubexpMatch{
					name:  nameOfGroup,
					begin: pair[0],
					end:   pair[1],
				})
			}
		}
	}
	return toReturn
}

// flat slice to pairs
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
