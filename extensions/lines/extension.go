package lines

import (
	"bufio"
	"bytes"
	"embed"
	"github.com/archstats/archstats/core"
	"github.com/archstats/archstats/core/definitions"
	"github.com/archstats/archstats/core/file"
)

const LineCount = "complexity__lines"

type Extension struct {
}

//go:embed definitions/**
var defs embed.FS

func (i *Extension) Init(settings core.Analyzer) error {
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
func (i *Extension) AnalyzeFile(theFile file.File) *file.Results {
	bytesReader := bytes.NewReader(theFile.Content())

	fileReader := bufio.NewReader(bytesReader)

	var lineCount int
	for {
		_, err := fileReader.ReadBytes('\n')
		lineCount++
		if err != nil {
			break
		}
	}

	return &file.Results{
		Stats: []*file.StatRecord{
			{
				StatType: LineCount,
				Value:    lineCount,
			},
		},
	}
}

func (i *Extension) typeAssertions() (core.Extension, core.FileAnalyzer) {
	return i, i
}
