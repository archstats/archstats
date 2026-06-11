package indentations

import (
	"bufio"
	"bytes"
	"embed"
	"github.com/archstats/archstats/core"
	"github.com/archstats/archstats/core/definitions"
	"github.com/archstats/archstats/core/file"
	"github.com/archstats/archstats/core/stats"
	"strings"
)

const (
	Max        = "complexity__indentation__max"
	Count      = "complexity__indentation__count"
	Avg        = "complexity__indentation__avg"
	Volatility = "complexity__indentation__volatility"
)

func FourTabs() *Extension {
	return &Extension{
		SpacesInTab: 4,
	}
}

func TwoTabs() *Extension {
	return &Extension{
		SpacesInTab: 2,
	}
}

//go:embed definitions/**
var defs embed.FS

type Extension struct {
	SpacesInTab int
}

func (i *Extension) typeAssertions() (core.Extension, core.FileAnalyzer) {
	return i, i
}

func (i *Extension) Init(settings core.Analyzer) error {
	defs, err := definitions.LoadYamlFiles(defs)
	if err != nil {
		return err
	}

	for _, definition := range defs {
		settings.AddDefinition(definition)
	}

	settings.RegisterFileAnalyzer(i)
	settings.RegisterStatAccumulator(Max, maxAccumulator)
	settings.RegisterStatAccumulator(Avg, avgAccumulator)
	settings.RegisterStatAccumulator(Volatility, sumAccumulator)
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

func sumAccumulator(values []interface{}) interface{} {
	var sum int
	for _, val := range values {
		if val == nil {
			continue
		}
		if i, ok := val.(int); ok {
			sum += i
		}
	}
	return sum
}

func (i *Extension) AnalyzeFile(theFile file.File) *file.Results {
	bytesReader := bytes.NewReader(theFile.Content())

	fileReader := bufio.NewReader(bytesReader)

	var maxIndentations int
	var totalIndentation int
	var lineCount int
	var volatility int
	var lastIndentation int = -1

	for {
		line, err := fileReader.ReadBytes('\n')
		if len(line) > 0 {
			trimmed := strings.TrimSpace(string(line))
			if trimmed != "" {
				lineCount++
				indentation := i.getLeadingIndentation(line)
				totalIndentation += indentation
				if indentation > maxIndentations {
					maxIndentations = indentation
				}
				if lastIndentation != -1 {
					diff := indentation - lastIndentation
					if diff < 0 {
						diff = -diff
					}
					volatility += diff
				}
				lastIndentation = indentation
			}
		}
		if err != nil {
			break
		}
	}

	return &file.Results{
		Stats: []*stats.Record{
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
			{
				StatType: Volatility,
				Value:    volatility,
			},
		},
	}
}

type indentationStat struct {
	indentation int
	lines       int
}

func (i *Extension) getLeadingIndentation(line []byte) int {
	lineTabs := strings.ReplaceAll(string(line), strings.Repeat(" ", i.SpacesInTab), "\t")
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
