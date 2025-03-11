package file

import "github.com/archstats/archstats/core/stats"

type Results struct {
	Directory string
	Component string
	Name      string
	Stats     []*stats.Record
	Snippets  []*Snippet
}
