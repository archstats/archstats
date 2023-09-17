package indentations

import (
	"bufio"
	"bytes"
	"github.com/RyanSusana/archstats/analysis"
	"github.com/RyanSusana/archstats/analysis/file"
	"strings"
)

const (
	Max   = "indentation_max"
	Count = "indentation_count"
	Avg   = "indentation_avg"
)

func Extension() analysis.Extension {
	return &extension{}
}

type extension struct{}

func (i *extension) typeAssertions() (analysis.Extension, analysis.FileAnalyzer) {
	return i, i
}

func (i *extension) Init(settings analysis.Analyzer) error {
	settings.RegisterFileAnalyzer(i)
	settings.RegisterStatAccumulator(Max, maxAccumulator)
	settings.RegisterStatAccumulator(Avg, avgAccumulator)
	return nil
}

func maxAccumulator(indentations []interface{}) interface{} {
	curMax := 0
	for _, indentation := range indentations {
		if indentation.(int) > curMax {
			curMax = indentation.(int)
		}
	}
	return curMax
}

func avgAccumulator(indentations []interface{}) interface{} {
	allIndentations := 0.0
	allLines := 0.0
	for _, indentation := range indentations {
		stat := indentation.(*indentationStat)
		allIndentations += float64(stat.indentation)
		allLines += float64(stat.lines)
	}
	return allIndentations / allLines
}

func (i *extension) AnalyzeFile(theFile file.File) *file.Results {
	bytesReader := bytes.NewReader(theFile.Content())

	fileReader := bufio.NewReader(bytesReader)

	fileReader.ReadBytes('\n')

	var maxIndentations int
	var totalIndentation int
	var lineCount int
	for {
		line, err := fileReader.ReadBytes('\n')
		lineCount++
		if err != nil {
			break
		}
		indentation := getLeadingIndentation(line)
		totalIndentation += indentation
		if indentation > maxIndentations {
			maxIndentations = indentation
		}
	}

	return &file.Results{
		Stats: []*file.StatRecord{
			{
				StatType: Max,
				Value:    maxIndentations,
			},
			{
				StatType: Count,
				Value:    totalIndentation,
			},
			{
				StatType: Avg,
				Value: &indentationStat{
					indentation: totalIndentation,
					lines:       lineCount,
				},
			},
		},
	}
}

type indentationStat struct {
	indentation int
	lines       int
}

func getLeadingIndentation(line []byte) int {
	lineTabs := strings.ReplaceAll(string(line), "    ", "\t")

	indentation := 0
	for _, char := range lineTabs {
		if char == '\t' {
			indentation++
		} else {
			break
		}
	}

	return indentation
}
