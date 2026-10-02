package indentations

import (
	"bufio"
	"bytes"
	"embed"
	"github.com/archstats/archstats/core"
	"github.com/archstats/archstats/core/definitions"
	"github.com/archstats/archstats/core/file"
	"github.com/archstats/archstats/core/stats"
	"path/filepath"
	"strings"
)

const (
	Max        = "complexity__indentation__max"
	Count      = "complexity__indentation__count"
	Avg        = "complexity__indentation__avg"
	Volatility = "complexity__indentation__volatility"
	// NonBlank counts the lines with anything on them, comments included.
	NonBlank = "complexity__lines__nonblank"
	// Deep counts the lines indented DeepLevel levels or more: in a language
	// no pack parses, the stand-in for code inside complex functions.
	Deep = "complexity__indentation__deep"

	// DeepLevel is where code health's fallback starts calling a line deep.
	// Three levels in predicted fixes better than four, five or six, and
	// better than the average or the deepest line.
	DeepLevel = 3
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

// Detected reads each file's indentation from the project, then the file.
func Detected() *Extension {
	return &Extension{}
}

//go:embed definitions/**
var defs embed.FS

type Extension struct {
	// SpacesInTab is the spaces that make one level in every file. 0 reads it
	// per file: the project's Prettier config or .editorconfig first, the
	// file's own indentation after that, tabWidth when neither says.
	SpacesInTab int

	root    string
	project *projectSettings
}

func (i *Extension) typeAssertions() (core.Extension, core.FileAnalyzer) {
	return i, i
}

func (i *Extension) Init(settings core.Analyzer) error {
	if root, err := filepath.Abs(settings.RootPath()); err == nil {
		i.root = root
	}
	i.project = newProjectSettings()
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
	// An empty or blank file has no indented lines to average: 0, not the
	// NaN of 0/0, which was stored as NULL and blanked the file's static
	// complexity with it.
	if allLines == 0 {
		return 0.0
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
	var deep int
	var lastIndentation int = -1
	width := i.widthFor(theFile)

	for {
		line, err := fileReader.ReadBytes('\n')
		if len(line) > 0 {
			trimmed := strings.TrimSpace(string(line))
			if trimmed != "" {
				lineCount++
				indentation := leadingIndentation(line, width)
				totalIndentation += indentation
				if indentation > maxIndentations {
					maxIndentations = indentation
				}
				if indentation >= DeepLevel {
					deep++
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
			{StatType: NonBlank, Value: lineCount},
			{StatType: Deep, Value: deep},
		},
	}
}

type indentationStat struct {
	indentation int
	lines       int
}

// widthFor is the spaces that make one level in theFile. Codebases differ:
// Go indents with tabs, most TypeScript with two spaces, most Java with four.
// Read with one fixed width, a two-space file came out half as deep as it is.
func (i *Extension) widthFor(theFile file.File) int {
	if i.SpacesInTab > 0 {
		return i.SpacesInTab
	}
	if i.project != nil && i.root != "" && theFile.Path() != "" {
		if w, ok := i.project.width(filepath.Join(i.root, filepath.FromSlash(theFile.Path()))); ok {
			return w
		}
	}
	if w, ok := detectWidth(theFile.Content()); ok {
		return w
	}
	return tabWidth
}

// leadingIndentation counts a line's levels: one per tab, one per width
// spaces.
func leadingIndentation(line []byte, width int) int {
	tabs, spaces := 0, 0
	for _, c := range line {
		switch c {
		case '\t':
			tabs++
		case ' ':
			spaces++
		default:
			return tabs + spaces/width
		}
	}
	return tabs + spaces/width
}
