package maven

import (
	"encoding/xml"
	"path"
	"regexp"
	"strings"

	"github.com/archstats/archstats/extensions/cicd/buildtool"
)

// Java and .NET projects often build their images without a Dockerfile in
// sight, or with one Dockerfile shared by every module and driven from the
// build tool. spring-petclinic-microservices is the case in point: each
// module declares exec-maven-plugin in a `buildDocker` profile, the root pom
// configures it in that profile's pluginManagement, and the image name is
// `${docker.image.prefix}/${project.artifactId}` -- a property of the root,
// resolved in the child. Read naively, none of its 8 images join to anything.

// ---------------------------------------------------------------------------
// Maven
// ---------------------------------------------------------------------------

type pomXML struct {
	GroupID    string `xml:"groupId"`
	ArtifactID string `xml:"artifactId"`
	Version    string `xml:"version"`
	Packaging  string `xml:"packaging"`
	Parent     struct {
		GroupID      string  `xml:"groupId"`
		ArtifactID   string  `xml:"artifactId"`
		Version      string  `xml:"version"`
		RelativePath *string `xml:"relativePath"`
	} `xml:"parent"`
	Properties   xmlProperties `xml:"properties"`
	Modules      []string      `xml:"modules>module"`
	Dependencies []pomDep      `xml:"dependencies>dependency"`
	Managed      []pomDep      `xml:"dependencyManagement>dependencies>dependency"`
	Build        pomBuild      `xml:"build"`
	Profiles     []struct {
		ID           string        `xml:"id"`
		Build        pomBuild      `xml:"build"`
		Properties   xmlProperties `xml:"properties"`
		Dependencies []pomDep      `xml:"dependencies>dependency"`
	} `xml:"profiles>profile"`
}

type xmlProperties struct {
	Entries []struct {
		XMLName xml.Name
		Value   string `xml:",chardata"`
	} `xml:",any"`
}

type pomDep struct {
	GroupID    string `xml:"groupId"`
	ArtifactID string `xml:"artifactId"`
	Version    string `xml:"version"`
	Scope      string `xml:"scope"`
	Type       string `xml:"type"`
}

type pomBuild struct {
	FinalName        string      `xml:"finalName"`
	Plugins          []pomPlugin `xml:"plugins>plugin"`
	PluginManagement []pomPlugin `xml:"pluginManagement>plugins>plugin"`
}

type pomPlugin struct {
	GroupID       string   `xml:"groupId"`
	ArtifactID    string   `xml:"artifactId"`
	Version       string   `xml:"version"`
	Configuration innerXML `xml:"configuration"`
	Executions    []struct {
		Configuration innerXML `xml:"configuration"`
	} `xml:"executions>execution"`
}

type innerXML struct {
	Inner string `xml:",innerxml"`
}

func (p pomPlugin) config() string {
	var b strings.Builder
	b.WriteString(p.Configuration.Inner)
	for _, e := range p.Executions {
		b.WriteString("\n")
		b.WriteString(e.Configuration.Inner)
	}
	return b.String()
}

// A pom is a pom.xml with its parent chain resolved: properties inherited
// and interpolated, and the plugins it builds with.
type pom struct {
	File       string
	Dir        string
	raw        pomXML
	content    string
	parent     *pom
	GroupID    string
	ArtifactID string
	Version    string
	Packaging  string
	Props      map[string]string
	// Declared dependencies with versions resolved where the pom or its
	// parents say them.
	Deps []resolvedDep
	// The version of Spring Boot the pom inherits, "" when none.
	SpringBoot  string
	JavaVersion string
	// Executable: a Spring Boot application, not a library.
	Executable bool
	Images     []*buildtool.Image
	Line       int
}

type resolvedDep struct {
	GroupID, ArtifactID, Version, Scope string
	Line                                int
}

