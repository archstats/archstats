package analysis

import "strings"

// A FileResultsEditor is a function that edits a snippet to remove the unwanted parts of the absolute path
type rootPathStripper struct {
	root string
}

func (p *rootPathStripper) Init(settings *Settings) {
	p.root = settings.RootPath
}

func (p *rootPathStripper) EditFileResults(results []*FileResults) {
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
