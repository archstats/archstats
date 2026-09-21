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

// `project(":exposed-core")` and `projects.exposedCore` both name a sibling.
var gradleProjectDep = regexp.MustCompile(`project\(["']:([A-Za-z0-9_.:-]+)["']\)`)

func (gradleReader) Read(root, path string) []*Module {
	raw, ok := readFile(path)
	if !ok {
		return nil
	}
	dir := relDir(root, path)
	name := dir
	if idx := strings.LastIndex(dir, "/"); idx != -1 {
		name = dir[idx+1:]
	}
	if name == "" {
		// The root build script of a multi-module build configures the build
		// itself rather than declaring a module anybody depends on.
		return nil
	}
	var deps []string
	for _, m := range gradleProjectDep.FindAllStringSubmatch(raw, -1) {
		last := m[1]
		if idx := strings.LastIndex(last, ":"); idx != -1 {
			last = last[idx+1:]
		}
		deps = appendUnique(deps, last)
	}
	return []*Module{{
		Name:      name,
		Dir:       dir,
		Manifest:  relFile(root, path),
		DependsOn: deps,
	}}
}