// readPoms parses every pom and resolves each against its parents.
func readPoms(files map[string][]byte) []*pom {
	var poms []*pom
	byCoord := map[string]*pom{}
	byPath := map[string]*pom{}
	for _, f := range buildtool.SortedKeys(files) {
		var raw pomXML
		if xml.Unmarshal(files[f], &raw) != nil || raw.ArtifactID == "" {
			continue
		}
		p := &pom{File: f, Dir: buildtool.DirOf(f), raw: raw, content: string(files[f]), ArtifactID: raw.ArtifactID, Packaging: raw.Packaging}
		p.Line = buildtool.LineOf(p.content, "<artifactId>"+raw.ArtifactID+"</artifactId>")
		poms = append(poms, p)
		byPath[f] = p
		g := raw.GroupID
		if g == "" {
			g = raw.Parent.GroupID
		}
		byCoord[g+":"+raw.ArtifactID] = p
		byCoord[":"+raw.ArtifactID] = p
	}
	for _, p := range poms {
		if p.raw.Parent.ArtifactID == "" {
			continue
		}
		rel := "../pom.xml"
		if p.raw.Parent.RelativePath != nil {
			rel = strings.TrimSpace(*p.raw.Parent.RelativePath)
		}
		if rel != "" {
			candidate := buildtool.JoinPath(p.Dir, rel)
			if !strings.HasSuffix(candidate, ".xml") {
				candidate = buildtool.JoinPath(candidate, "pom.xml")
			}
			if par := byPath[candidate]; par != nil && par.ArtifactID == p.raw.Parent.ArtifactID {
				p.parent = par
				continue
			}
		}
		if par := byCoord[p.raw.Parent.GroupID+":"+p.raw.Parent.ArtifactID]; par != nil && par != p {
			p.parent = par
		}
	}
	for _, p := range poms {
		p.resolve()
	}
	return poms
}

// ancestors lists the pom's parents, nearest first, stopping at a cycle.
func (p *pom) ancestors() []*pom {
	var out []*pom
	seen := map[*pom]bool{p: true}
	for a := p.parent; a != nil && !seen[a]; a = a.parent {
		seen[a] = true
		out = append(out, a)
	}
	return out
}

var propRef = regexp.MustCompile(`\$\{([^}]+)\}`)

func (p *pom) resolve() {
	chain := append([]*pom{p}, p.ancestors()...)
	props := map[string]string{}
	// Farthest ancestor first, so nearer poms override.
	for i := len(chain) - 1; i >= 0; i-- {
		for _, e := range chain[i].raw.Properties.Entries {
			props[e.XMLName.Local] = strings.TrimSpace(e.Value)
		}
		for _, prof := range chain[i].raw.Profiles {
			for _, e := range prof.Properties.Entries {
				if _, set := props[e.XMLName.Local]; !set {
					props[e.XMLName.Local] = strings.TrimSpace(e.Value)
				}
			}
		}
	}
	p.GroupID = p.raw.GroupID
	if p.GroupID == "" {
		p.GroupID = p.raw.Parent.GroupID
	}
	p.Version = p.raw.Version
	if p.Version == "" {
		p.Version = p.raw.Parent.Version
	}
	finalName := p.raw.Build.FinalName
	for _, a := range p.ancestors() {
		if finalName == "" {
			finalName = a.raw.Build.FinalName
		}
	}
	if finalName == "" {
		finalName = "${project.artifactId}-${project.version}"
	}
	builtins := map[string]string{
		"project.artifactId": p.ArtifactID, "artifactId": p.ArtifactID, "pom.artifactId": p.ArtifactID,
		"project.groupId": p.GroupID, "groupId": p.GroupID,
		"project.version": p.Version, "version": p.Version,
		"project.basedir": p.Dir, "basedir": p.Dir,
		"project.build.directory":   buildtool.JoinPath(p.Dir, "target"),
		"project.parent.version":    p.raw.Parent.Version,
		"project.parent.artifactId": p.raw.Parent.ArtifactID,
	}
	for k, v := range builtins {
		props[k] = v
	}
	props["project.build.finalName"] = finalName
	p.Props = props
	p.Props["project.build.finalName"] = p.interp(finalName)

	if p.raw.Parent.ArtifactID == "spring-boot-starter-parent" || p.raw.Parent.ArtifactID == "spring-boot-parent" {
		p.SpringBoot = p.raw.Parent.Version
	}
	for _, a := range p.ancestors() {
		if p.SpringBoot == "" {
			p.SpringBoot = a.SpringBoot
		}
	}
	for _, d := range p.allManaged() {
		if p.SpringBoot == "" && d.ArtifactID == "spring-boot-dependencies" {
			p.SpringBoot = p.interp(d.Version)
		}
	}
	for _, k := range []string{"java.version", "maven.compiler.release", "maven.compiler.target", "maven.compiler.source", "java.release"} {
		if v := p.interp(props[k]); v != "" && !strings.Contains(v, "${") {
			p.JavaVersion = strings.TrimPrefix(v, "1.")
			break
		}
	}

	managed := map[string]string{}
	for _, d := range p.allManaged() {
		managed[d.GroupID+":"+d.ArtifactID] = d.Version
	}
	for _, d := range p.raw.Dependencies {
		v := d.Version
		if v == "" {
			v = managed[d.GroupID+":"+d.ArtifactID]
		}
		p.Deps = append(p.Deps, resolvedDep{
			GroupID: p.interp(d.GroupID), ArtifactID: p.interp(d.ArtifactID), Version: cleanVersion(p.interp(v)), Scope: d.Scope,
			Line: buildtool.LineOf(p.content, "<artifactId>"+d.ArtifactID+"</artifactId>"),
		})
	}

	for _, pl := range p.effectivePlugins() {
		switch pl.ArtifactID {
		case "spring-boot-maven-plugin":
			if p.Packaging == "" || p.Packaging == "jar" || p.Packaging == "war" {
				p.Executable = true
			}
		}
		if b := p.imageFromPlugin(pl); b != nil {
			p.Images = append(p.Images, b)
		}
	}
	// A version managed through a property is a version all the same.
	if strings.Contains(p.SpringBoot, "${") {
		p.SpringBoot = ""
	}
}

