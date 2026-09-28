package module

import (
	"encoding/xml"
	"path/filepath"
	"regexp"
	"strings"
)

// ---------------------------------------------------------------------------
// dotnet: C#
//
// The `.csproj` is .NET's real module, and in nopCommerce it is the whole
// architecture: 40 projects named `Nop.Core`, `Nop.Services`, `Nop.Data`,
// `Nop.Web`, and `Nop.Plugin.Payments.PayPal` and thirty more like it. The
// namespaces do not say which of those is a plugin. The project references
// do, and they are what makes "core must never depend on a plugin"
// checkable.
// ---------------------------------------------------------------------------

type dotnetReader struct{}

func (dotnetReader) Kind() string { return "dotnet" }
func (dotnetReader) Claims(b string) bool {
	switch strings.ToLower(filepath.Ext(b)) {
	case ".csproj", ".fsproj", ".vbproj":
		return true
	}
	return false
}

type msbuildProject struct {
	ItemGroups []struct {
		ProjectReference []struct {
			Include string `xml:"Include,attr"`
		} `xml:"ProjectReference"`
		PackageReference []struct {
			Include string `xml:"Include,attr"`
		} `xml:"PackageReference"`
	} `xml:"ItemGroup"`
}

func (dotnetReader) Read(root, path string) []*Module {
	raw, ok := readFile(path)
	if !ok {
		return nil
	}
	name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	if name == "" {
		return nil
	}
	var proj msbuildProject
	var deps []string
	// A malformed or unusual project file still declares a module by its own
	// name; only its dependencies are lost.
	if xml.Unmarshal([]byte(raw), &proj) == nil {
		for _, group := range proj.ItemGroups {
			for _, ref := range group.ProjectReference {
				// `..\Nop.Core\Nop.Core.csproj` names the project, not a path
				// anybody downstream cares about.
				p := strings.ReplaceAll(ref.Include, `\`, "/")
				deps = appendUnique(deps, strings.TrimSuffix(filepath.Base(p), filepath.Ext(p)))
			}
			for _, ref := range group.PackageReference {
				deps = appendUnique(deps, ref.Include)
			}
		}
	}
	return []*Module{{
		Name:      name,
		Dir:       relDir(root, path),
		Manifest:  relFile(root, path),
		DependsOn: deps,
	}}
}

// ---------------------------------------------------------------------------
// maven: Java
// ---------------------------------------------------------------------------

type mavenReader struct{}

func (mavenReader) Kind() string         { return "maven" }
func (mavenReader) Claims(b string) bool { return b == "pom.xml" }

type pom struct {
	ArtifactId   string `xml:"artifactId"`
	Dependencies struct {
		Dependency []struct {
			ArtifactId string `xml:"artifactId"`
		} `xml:"dependency"`
	} `xml:"dependencies"`
}

func (mavenReader) Read(root, path string) []*Module {
	raw, ok := readFile(path)
	if !ok {
		return nil
	}
	var p pom
	if xml.Unmarshal([]byte(raw), &p) != nil || p.ArtifactId == "" {
		return nil
	}
	var deps []string
	for _, d := range p.Dependencies.Dependency {
		if d.ArtifactId != "" {
			deps = appendUnique(deps, d.ArtifactId)
		}
	}
	return []*Module{{
		Name:      p.ArtifactId,
		Dir:       relDir(root, path),
		Manifest:  relFile(root, path),
		DependsOn: deps,
	}}
}

// ---------------------------------------------------------------------------
// gradle: Kotlin and Java
//
// A gradle build script does not name itself. Exposed has 50 of them and the
// name of each module is the directory it sits in, which is also how
// `include(":exposed-core")` in settings.gradle.kts refers to it.
// ---------------------------------------------------------------------------

type gradleReader struct{}

func (gradleReader) Kind() string { return "gradle" }
func (gradleReader) Claims(b string) bool {
	return b == "build.gradle" || b == "build.gradle.kts"
}

// `project(":core:data")`, in Kotlin or Groovy, with or without parentheses
// around the call that holds it.
var gradleProjectDep = regexp.MustCompile(`project\(\s*(?:path\s*[:=]\s*)?["']:([A-Za-z0-9_.:-]+)["']`)

// `projects.core.dataTest`: a type-safe project accessor, which spells each
// path segment in camel case. Resolved against the modules once all are read.
var gradleAccessorDep = regexp.MustCompile(`\bprojects\.([A-Za-z0-9_]+(?:\.[A-Za-z0-9_]+)*)`)

// A Gradle module is named by its project path -- `feature:foryou:impl` --
// which is the directory under the build's settings file with colons for
// slashes. Named by its last directory instead, nowinandroid's feature
// modules were all "impl" and "api".
func (gradleReader) Read(root, path string) []*Module {
	raw, ok := readFile(path)
	if !ok {
		return nil
	}
	dir := relDir(root, path)
	name := gradleProjectPath(root, dir)
	if name == "" {
		// The root build script of a multi-module build configures the build
		// itself rather than declaring a module anybody depends on.
		return nil
	}
	var deps []string
	for _, m := range gradleProjectDep.FindAllStringSubmatch(raw, -1) {
		deps = appendUnique(deps, strings.Trim(m[1], ":"))
	}
	for _, m := range gradleAccessorDep.FindAllStringSubmatch(raw, -1) {
		// `projects.core.data.dependencyProject` and `.path` are properties of
		// the accessor, not segments of the path.
		segs := strings.Split(m[1], ".")
		for len(segs) > 1 && (segs[len(segs)-1] == "path" || segs[len(segs)-1] == "dependencyProject") {
			segs = segs[:len(segs)-1]
		}
		deps = appendUnique(deps, gradleAccessorPrefix+strings.Join(segs, ":"))
	}
	return []*Module{{
		Name:      name,
		Dir:       dir,
		Manifest:  relFile(root, path),
		DependsOn: deps,
		Type:      gradleModuleType(raw),
	}}
}

// gradleProjectPath is a module's path within its build: the directory
// relative to the nearest settings file above it, colon-separated. An
// included build (build-logic) has its own settings file, so its modules are
// named within it. With no settings file anywhere, the repo root stands in.
func gradleProjectPath(root, dir string) string {
	base := ""
	for d := dir; d != ""; {
		idx := strings.LastIndex(d, "/")
		parent := ""
		if idx >= 0 {
			parent = d[:idx]
		}
		if parent != "" && hasGradleSettings(filepath.Join(root, filepath.FromSlash(parent))) {
			base = parent
			break
		}
		d = parent
	}
	rel := strings.TrimPrefix(strings.TrimPrefix(dir, base), "/")
	return strings.ReplaceAll(rel, "/", ":")
}

func hasGradleSettings(dir string) bool {
	for _, f := range []string{"settings.gradle.kts", "settings.gradle"} {
		if _, ok := readFile(filepath.Join(dir, f)); ok {
			return true
		}
	}
	return false
}

// Written before resolution so the accessor and a literal path are never
// confused; stripped by resolveGradleAccessors.
const gradleAccessorPrefix = "\x00accessor:"

var gradlePluginsBlock = regexp.MustCompile(`(?s)\bplugins\s*\{(.*?)\n\s*\}|\bplugins\s*\{([^\n}]*)\}`)
var gradleApplyPlugin = regexp.MustCompile(`apply\s*\(?\s*plugin\s*[:=]\s*["']([^"']+)["']`)

// gradleModuleType reads what a module builds from the plugins it applies.
// Convention plugins carry the platform in their names -- nowinandroid's
// `nowinandroid.android.feature.impl`, Tivi's `app.tivi.android.application`
// -- so the ids are matched on their words, not on a list of exact ids.
func gradleModuleType(raw string) string {
	var ids []string
	for _, m := range gradlePluginsBlock.FindAllStringSubmatch(raw, -1) {
		ids = append(ids, m[1]+m[2])
	}
	for _, m := range gradleApplyPlugin.FindAllStringSubmatch(raw, -1) {
		ids = append(ids, m[1])
	}
	text := strings.ToLower(strings.Join(ids, "\n"))
	text = strings.NewReplacer("-", ".", "_", ".", `("`, ".", `")`, "", " ", "").Replace(text)
	has := func(words ...string) bool {
		for _, w := range words {
			if strings.Contains(text, w) {
				return true
			}
		}
		return false
	}
	switch {
	case has("multiplatform"):
		return "kotlin-multiplatform"
	case has("android.application", "android.app\n", "android.app)"):
		return "android-application"
	case has("dynamic.feature"):
		return "android-dynamic-feature"
	case has("com.android.test", "android.test\n", "android.test)"):
		return "android-test"
	case has("android.library", "android.feature", "android.lib"):
		return "android-library"
	case has("`kotlin.dsl`", "kotlin.dsl", "java.gradle.plugin"):
		return "build-logic"
	case has("application"):
		return "jvm-application"
	case has("java.library", "kotlin.jvm", "kotlin(\"jvm", "kotlin.jvm", "`java`", "java\n"):
		return "jvm-library"
	}
	return ""
}

// resolveGradleAccessors turns each `projects.core.dataTest` a module declared
// into the module it means. An accessor spells a path segment in camel case
// and a directory is often kebab case, so both are compared with the case and
// the separators taken out; an accessor that matches no module of this build
// keeps its colon form.
func resolveGradleAccessors(m *Map) {
	norm := func(s string) string {
		return strings.ToLower(strings.NewReplacer("-", "", "_", "", ".", "").Replace(s))
	}
	byNorm := map[string]string{}
	for _, mod := range m.modules {
		if mod.Kind == "gradle" {
			if _, taken := byNorm[norm(mod.Name)]; !taken {
				byNorm[norm(mod.Name)] = mod.Name
			}
		}
	}
	for _, mod := range m.modules {
		for i, dep := range mod.DependsOn {
			if !strings.HasPrefix(dep, gradleAccessorPrefix) {
				continue
			}
			path := strings.TrimPrefix(dep, gradleAccessorPrefix)
			if name, ok := byNorm[norm(path)]; ok {
				path = name
			}
			mod.DependsOn[i] = path
		}
		// An accessor and a project() call can name the same module.
		var uniq []string
		for _, d := range mod.DependsOn {
			uniq = appendUnique(uniq, d)
		}
		mod.DependsOn = uniq
	}
}
