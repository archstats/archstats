package indentation

import (
	"github.com/RyanSusana/archstats/analysis"
)

type Analyzer struct {
	indentationStats map[string]indentationStats
}

func (i *Analyzer) AnalyzeFile(file analysis.File) *analysis.FileResults {
	//content := file.Content()

	//read line by line
	lines := []string{}

	var maxIndentations int
	var totalIndentation int

	for _, line := range lines {
		curIndentation := getLeadingIndentation(line)

		totalIndentation += curIndentation

		if curIndentation > maxIndentations {
			maxIndentations = curIndentation
		}
	}

	return nil
}

func getLeadingIndentation(line string) int {

	return 0
}

type indentationStats struct {
	file              string
	maxIndentations   int
	totalLines        int
	totalIndentations int
}
