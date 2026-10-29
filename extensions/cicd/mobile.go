package cicd

import (
	"sort"
	"strings"

	"github.com/archstats/archstats/core/module"
	"github.com/archstats/archstats/extensions/cicd/buildtool"
)

// Mobile apps are deployables too: what ships to a store is a module a
// build tool says is a store app -- an Android application, an iOS app
// target, a Flutter or React Native app -- and what matters about each is the same as for an image -- which code
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
		buildtool.MobileApp
	}
	var apps []app
	for _, facts := range b.tools {
		for _, m := range facts.Mobile {
			apps = append(apps, app{m})
		}
	}
	// A native project inside a cross-platform app is that app's.
	var crossDirs []string
	for _, a := range apps {
		if a.CrossPlatform {
			crossDirs = append(crossDirs, cleanDir(a.Module.Dir))
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
	sort.SliceStable(apps, func(i, j int) bool { return apps[i].Module.Manifest < apps[j].Module.Manifest })
	for _, a := range apps {
		dir := cleanDir(a.Module.Dir)
		if !a.CrossPlatform && inside(dir) {
			continue
		}
		if b.skipped(a.Module.Manifest) {
			continue
		}
		d := &Artifact{Kind: "mobile_app", Platform: a.Platform, BuiltBy: a.BuiltBy, File: a.Module.Manifest, Line: 1, Context: dir, priority: 5}
		d.aliases = []string{a.Module.Name}
		d.modules = []string{a.Module.Name}
		if a.StoreID != "" {
			d.aliases = append(d.aliases, a.StoreID)
		}
		b.m.Artifacts = append(b.m.Artifacts, d)
		b.addContent(d, &Content{Path: dir, Module: a.Module.Name, File: a.Module.Manifest, Line: 1, Resolution: "declared"})
		for _, f := range a.Module.Files {
			if dirOf(f) != dir {
				b.addContent(d, &Content{Path: dirOf(f), Module: a.Module.Name, File: a.Module.Manifest, Line: 1, Resolution: "declared"})
			}
		}
		for _, extra := range a.Module.Dirs {
			b.addContent(d, &Content{Path: cleanDir(extra), Module: a.Module.Name, File: a.Module.Manifest, Line: 1, Resolution: "declared"})
		}
	}
}

// mobilePart is whether a module found inside a mobile app's folder is part
// of the app: the app's own module, or one of a kind the build tools say a
// mobile app is made of.
func (b *builder) mobilePart(d *Artifact, mod *module.Module) bool {
	if len(d.aliases) > 0 && mod.Name == d.aliases[0] {
		return true
	}
	return b.mobileKinds[mod.Kind]
}
