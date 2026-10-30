package gradle

import (
	"path"
	"regexp"
	"strings"

	"github.com/archstats/archstats/extensions/cicd/buildtool"
)

// ---------------------------------------------------------------------------
// Gradle, by pattern: build scripts are programs, and only the shapes
// written the same way everywhere are read.
// ---------------------------------------------------------------------------

var (
	gradleBootPlugin = regexp.MustCompile(`id\s*\(?\s*["']org\.springframework\.boot["']|apply\s+plugin:\s*["']org\.springframework\.boot["']|alias\s*\(\s*libs\.plugins\.spring\.boot\s*\)`)
	gradleJibImage   = regexp.MustCompile(`(?s)jib\s*\{.*?to\s*\{.*?image\s*=\s*["']([^"']+)["']|jib\.to\.image\s*=\s*["']([^"']+)["']`)
	gradleJibPlugin  = regexp.MustCompile(`com\.google\.cloud\.tools\.jib`)
	gradleBootImage  = regexp.MustCompile(`(?s)bootBuildImage\b.*?imageName(?:\s*=\s*|\.set\(\s*)["']([^"']+)["']`)
	gradleJavaVer    = regexp.MustCompile(`(?:sourceCompatibility|targetCompatibility)\s*=\s*(?:JavaVersion\.VERSION_)?['"]?(1[._])?(\d+)|languageVersion(?:\.set\(|\s*=\s*)\s*JavaLanguageVersion\.of\((\d+)\)|jvmToolchain\((\d+)\)`)
)

type gradleProject struct {
	File        string
	Dir         string
	Name        string
	Executable  bool
	JavaVersion string
	Images      []*buildtool.Image
}

func readGradle(file string, content []byte, name string) *gradleProject {
	s := string(content)
	g := &gradleProject{File: file, Dir: buildtool.DirOf(file), Name: name}
	g.Executable = gradleBootPlugin.MatchString(s)
	if m := gradleJavaVer.FindStringSubmatch(s); m != nil {
		for _, v := range []string{m[2], m[3], m[4]} {
			if v != "" {
				g.JavaVersion = v
				break
			}
		}
	}
	image := ""
	builtBy := ""
	if m := gradleJibImage.FindStringSubmatch(s); m != nil {
		image, builtBy = m[1]+m[2], "jib"
	} else if gradleJibPlugin.MatchString(s) {
		image, builtBy = name, "jib"
	} else if m := gradleBootImage.FindStringSubmatch(s); m != nil {
		image, builtBy = m[1], "spring-boot"
	}
	if image != "" {
		image = strings.ReplaceAll(strings.ReplaceAll(image, "${project.name}", name), "$name", name)
		if n := buildtool.NormalizeImage(image); n.Repo != "" {
			ln := buildtool.LineOf(s, image)
			if ln == 0 {
				ln = 1
			}
			g.Images = append(g.Images, &buildtool.Image{Names: []buildtool.ImageName{n}, Context: g.Dir, Module: name, BuiltBy: builtBy, File: file, Line: ln})
		}
	}
	return g
}

// ---------------------------------------------------------------------------
// .NET project files and Aspire app hosts

// Tool reads Gradle build scripts: the images Jib and Spring Boot build, the
// Spring Boot applications, the Java toolchain, and the Android applications
// a store ships.
func Tool() buildtool.Tool { return tool{} }

type tool struct{}

func (tool) Name() string               { return "gradle" }
func (tool) Kind(string, []byte) string { return "" }

var (
	rootProjectName = regexp.MustCompile(`rootProject\.name\s*=\s*["']([^"']+)["']`)
	appID           = regexp.MustCompile(`\bapplicationId\s*=?\s*["']([A-Za-z0-9_.]+)["']`)
)

func (tool) Read(in buildtool.Input) buildtool.Facts {
	facts := buildtool.Facts{Runtimes: map[string]string{}, MobileModuleKinds: []string{"gradle"}}
	for _, f := range buildtool.SortedKeys(in.Files) {
		base := path.Base(f)
		if in.Kinds[f] != "build" || (base != "build.gradle" && base != "build.gradle.kts") {
			continue
		}
		dir := buildtool.DirOf(f)
		name := path.Base(dir)
		if dir == "." {
			name = ""
		}
		// The module map's name is the project path
		// (`feature:foryou:impl`); a deployable must name its module the
		// same way or the two never join.
		if in.Modules != nil {
			if mod := in.Modules.Of(f); mod != nil && mod.Dir == dir && mod.Kind == "gradle" {
				name = mod.Name
			}
		}
		if name == "" {
			continue
		}
		g := readGradle(f, in.Files[f], name)
		facts.Images = append(facts.Images, g.Images...)
		if g.JavaVersion != "" {
			facts.Runtimes[g.File] = "java " + g.JavaVersion
		}
		if g.Executable {
			app := g.Name
			// A single-project build is named by settings.gradle, not by the
			// folder it was cloned into.
			for _, settings := range []string{"settings.gradle", "settings.gradle.kts"} {
				if m := rootProjectName.FindSubmatch(in.Files[buildtool.JoinPath(g.Dir, settings)]); m != nil {
					app = string(m[1])
				}
			}
			facts.Apps = append(facts.Apps, buildtool.App{Name: app, Module: g.Name, Dir: g.Dir, File: g.File, Line: 1, BuiltBy: "gradle", NeedsDelivery: true})
		}
	}
	if in.Modules != nil {
		for _, mod := range in.Modules.Modules() {
			if mod.Kind == "gradle" && mod.Type == "android-application" {
				app := buildtool.MobileApp{Module: mod, Platform: "android", BuiltBy: "gradle"}
				if m := appID.FindSubmatch(in.Files[mod.Manifest]); m != nil {
					app.StoreID = string(m[1])
				}
				facts.Mobile = append(facts.Mobile, app)
			}
		}
	}
	return facts
}
