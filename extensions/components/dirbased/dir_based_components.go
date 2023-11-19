package dirbased

import (
	"github.com/archstats/archstats/core"
	"github.com/archstats/archstats/core/file"
)

type componentLinker struct {
}

func (c *componentLinker) Init(settings core.Analyzer) error {
	return nil
}

func (c *componentLinker) interfaceAssertions() core.FileResultsEditor {
	return c
}

func (c *componentLinker) EditFileResults(allFileResults []*file.Results) {
	for _, result := range allFileResults {
		for _, snippet := range result.Snippets {
			snippet.Component = snippet.Directory
		}
	}
}
