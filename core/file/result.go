package file

import (
	"github.com/archstats/archstats/core/stats"
	"github.com/archstats/archstats/core/unit"
)

type Results struct {
	Directory string
	Component string
	Name      string
	Stats     []*stats.Record
	Snippets  []*Snippet
	// The named things declared in this file. A language pack fills these in
	// the same way it fills Stats and Snippets; the engine folds units that
	// turn out to be the same thing across files.
	Units []*unit.Unit
}
