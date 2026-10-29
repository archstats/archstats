package cicd

import (
	"regexp"
	"sort"
	"strings"

	"github.com/archstats/archstats/core/module"
)

// Mobile apps are deployables too: what ships to a store is an Android
// application module, an iOS app target, a Flutter app or a React Native
// app, and what matters about each is the same as for an image -- which code
// is inside, and what technology it carries. The native projects inside a
// Flutter or React Native app (its android/ and ios/ folders) are part of
// that app, not apps of their own.
//
// Their stack is read from what the builds declare: Gradle files through the
// version catalog most Android builds keep their versions in, Xcode package
// products and SwiftPM manifests with the versions Package.resolved pins,
// Podfile.lock, pubspec.yaml with pubspec.lock, and package.json.

func (b *builder) mobileArtifacts() {
	type app struct {
		mod      *module.Module
		platform string
		builtBy  string
	}
	var apps []app
	for _, mod := range b.in.Modules.Modules() {
		switch {
		case mod.Kind == "gradle" && mod.Type == "android-application":
			apps = append(apps, app{mod, "android", "gradle"})
		case mod.Kind == "xcode" && mod.Type == "ios-application":
			apps = append(apps, app{mod, "ios", "xcode"})
		case mod.Kind == "pub" && mod.Type == "flutter-app":
			apps = append(apps, app{mod, "flutter", "flutter"})
		case mod.Kind == "node" && (hasDep(mod, "react-native") || hasDep(mod, "expo")):
			by := "react-native"
			if hasDep(mod, "expo") {
				by = "expo"
			}
			apps = append(apps, app{mod, "react-native", by})
		}
	}
	// A native project inside a cross-platform app is that app's.
	var crossDirs []string
	for _, a := range apps {
		if a.platform == "flutter" || a.platform == "react-native" {
			crossDirs = append(crossDirs, cleanDir(a.mod.Dir))
		}
	}
	inside := func(dir string) bool {
		for _, c := range crossDirs {
			if c == "." || dir == c || strings.HasPrefix(dir, c+"/") {
				return true
			}
		}
		return false
	}
	sort.SliceStable(apps, func(i, j int) bool { return apps[i].mod.Manifest < apps[j].mod.Manifest })
	for _, a := range apps {
		dir := cleanDir(a.mod.Dir)
		if (a.platform == "android" || a.platform == "ios") && inside(dir) {
			continue
		}
		if b.skipped(a.mod.Manifest) {
			continue
		}
		d := &Artifact{Kind: "mobile_app", Platform: a.platform, BuiltBy: a.builtBy, File: a.mod.Manifest, Line: 1, Context: dir, priority: 5}
		d.aliases = []string{a.mod.Name}
		d.modules = []string{a.mod.Name}
		if a.platform == "android" {
			if m := gradleAppID.FindSubmatch(b.in.Files[a.mod.Manifest]); m != nil {
				d.aliases = append(d.aliases, string(m[1]))
			}
		}
		b.m.Artifacts = append(b.m.Artifacts, d)
		b.addContent(d, &Content{Path: dir, Module: a.mod.Name, File: a.mod.Manifest, Line: 1, Resolution: "declared"})
		for _, f := range a.mod.Files {
			if dirOf(f) != dir {
				b.addContent(d, &Content{Path: dirOf(f), Module: a.mod.Name, File: a.mod.Manifest, Line: 1, Resolution: "declared"})
			}
		}
		for _, extra := range a.mod.Dirs {
			b.addContent(d, &Content{Path: cleanDir(extra), Module: a.mod.Name, File: a.mod.Manifest, Line: 1, Resolution: "declared"})
		}
	}
}

// mobilePart is whether a module found inside a mobile app's folder is part
// of the app: the app's own module, or a native, Swift or Dart one.
func mobilePart(d *Artifact, mod *module.Module) bool {
	if len(d.aliases) > 0 && mod.Name == d.aliases[0] {
		return true
	}
	switch mod.Kind {
	case "gradle", "xcode", "swiftpm", "pub":
		return true
	}
	return false
}

func hasDep(mod *module.Module, name string) bool {
	for _, d := range mod.DependsOn {
		if d == name {
			return true
		}
	}
	return false
}

var gradleAppID = regexp.MustCompile(`\bapplicationId\s*=?\s*["']([A-Za-z0-9_.]+)["']`)