func cleanVersion(v string) string {
	if strings.Contains(v, "${") {
		return ""
	}
	return v
}

func (p *pom) allManaged() []pomDep {
	var out []pomDep
	for _, a := range append([]*pom{p}, p.ancestors()...) {
		out = append(out, a.raw.Managed...)
	}
	return out
}

func (p *pom) interp(s string) string {
	for i := 0; i < 8 && strings.Contains(s, "${"); i++ {
		next := propRef.ReplaceAllStringFunc(s, func(m string) string {
			k := m[2 : len(m)-1]
			if v, ok := p.Props[k]; ok {
				return v
			}
			return m
		})
		if next == s {
			break
		}
		s = next
	}
	return s
}

// A plugin as the module sees it: declared by the module or inherited from a
// parent's plugins, with any configuration a parent gives it through
// pluginManagement appended.
type effectivePlugin struct {
	ArtifactID string
	Config     string
	File       string
	Line       int
}

func (p *pom) effectivePlugins() []effectivePlugin {
	chain := append([]*pom{p}, p.ancestors()...)
	declared := map[string]effectivePlugin{}
	var order []string
	for _, c := range chain {
		builds := []pomBuild{c.raw.Build}
		for _, prof := range c.raw.Profiles {
			builds = append(builds, prof.Build)
		}
		for _, b := range builds {
			for _, pl := range b.Plugins {
				if _, ok := declared[pl.ArtifactID]; !ok {
					order = append(order, pl.ArtifactID)
					declared[pl.ArtifactID] = effectivePlugin{ArtifactID: pl.ArtifactID, File: c.File, Line: buildtool.LineOf(c.content, "<artifactId>"+pl.ArtifactID+"</artifactId>")}
				}
				e := declared[pl.ArtifactID]
				e.Config += "\n" + pl.config()
				declared[pl.ArtifactID] = e
			}
		}
	}
	for _, c := range chain {
		builds := []pomBuild{c.raw.Build}
		for _, prof := range c.raw.Profiles {
			builds = append(builds, prof.Build)
		}
		for _, b := range builds {
			for _, pl := range b.PluginManagement {
				if e, ok := declared[pl.ArtifactID]; ok {
					e.Config += "\n" + pl.config()
					declared[pl.ArtifactID] = e
				}
			}
		}
	}
	out := make([]effectivePlugin, 0, len(order))
	for _, id := range order {
		out = append(out, declared[id])
	}
	return out
}

var (
	jibToImage     = regexp.MustCompile(`(?s)<to>.*?<image>([^<]+)</image>`)
	bootImageName  = regexp.MustCompile(`(?s)<image>\s*(?:<[^>]+>[^<]*</[^>]+>\s*)*?<name>([^<]+)</name>`)
	fabric8Image   = regexp.MustCompile(`(?s)<images>.*?<name>([^<]+)</name>`)
	spotifyRepo    = regexp.MustCompile(`<repository>([^<]+)</repository>`)
	spotifyName    = regexp.MustCompile(`<imageName>([^<]+)</imageName>`)
	execArgument   = regexp.MustCompile(`<argument>([^<]*)</argument>`)
	execExecutable = regexp.MustCompile(`<executable>([^<]*)</executable>`)
	execWorkdir    = regexp.MustCompile(`<workingDirectory>([^<]*)</workingDirectory>`)
)

