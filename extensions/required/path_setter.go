package required

import (
	"github.com/RyanSusana/archstats/analysis"
	"github.com/RyanSusana/archstats/analysis/file"
	"strings"
)

// A FileResultsEditor is a function that edits a snippet to remove the unwanted parts of the absolute path
type rootPathStripper struct {
	root string
}

func (p *rootPathStripper) Init(settings analysis.Analyzer) error {
	p.root = settings.RootPath()
	return nil
}

func (p *rootPathStripper) EditFileResults(results []*file.Results) {
	for _, result := range results {
		newFileName := result.Name[len(p.root):]
		result.Name = newFileName
		directoryName := newFileName[:strings.LastIndex(newFileName, "/")]
		for _, snippet := range result.Snippets {
			snippet.File = newFileName
			snippet.Directory = directoryName
		}
	}
}
