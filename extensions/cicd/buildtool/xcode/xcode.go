// Package xcode reads the iOS applications Xcode projects build.
package xcode

import "github.com/archstats/archstats/extensions/cicd/buildtool"

// Tool reads Xcode and SwiftPM modules.
func Tool() buildtool.Tool { return tool{} }

type tool struct{}

func (tool) Name() string               { return "xcode" }
func (tool) Kind(string, []byte) string { return "" }

func (tool) Read(in buildtool.Input) buildtool.Facts {
	facts := buildtool.Facts{MobileModuleKinds: []string{"xcode", "swiftpm"}}
	if in.Modules == nil {
		return facts
	}
	for _, mod := range in.Modules.Modules() {
		if mod.Kind == "xcode" && mod.Type == "ios-application" {
			facts.Mobile = append(facts.Mobile, buildtool.MobileApp{Module: mod, Platform: "ios", BuiltBy: "xcode"})
		}
	}
	return facts
}
