package deployables

import (
	"encoding/json"
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

func (b *builder) mobileDeployables() {
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
		d := &Deployable{Kind: "mobile_app", Platform: a.platform, BuiltBy: a.builtBy, File: a.mod.Manifest, Line: 1, Context: dir, priority: 5}
		d.aliases = []string{a.mod.Name}
		d.modules = []string{a.mod.Name}
		if a.platform == "android" {
			if m := gradleAppID.FindSubmatch(b.in.Files[a.mod.Manifest]); m != nil {
				d.aliases = append(d.aliases, string(m[1]))
			}
		}
		b.m.Deployables = append(b.m.Deployables, d)
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
func mobilePart(d *Deployable, mod *module.Module) bool {
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

// ---------------------------------------------------------------------------
// Android: Gradle and the version catalog
// ---------------------------------------------------------------------------

var gradleAppID = regexp.MustCompile(`\bapplicationId\s*=?\s*["']([A-Za-z0-9_.]+)["']`)

var (
	gradleLibDep    = regexp.MustCompile(`(?m)^\s*(implementation|api|ksp|kapt|compileOnly|runtimeOnly|annotationProcessor|coreLibraryDesugaring)\s*\(\s*(?:platform\(\s*)?(?:libs\.([A-Za-z0-9_.]+)|["']([^"':]+):([^"':]+)(?::([^"']+))?["'])`)
	gradlePluginRef = regexp.MustCompile(`alias\(\s*libs\.plugins\.([A-Za-z0-9_.]+)\s*\)|id\(\s*["']([^"']+)["']\s*\)|kotlin\(\s*["']([^"']+)["']\s*\)`)
	gradleSdk       = regexp.MustCompile(`\b(minSdk|targetSdk|compileSdk)(?:Version)?\s*=?\s*(?:libs\.versions\.([A-Za-z0-9_.]+)[^\n]*|(\d+))`)
	composeFeature  = regexp.MustCompile(`\bcompose\s*=\s*true`)
	tomlSection     = regexp.MustCompile(`^\s*\[([A-Za-z0-9_-]+)\]\s*$`)
	tomlEntry       = regexp.MustCompile(`^\s*([A-Za-z0-9_.-]+)\s*=\s*(.+?)\s*$`)
	tomlField       = regexp.MustCompile(`([A-Za-z0-9_.]+)\s*=\s*"([^"]*)"`)
)

type catalog struct {
	versions  map[string]string
	libraries map[string][3]string // alias -> group, artifact, version
	plugins   map[string][2]string // alias -> id, version
	file      string
}

// catalogKey is how an accessor names an alias: `libs.androidx.room.runtime`
// is `androidx-room-runtime`, `androidx_room_runtime` or
// `androidx.room.runtime` in the file.
func catalogKey(s string) string {
	return strings.ToLower(strings.NewReplacer("-", ".", "_", ".").Replace(s))
}

func readCatalog(file string, content []byte) *catalog {
	c := &catalog{versions: map[string]string{}, libraries: map[string][3]string{}, plugins: map[string][2]string{}, file: file}
	section := ""
	for _, line := range strings.Split(string(content), "\n") {
		if m := tomlSection.FindStringSubmatch(line); m != nil {
			section = m[1]
			continue
		}
		m := tomlEntry.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		key, raw := catalogKey(m[1]), strings.TrimSpace(m[2])
		fields := map[string]string{}
		for _, f := range tomlField.FindAllStringSubmatch(raw, -1) {
			fields[f[1]] = f[2]
		}
		plain := strings.Trim(raw, `"`)
		switch section {
		case "versions":
			if strings.HasPrefix(raw, `"`) {
				c.versions[key] = plain
			} else if v := fields["strictly"] + fields["require"] + fields["prefer"]; v != "" {
				c.versions[key] = v
			}
		case "libraries":
			group, artifact, version := fields["group"], fields["name"], fields["version"]
			if mod := fields["module"]; mod != "" {
				group, artifact, _ = strings.Cut(mod, ":")
			}
			if strings.HasPrefix(raw, `"`) {
				parts := strings.Split(plain, ":")
				if len(parts) >= 2 {
					group, artifact = parts[0], parts[1]
				}
				if len(parts) == 3 {
					version = parts[2]
				}
			}
			if ref := fields["version.ref"]; ref != "" {
				version = c.versions[catalogKey(ref)]
			}
			if group != "" && artifact != "" {
				c.libraries[key] = [3]string{group, artifact, version}
			}
		case "plugins":
			id, version := fields["id"], fields["version"]
			if strings.HasPrefix(raw, `"`) {
				id, version, _ = strings.Cut(plain, ":")
			}
			if ref := fields["version.ref"]; ref != "" {
				version = c.versions[catalogKey(ref)]
			}
			if id != "" {
				c.plugins[key] = [2]string{id, version}
			}
		}
	}
	return c
}

// catalogFor is the version catalog of the build a Gradle file belongs to:
// the nearest gradle/libs.versions.toml above it.
func (b *builder) catalogFor(file string) *catalog {
	if b.catalogs == nil {
		b.catalogs = map[string]*catalog{}
		for f, content := range b.in.Files {
			if strings.HasSuffix(f, "libs.versions.toml") {
				b.catalogs[f] = readCatalog(f, content)
			}
		}
	}
	for dir := dirOf(file); ; dir = dirOf(dir) {
		for _, name := range []string{"gradle/libs.versions.toml", "libs.versions.toml"} {
			if c := b.catalogs[joinPath(dir, name)]; c != nil {
				return c
			}
		}
		if dir == "." || dir == "" {
			return nil
		}
	}
}

// gradleStack records what one Gradle file declares: libraries (through the
// catalog or as coordinates), plugins that are frameworks, and SDK levels.
func (b *builder) gradleStack(d *Deployable, file string, content []byte, primary bool) {
	cat := b.catalogFor(file)
	text := string(content)
	for _, m := range gradleLibDep.FindAllStringSubmatchIndex(text, -1) {
		line := strings.Count(text[:m[0]], "\n") + 1
		sub := func(i int) string {
			if m[2*i] < 0 {
				return ""
			}
			return text[m[2*i]:m[2*i+1]]
		}
		var group, artifact, version string
		if alias := sub(2); alias != "" {
			if cat == nil {
				continue
			}
			lib, ok := cat.libraries[catalogKey(alias)]
			if !ok {
				continue
			}
			group, artifact, version = lib[0], lib[1], lib[2]
		} else {
			group, artifact, version = sub(3), sub(4), sub(5)
		}
		role := "library"
		if strings.HasPrefix(group, "androidx.compose") || group == "org.jetbrains.compose" {
			b.dep(d, "android", "compose", "", "framework", "manifest", file, line)
		}
		b.dep(d, "maven", group+":"+artifact, version, role, "manifest", file, line)
	}
	for _, m := range gradlePluginRef.FindAllStringSubmatchIndex(text, -1) {
		line := strings.Count(text[:m[0]], "\n") + 1
		id, version := "", ""
		switch {
		case m[2] >= 0:
			if cat != nil {
				if p, ok := cat.plugins[catalogKey(text[m[2]:m[3]])]; ok {
					id, version = p[0], p[1]
				}
			}
			if id == "" {
				id = text[m[2]:m[3]]
			}
		case m[4] >= 0:
			id = text[m[4]:m[5]]
		case m[6] >= 0:
			id = "org.jetbrains.kotlin." + text[m[6]:m[7]]
		}
		lower := strings.ToLower(id)
		switch {
		case strings.Contains(lower, "compose"):
			b.dep(d, "android", "compose", version, "framework", "manifest", file, line)
		case strings.Contains(lower, "hilt"):
			b.dep(d, "maven", "com.google.dagger:hilt-android", version, "library", "manifest", file, line)
		case strings.Contains(lower, "multiplatform"):
			b.dep(d, "android", "kotlin-multiplatform", version, "framework", "manifest", file, line)
		}
	}
	if composeFeature.MatchString(text) {
		b.dep(d, "android", "compose", "", "framework", "manifest", file, lineOf(text, "compose"))
	}
	if !primary {
		return
	}
	for _, m := range gradleSdk.FindAllStringSubmatch(text, -1) {
		v := m[3]
		if v == "" && cat != nil {
			v = cat.versions[catalogKey(m[2])]
		}
		if v != "" {
			b.dep(d, "android", m[1], v, "runtime", "manifest", file, lineOf(text, m[0]))
		}
	}
}

// ---------------------------------------------------------------------------
// iOS: Xcode, SwiftPM, CocoaPods
// ---------------------------------------------------------------------------

var (
	swiftPackageURL = regexp.MustCompile(`\.package\(\s*(?:name:\s*"[^"]*",\s*)?url:\s*"([^"]+)"\s*,\s*(?:from:\s*|exact:\s*|\.upToNextMajor\(from:\s*|\.upToNextMinor\(from:\s*|branch:\s*|revision:\s*)?"([^"]*)"`)
	podLockEntry    = regexp.MustCompile(`(?m)^  - "?([A-Za-z0-9_.+-]+) \(([^)]+)\)"?:?$`)
	swiftTools      = regexp.MustCompile(`swift-tools-version:\s*([0-9.]+)`)
)

func packageName(url string) string {
	url = strings.TrimSuffix(strings.TrimSuffix(url, "/"), ".git")
	return url[strings.LastIndex(url, "/")+1:]
}

// resolvedPins reads every Package.resolved in the workspace: identity ->
// version.
func (b *builder) resolvedPins() map[string]string {
	if b.pins != nil {
		return b.pins
	}
	b.pins = map[string]string{}
	for f, content := range b.in.Files {
		if !strings.HasSuffix(f, "Package.resolved") {
			continue
		}
		var v2 struct {
			Pins []struct {
				Identity, Location string
				State              struct{ Version, Revision string }
			}
			Object struct {
				Pins []struct {
					Package, RepositoryURL string
					State                  struct{ Version string }
				}
			}
		}
		if json.Unmarshal(content, &v2) != nil {
			continue
		}
		for _, p := range v2.Pins {
			b.pins[strings.ToLower(packageName(p.Location))] = p.State.Version
			b.pins[strings.ToLower(p.Identity)] = p.State.Version
		}
		for _, p := range v2.Object.Pins {
			b.pins[strings.ToLower(packageName(p.RepositoryURL))] = p.State.Version
		}
	}
	return b.pins
}

func (b *builder) swiftPackageStack(d *Deployable, file string, content []byte) {
	text := string(content)
	pins := b.resolvedPins()
	for _, m := range swiftPackageURL.FindAllStringSubmatchIndex(text, -1) {
		url, version := text[m[2]:m[3]], text[m[4]:m[5]]
		name := packageName(url)
		source := "manifest"
		if v := pins[strings.ToLower(name)]; v != "" {
			version, source = v, "lockfile"
		}
		b.dep(d, "swiftpm", name, version, frameworkRole(name), source, file, strings.Count(text[:m[0]], "\n")+1)
	}
}

// xcodeStack reads an app target's build settings and the remote packages
// its project references.
func (b *builder) xcodeStack(d *Deployable, mod *module.Module, content []byte) {
	root, _ := module.ParseOldPlist(string(content)).(map[string]any)
	objects, _ := root["objects"].(map[string]any)
	if objects == nil {
		return
	}
	get := func(id any) map[string]any {
		s, _ := id.(string)
		o, _ := objects[s].(map[string]any)
		return o
	}
	str := func(o map[string]any, k string) string { s, _ := o[k].(string); return s }
	list := func(o map[string]any, k string) []any { l, _ := o[k].([]any); return l }
	var target map[string]any
	for _, v := range objects {
		if o, ok := v.(map[string]any); ok && str(o, "isa") == "PBXNativeTarget" && str(o, "name") == mod.Name {
			target = o
		}
	}
	if target == nil {
		return
	}
	settings := func(listID any) map[string]any {
		for _, c := range list(get(listID), "buildConfigurations") {
			cfg := get(c)
			if s, ok := cfg["buildSettings"].(map[string]any); ok && (str(cfg, "name") == "Release" || len(list(get(listID), "buildConfigurations")) == 1) {
				return s
			}
		}
		for _, c := range list(get(listID), "buildConfigurations") {
			if s, ok := get(c)["buildSettings"].(map[string]any); ok {
				return s
			}
		}
		return nil
	}
	own := settings(target["buildConfigurationList"])
	var project map[string]any
	for _, v := range objects {
		if o, ok := v.(map[string]any); ok && str(o, "isa") == "PBXProject" {
			project = settings(o["buildConfigurationList"])
		}
	}
	setting := func(k string) string {
		if v := str(own, k); v != "" {
			return v
		}
		return str(project, k)
	}
	if v := setting("IPHONEOS_DEPLOYMENT_TARGET"); v != "" {
		b.dep(d, "ios", "deployment-target", v, "runtime", "manifest", mod.Manifest, 0)
	}
	if v := setting("SWIFT_VERSION"); v != "" {
		b.dep(d, "ios", "swift", v, "runtime", "manifest", mod.Manifest, 0)
	}
	if v := setting("PRODUCT_BUNDLE_IDENTIFIER"); v != "" {
		d.aliases = appendUnique(d.aliases, v)
	}
	pins := b.resolvedPins()
	for _, p := range list(target, "packageProductDependencies") {
		dep := get(p)
		pkg := get(dep["package"])
		url := str(pkg, "repositoryURL")
		if url == "" {
			continue // a local package: carried as a module, read from its manifest
		}
		name := packageName(url)
		version, source := "", "manifest"
		if req, ok := pkg["requirement"].(map[string]any); ok {
			version = str(req, "minimumVersion") + str(req, "version")
		}
		if v := pins[strings.ToLower(name)]; v != "" {
			version, source = v, "lockfile"
		}
		b.dep(d, "swiftpm", name, version, frameworkRole(name), source, mod.Manifest, 0)
	}
	// Pods, from the lock beside the project.
	for dir := dirOf(mod.Manifest); ; dir = dirOf(dir) {
		if lock, ok := b.in.Files[joinPath(dir, "Podfile.lock")]; ok {
			text := strings.ReplaceAll(string(lock), "\r\n", "\n")
			if end := strings.Index(text, "\nDEPENDENCIES:"); end > 0 {
				text = text[:end]
			}
			for _, m := range podLockEntry.FindAllStringSubmatch(text, -1) {
				if !strings.Contains(m[1], "/") {
					b.dep(d, "cocoapods", m[1], m[2], frameworkRole(m[1]), "lockfile", joinPath(dir, "Podfile.lock"), lineOf(string(lock), "  - "+m[1]+" ("))
				}
			}
			break
		}
		if dir == "." || dir == "" {
			break
		}
	}
}

// frameworkRole marks the libraries that decide how an app is written.
func frameworkRole(name string) string {
	switch strings.ToLower(name) {
	case "swift-composable-architecture", "composablearchitecture", "rxswift", "react-native", "expo", "flutter":
		return "framework"
	}
	return "library"
}

// ---------------------------------------------------------------------------
// Flutter: pubspec and its lock
// ---------------------------------------------------------------------------

var (
	pubLockPackage = regexp.MustCompile(`(?m)^  ([A-Za-z0-9_]+):\n(?:    .*\n)*?    version: "([^"]+)"`)
	pubSdk         = regexp.MustCompile(`(?m)^\s+sdk:\s*["']?([^"'\n]+)`)
)

func (b *builder) pubStack(d *Deployable, mod *module.Module, content []byte, primary bool) {
	locked := map[string]string{}
	lockFile := joinPath(cleanDir(mod.Dir), "pubspec.lock")
	if lock, ok := b.in.Files[lockFile]; ok {
		// Written on Windows, a lock has CRLF line ends: immich's does.
		for _, m := range pubLockPackage.FindAllStringSubmatch(strings.ReplaceAll(string(lock), "\r\n", "\n"), -1) {
			locked[m[1]] = m[2]
		}
	}
	text := string(content)
	devAt := strings.Index(text, "\ndev_dependencies:")
	for _, name := range mod.DependsOn {
		if b.in.Modules.ByName(name) != nil {
			continue // internal, recorded as such
		}
		line := lineOf(text, "\n  "+name+":")
		if devAt >= 0 && strings.Index(text, "\n  "+name+":") > devAt {
			continue // a dev dependency does not ship
		}
		role := "library"
		if name == "flutter" {
			role = "framework"
		}
		version, source := locked[name], "lockfile"
		if version == "0.0.0" {
			// What pubspec.lock records for an SDK package.
			version = ""
		}
		if version == "" {
			source = "manifest"
		}
		b.dep(d, "pub", name, version, role, source, mod.Manifest, line+1)
	}
	if primary {
		if m := pubSdk.FindStringSubmatch(text); m != nil {
			b.dep(d, "runtime", "dart", strings.TrimSpace(m[1]), "runtime", "manifest", mod.Manifest, lineOf(text, m[0]))
		}
	}
}

func (b *builder) mobileStack(d *Deployable, mod *module.Module, primary bool) {
	content := b.in.Files[mod.Manifest]
	switch mod.Kind {
	case "gradle":
		b.gradleStack(d, mod.Manifest, content, primary)
	case "xcode":
		if primary {
			b.xcodeStack(d, mod, content)
		}
	case "swiftpm":
		b.swiftPackageStack(d, mod.Manifest, content)
		if primary {
			if m := swiftTools.FindStringSubmatch(string(content)); m != nil {
				b.dep(d, "ios", "swift-tools", m[1], "runtime", "manifest", mod.Manifest, 1)
			}
		}
	case "pub":
		b.pubStack(d, mod, content, primary)
	}
}
