package lines

import (
	"bytes"
	"embed"
	"github.com/archstats/archstats/core"
	"github.com/archstats/archstats/core/definitions"
	"github.com/archstats/archstats/core/file"
	"github.com/archstats/archstats/core/stats"
)

const LineCount = "complexity__lines"

func Extension() core.Extension {
	return &extension{}
}

type extension struct {
}

//go:embed definitions/**
var defs embed.FS

func (i *extension) Init(settings core.Analyzer) error {
	defs, err := definitions.LoadYamlFiles(defs)

	if err != nil {
		return err
	}

	for _, definition := range defs {
		settings.AddDefinition(definition)
	}
	settings.RegisterFileAnalyzer(i)
	return nil
}
func (i *extension) AnalyzeFile(theFile file.File) *file.Results {
	return &file.Results{
		Stats: []*stats.Record{
			{
				StatType: LineCount,
				Value:    countLines(theFile.Content()),
			},
		},
	}
}

// countLines counts lines the way an editor and `wc -l` do: a final newline
// ends the last line rather than starting another, and an empty file has
// none. Counting the text after the final newline as a line put every file
// one line over -- django-oscar's one-line views.py read as two, and its
// 1,407 files as 1,407 lines more than they hold.
func countLines(content []byte) int {
	if len(content) == 0 {
		return 0
	}
	n := bytes.Count(content, []byte("\n"))
	if content[len(content)-1] != '\n' {
		n++
	}
	return n
}

func (i *extension) typeAssertions() (core.Extension, core.FileAnalyzer) {
	return i, i
}
