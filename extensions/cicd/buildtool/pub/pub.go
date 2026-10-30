// Package pub reads the Flutter apps pubspecs declare.
package pub

import "github.com/archstats/archstats/extensions/cicd/buildtool"

// Tool reads pub modules.
func Tool() buildtool.Tool { return tool{} }

type tool struct{}

func (tool) Name() string               { return "pub" }
func (tool) Kind(string, []byte) string { return "" }

func (tool) Read(in buildtool.Input) buildtool.Facts {
	facts := buildtool.Facts{MobileModuleKinds: []string{"pub"}}
	if in.Modules == nil {
		return facts
	}
	for _, mod := range in.Modules.Modules() {
		if mod.Kind == "pub" && mod.Type == "flutter-app" {
			facts.Mobile = append(facts.Mobile, buildtool.MobileApp{Module: mod, Platform: "flutter", BuiltBy: "flutter", CrossPlatform: true})
		}
	}
	return facts
}