func (p *pom) imageFromPlugin(pl effectivePlugin) *buildtool.Image {
	cfg := pl.Config
	b := &buildtool.Image{Context: p.Dir, Module: p.ArtifactID, File: pl.File, Line: pl.Line}
	name := ""
	switch pl.ArtifactID {
	case "jib-maven-plugin":
		b.BuiltBy = "jib"
		if m := jibToImage.FindStringSubmatch(cfg); m != nil {
			name = m[1]
		} else {
			name = p.ArtifactID
		}
	case "spring-boot-maven-plugin":
		// Every Spring Boot application carries this plugin to repackage
		// its jar. It builds an image only when configured to.
		m := bootImageName.FindStringSubmatch(cfg)
		if m == nil {
			return nil
		}
		b.BuiltBy = "spring-boot"
		name = m[1]
	case "docker-maven-plugin":
		b.BuiltBy = "maven-docker"
		if m := fabric8Image.FindStringSubmatch(cfg); m != nil {
			name = m[1]
		} else if m := spotifyName.FindStringSubmatch(cfg); m != nil {
			name = m[1]
		}
	case "dockerfile-maven-plugin":
		b.BuiltBy = "maven-docker"
		if m := spotifyRepo.FindStringSubmatch(cfg); m != nil {
			name = m[1]
		}
		b.Dockerfile = buildtool.JoinPath(p.Dir, "Dockerfile")
	case "exec-maven-plugin":
		exe := ""
		if m := execExecutable.FindStringSubmatch(cfg); m != nil {
			exe = strings.ToLower(p.interp(m[1]))
		}
		var args []string
		for _, m := range execArgument.FindAllStringSubmatch(cfg, -1) {
			args = append(args, p.interp(strings.TrimSpace(m[1])))
		}
		isDocker := strings.Contains(exe, "docker") || strings.Contains(exe, "podman") || strings.Contains(exe, "container")
		hasBuild := false
		for _, a := range args {
			if a == "build" || a == "buildx" {
				hasBuild = true
			}
		}
		if !isDocker || !hasBuild {
			return nil
		}
		b.BuiltBy = "maven-docker"
		workdir := p.Dir
		if m := execWorkdir.FindStringSubmatch(cfg); m != nil {
			if w := p.dirValue(p.interp(m[1])); w != "" {
				workdir = w
			}
		}
		dockerfile, context := "Dockerfile", ""
		for i := 0; i < len(args); i++ {
			switch args[i] {
			case "-t", "--tag":
				if i+1 < len(args) {
					name = args[i+1]
					i++
				}
			case "-f", "--file":
				if i+1 < len(args) {
					dockerfile = args[i+1]
					i++
				}
			case "--build-arg", "--platform", "--target", "--label":
				i++
			default:
				if !strings.HasPrefix(args[i], "-") && args[i] != "build" && args[i] != "buildx" && args[i] != "" && !strings.Contains(args[i], "${") {
					context = args[i]
				}
			}
		}
		if d := p.dirValue(context); d != "" {
			b.Context = d
		} else {
			b.Context = workdir
		}
		b.Dockerfile = buildtool.JoinPath(workdir, dockerfile)
	default:
		return nil
	}
	name = p.interp(strings.TrimSpace(name))
	n := buildtool.NormalizeImage(name)
	if n.Repo == "" {
		return nil
	}
	b.Names = []buildtool.ImageName{n}
	return b
}

// dirValue turns a property value that is a directory (`${basedir}/../docker`
// after interpolation) into a clean repository-relative path.
func (p *pom) dirValue(v string) string {
	if v == "" || strings.Contains(v, "${") {
		return ""
	}
	return buildtool.JoinPath(".", v)
}

// Tool reads Maven poms: the images Jib and Spring Boot plugins build, the
// Spring Boot applications they package, and the Java version they target.
func Tool() buildtool.Tool { return tool{} }

type tool struct{}

func (tool) Name() string               { return "maven" }
func (tool) Kind(string, []byte) string { return "" }
func (tool) Read(in buildtool.Input) buildtool.Facts {
	files := map[string][]byte{}
	for f, content := range in.Files {
		if in.Kinds[f] == "build" && path.Base(f) == "pom.xml" {
			files[f] = content
		}
	}
	facts := buildtool.Facts{Runtimes: map[string]string{}}
	for _, p := range readPoms(files) {
		facts.Images = append(facts.Images, p.Images...)
		if p.Executable {
			facts.Apps = append(facts.Apps, buildtool.App{Name: p.ArtifactID, Module: p.ArtifactID, Dir: p.Dir, File: p.File, Line: p.Line, BuiltBy: "maven", NeedsDelivery: true})
		}
		if p.JavaVersion != "" {
			facts.Runtimes[p.File] = "java " + p.JavaVersion
		}
	}
	return facts
}
