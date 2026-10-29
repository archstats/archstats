// Package buildtool is the contract between the CI/CD extension and the
// build tools it knows through plug-ins: what a tool's manifests say they
// build. cicd itself knows no build tool; the engine hands it one Tool per
// ecosystem -- Maven, Gradle, .NET, npm, Xcode, pub, Go -- each in a package
// of its own (ADR 0025).
package buildtool

import (
	stdpath "path"
	"sort"
	"strings"

	"github.com/archstats/archstats/core/module"
)

// A Tool reads one build tool's manifests for what they build: the images
// its plugins push, the applications it packages, the apps it ships to a
// store or a desktop, and the runtime it targets.
type Tool interface {
	// Name identifies the tool.
	Name() string
	// Kind names a file this tool must read that no system kind keeps -- an
	// Aspire app host's Program.cs -- and "" for any other file.
	Kind(path string, content []byte) string
	// Read reads the tool's files out of everything the extension kept.
	Read(in Input) Facts
}

// Input is everything the extension kept, for a tool to read its own files
// from.
type Input struct {
	// Kept files by clean repository-relative path ("a/b", "x" at the root).
	Files map[string][]byte
	// Each kept file's kind: its system kind, or a tool's own.
	Kinds   map[string]string
	Modules *module.Map
}

// Facts are what one tool's manifests declare.
type Facts struct {
	Images  []*Image
	Apps    []App
	Mobile  []MobileApp
	Desktop []DesktopApp
	// Runtimes is the runtime each manifest declares -- "java 17", "dotnet
	// 8.0", "go 1.23" -- by the manifest's path.
	Runtimes map[string]string
	// MobileModuleKinds are the module kinds that belong to a mobile app
	// when they sit inside its folder: an Android app's Gradle modules, an
	// iOS app's Xcode targets and Swift packages.
	MobileModuleKinds []string
}

// An Image is a declaration that an image is built: a compose service with
// `build:`, a Skaffold artifact, a docker step in a pipeline, a Jib or Spring
// Boot image plugin, or a Dockerfile standing on its own.
type Image struct {
	Names   []ImageName
	Aliases []string // other names it answers to: a compose service name
	// The build context and the Dockerfile inside it. Dockerfile is empty
	// for builds that use none (Jib, buildpacks, Spring Boot build-image).
	Context    string
	Dockerfile string
	Target     string
	// For Jib and Spring Boot: the module the image is built from.
	Module  string
	BuiltBy string
	File    string
	Line    int
	// A compose service that names both `build:` and `image:` in one block
	// states the join outright.
	Declared bool
	// The workload this build belongs to (compose), if any.
	Workload string
	// The pipeline job and step that build it, for a pipeline build.
	Job  string
	Step int
	// Tags as written, for the push-only-latest flag.
	RawTags []string
}

// An App is an application a build tool packages to run: a Spring Boot jar,
// an ASP.NET project, a project an Aspire host composes.
type App struct {
	// What it is called where it runs, and the module that builds it.
	Name, Module string
	Dir, File    string
	Line         int
	BuiltBy      string
	// Packaged to run, but an app only where its repository delivers
	// something: a pipeline or a deployment. An Aspire host's projects are
	// delivered by the host.
	NeedsDelivery bool
}

// A MobileApp is a module that ships to a store.
type MobileApp struct {
	Module   *module.Module
	Platform string
	BuiltBy  string
	// Its native projects (android/, ios/) are part of it, not apps of
	// their own: a Flutter or React Native app.
	CrossPlatform bool
	// What the store knows it as, where the build says: applicationId.
	StoreID string
}

// A DesktopApp is a module that ships as a desktop application.
type DesktopApp struct {
	Names    []string
	Dir      string
	File     string
	Platform string
}

// ImageName is an image reference reduced to what can be joined on: the
// last path segment, lower-cased, without registry, tag or digest.
type ImageName struct {
	// "librechat-api" for ghcr.io/${{ github.repository_owner }}/librechat-api:latest.
	Repo string
	// The tag, when it is written out: "latest-cart" for the OTel demo, which
	// names all its images `demo` and tells them apart by tag.
	Tag string
}

// String writes the name back the short way: repo, or repo:tag.
func (n ImageName) String() string {
	if n.Tag == "" {
		return n.Repo
	}
	return n.Repo + ":" + n.Tag
}

// IsInterpolated reports whether a value is filled in by a template or a
// shell, so names nothing the files can know.
func IsInterpolated(s string) bool {
	return strings.Contains(s, "${") || strings.Contains(s, "{{") || strings.Contains(s, "$(") ||
		strings.HasPrefix(s, "$") || strings.Contains(s, "<") || strings.Contains(s, "%")
}

// NormalizeImage reduces an image reference. It returns an empty Repo when
// the name itself is interpolated: europe-west4-docker.pkg.dev/${p}/repo/${IMAGE}
// names no image that can be known from the files, and must not become "repo".
func NormalizeImage(ref string) ImageName {
	ref = strings.TrimSpace(ref)
	ref = strings.Trim(ref, `"'`)
	ref = strings.TrimPrefix(ref, "docker://")
	if i := strings.Index(ref, "@sha256:"); i >= 0 {
		ref = ref[:i]
	} else if i := strings.Index(ref, "@"); i >= 0 && !strings.Contains(ref[i:], "/") {
		ref = ref[:i]
	}
	segs := strings.Split(ref, "/")
	last := segs[len(segs)-1]
	repo, tag, _ := strings.Cut(last, ":")
	if repo == "" || IsInterpolated(repo) {
		return ImageName{}
	}
	if IsInterpolated(tag) {
		tag = ""
	}
	return ImageName{Repo: strings.ToLower(repo), Tag: strings.ToLower(tag)}
}

// LineOf is the 1-based line of the first occurrence of needle, 0 if absent.
func LineOf(content, needle string) int {
	i := strings.Index(content, needle)
	if i < 0 {
		return 0
	}
	return strings.Count(content[:i], "\n") + 1
}

// DirOf is a clean path's directory, "." at the root.
func DirOf(p string) string {
	d := stdpath.Dir(p)
	if d == "" {
		return "."
	}
	return d
}

// JoinPath joins a path written in a file onto the directory the file sits
// in, and gives "" for one that leaves the checkout.
func JoinPath(dir, p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return dir
	}
	if strings.HasPrefix(p, "/") {
		// Absolute paths in a repository file are not about this checkout.
		return ""
	}
	j := stdpath.Clean(stdpath.Join(dir, p))
	if j == ".." || strings.HasPrefix(j, "../") {
		return ""
	}
	return j
}

// HasDep reports whether a module declares a dependency on name.
func HasDep(mod *module.Module, name string) bool {
	for _, d := range mod.DependsOn {
		if d == name {
			return true
		}
	}
	return false
}

// SortedKeys lists a map's keys in order, so a tool reads its files the same
// way on every scan.
func SortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
