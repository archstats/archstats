// Package node reads what npm packages ship: React Native and Expo apps,
// Electron apps, and the Node version a package says it runs on.
package node

import (
	"regexp"

	"github.com/archstats/archstats/extensions/cicd/buildtool"
)

// Tool reads package.json modules.
func Tool() buildtool.Tool { return tool{} }

type tool struct{}

func (tool) Name() string               { return "node" }
func (tool) Kind(string, []byte) string { return "" }

var engines = regexp.MustCompile(`"engines"\s*:\s*\{[^}]*"node"\s*:\s*"[^0-9]*(\d+)`)

func (tool) Read(in buildtool.Input) buildtool.Facts {
	facts := buildtool.Facts{Runtimes: map[string]string{}}
	if in.Modules == nil {
		return facts
	}
	for _, mod := range in.Modules.Modules() {
		if mod.Kind != "node" {
			continue
		}
		if m := engines.FindSubmatch(in.Files[mod.Manifest]); m != nil {
			facts.Runtimes[mod.Manifest] = "node " + string(m[1])
		}
		switch {
		case buildtool.HasDep(mod, "expo"):
			facts.Mobile = append(facts.Mobile, buildtool.MobileApp{Module: mod, Platform: "react-native", BuiltBy: "expo", CrossPlatform: true})
		case buildtool.HasDep(mod, "react-native"):
			facts.Mobile = append(facts.Mobile, buildtool.MobileApp{Module: mod, Platform: "react-native", BuiltBy: "react-native", CrossPlatform: true})
		}
		if buildtool.HasDep(mod, "electron") {
			facts.Desktop = append(facts.Desktop, buildtool.DesktopApp{Names: []string{mod.Name}, Dir: cleanDir(mod.Dir), File: mod.Manifest, Platform: "electron"})
		}
	}
	return facts
}

func cleanDir(d string) string {
	if d == "" {
		return "."
	}
	return d
}
