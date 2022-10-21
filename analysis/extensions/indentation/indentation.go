package indentation

import (
	"bufio"
	"bytes"
	"github.com/RyanSusana/archstats/analysis"
	"strings"
)

const (
	IndentationMax   = "indentation_max"
	IndentationCount = "indentation_count"
)

type Analyzer struct {
	indentationStats map[string]indentationStats
}

func (i *Analyzer) typeAssertions() (analysis.Initializable, analysis.FileAnalyzer) {
	return i, i
}

func (i *Analyzer) Init(settings analysis.Settings) {
	settings.SetStatAccumulator(IndentationMax, func(indentations []interface{}) interface{} {
		curMax := 0
		for _, indentation := range indentations {
			if indentation.(int) > curMax {
				curMax = indentation.(int)
			}

		}
		return curMax
	})
}

func (i *Analyzer) AnalyzeFile(file analysis.File) *analysis.FileResults {
	bytesReader := bytes.NewReader(file.Content())

	fileReader := bufio.NewReader(bytesReader)

	fileReader.ReadBytes('\n')

	var maxIndentations int
	var totalIndentation int
	for {
		line, err := fileReader.ReadBytes('\n')
		if err != nil {
			break
		}
		indentation := getLeadingIndentation(line)
		totalIndentation += indentation
		if indentation > maxIndentations {
			maxIndentations = indentation
		}
	}

	return &analysis.FileResults{
		Stats: []*analysis.StatRecord{
			{
				StatType: IndentationMax,
				Value:    maxIndentations,
			},
			{
				StatType: IndentationCount,
				Value:    totalIndentation,
			},
		},
	}
}

func getLeadingIndentation(line []byte) int {
	strings.ReplaceAll(string(line), "   ", "\t")

	return 0
}

type indentationStats struct {
	file              string
	maxIndentations   int
	totalLines        int
	totalIndentations int
}
