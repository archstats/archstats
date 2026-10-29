// Package gomod reads the Go version a go.mod targets.
package gomod

import (
	"regexp"

	"github.com/archstats/archstats/extensions/cicd/buildtool"
)

// Tool reads go.mod modules.
func Tool() buildtool.Tool { return tool{} }

type tool struct{}

func (tool) Name() string               { return "go" }
func (tool) Kind(string, []byte) string { return "" }

var goVersion = regexp.MustCompile(`(?m)^go\s+(\d+\.\d+)`)

func (tool) Read(in buildtool.Input) buildtool.Facts {
	facts := buildtool.Facts{Runtimes: map[string]string{}}
	if in.Modules == nil {
		return facts
	}
	for _, mod := range in.Modules.Modules() {
		if mod.Kind != "go" {
			continue
		}
		if m := goVersion.FindSubmatch(in.Files[mod.Manifest]); m != nil {
			facts.Runtimes[mod.Manifest] = "go " + string(m[1])
		}
	}
	return facts
}
