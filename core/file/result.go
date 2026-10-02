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
	// The functions declared in this file, measured; see Function.
	Functions []*Function
	// Someone else's code carried in the repository; see IsThirdParty.
	ThirdParty bool
	// Written by a tool, by its own header's account; see IsGenerated.
	Generated bool
	// Role is production, test, generated, third_party or non_code; see Role.
	Role string
	// SystemKind is build, lockfile, ci, container, deploy, infra, config or
	// empty; see SystemKind.
	SystemKind string
}
