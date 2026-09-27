package deployables

import (
	"encoding/xml"
	"regexp"
	"sort"
	"strings"
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
	Images     []*imageBuild
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
	for _, f := range sortedKeys(files) {
		var raw pomXML
		if xml.Unmarshal(files[f], &raw) != nil || raw.ArtifactID == "" {
			continue
		}
		p := &pom{File: f, Dir: dirOf(f), raw: raw, content: string(files[f]), ArtifactID: raw.ArtifactID, Packaging: raw.Packaging}
		p.Line = lineOf(p.content, "<artifactId>"+raw.ArtifactID+"</artifactId>")
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
			candidate := joinPath(p.Dir, rel)
			if !strings.HasSuffix(candidate, ".xml") {
				candidate = joinPath(candidate, "pom.xml")
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
		"project.build.directory":   joinPath(p.Dir, "target"),
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
			Line: lineOf(p.content, "<artifactId>"+d.ArtifactID+"</artifactId>"),
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
					declared[pl.ArtifactID] = effectivePlugin{ArtifactID: pl.ArtifactID, File: c.File, Line: lineOf(c.content, "<artifactId>"+pl.ArtifactID+"</artifactId>")}
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

func (p *pom) imageFromPlugin(pl effectivePlugin) *imageBuild {
	cfg := pl.Config
	b := &imageBuild{Context: p.Dir, Module: p.ArtifactID, File: pl.File, Line: pl.Line}
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
		b.Dockerfile = joinPath(p.Dir, "Dockerfile")
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
		b.Dockerfile = joinPath(workdir, dockerfile)
	default:
		return nil
	}
	name = p.interp(strings.TrimSpace(name))
	n := NormalizeImage(name)
	if n.Repo == "" {
		return nil
	}
	b.Names = []ImageName{n}
	return b
}

// dirValue turns a property value that is a directory (`${basedir}/../docker`
// after interpolation) into a clean repository-relative path.
func (p *pom) dirValue(v string) string {
	if v == "" || strings.Contains(v, "${") {
		return ""
	}
	return joinPath(".", v)
}

// lineOf is the 1-based line of the first occurrence of needle, 0 if absent.
func lineOf(content, needle string) int {
	i := strings.Index(content, needle)
	if i < 0 {
		return 0
	}
	return strings.Count(content[:i], "\n") + 1
}

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
	Images      []*imageBuild
}

func readGradle(file string, content []byte, name string) *gradleProject {
	s := string(content)
	g := &gradleProject{File: file, Dir: dirOf(file), Name: name}
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
		if n := NormalizeImage(image); n.Repo != "" {
			ln := lineOf(s, image)
			if ln == 0 {
				ln = 1
			}
			g.Images = append(g.Images, &imageBuild{Names: []ImageName{n}, Context: g.Dir, Module: name, BuiltBy: builtBy, File: file, Line: ln})
		}
	}
	return g
}

// ---------------------------------------------------------------------------
// .NET project files and Aspire app hosts
// ---------------------------------------------------------------------------

var (
	csprojSdk        = regexp.MustCompile(`<Project\s+Sdk\s*=\s*"([^"]+)"`)
	csprojOutputType = regexp.MustCompile(`<OutputType>\s*(\w+)\s*</OutputType>`)
	csprojTarget     = regexp.MustCompile(`<TargetFrameworks?>\s*([^<;]+)`)
	csprojContainer  = regexp.MustCompile(`<ContainerRepository>\s*([^<]+?)\s*</ContainerRepository>`)
	csprojAspireHost = regexp.MustCompile(`<IsAspireHost>\s*true\s*</IsAspireHost>|Aspire\.AppHost\.Sdk`)
	csprojPackage    = regexp.MustCompile(`<PackageReference\s+Include\s*=\s*"([^"]+)"(?:\s+Version\s*=\s*"([^"]*)")?`)
)

type dotnetProject struct {
	File            string
	Dir             string
	Name            string
	Sdk             string
	Web             bool
	AspireHost      bool
	TargetFramework string
	Images          []*imageBuild
	Packages        []resolvedDep
}

func readCsproj(file string, content []byte) *dotnetProject {
	s := string(content)
	base := file[strings.LastIndex(file, "/")+1:]
	name := strings.TrimSuffix(base, base[strings.LastIndex(base, "."):])
	d := &dotnetProject{File: file, Dir: dirOf(file), Name: name}
	if m := csprojSdk.FindStringSubmatch(s); m != nil {
		d.Sdk = m[1]
	}
	d.Web = strings.HasPrefix(d.Sdk, "Microsoft.NET.Sdk.Web") || strings.HasPrefix(d.Sdk, "Microsoft.NET.Sdk.Worker") || strings.HasPrefix(d.Sdk, "Microsoft.NET.Sdk.BlazorWebAssembly")
	d.AspireHost = csprojAspireHost.MatchString(s)
	if m := csprojTarget.FindStringSubmatch(s); m != nil {
		d.TargetFramework = strings.TrimSpace(m[1])
	}
	if m := csprojContainer.FindStringSubmatch(s); m != nil {
		if n := NormalizeImage(m[1]); n.Repo != "" {
			d.Images = append(d.Images, &imageBuild{Names: []ImageName{n}, Context: d.Dir, Module: name, BuiltBy: "dotnet-publish", File: file, Line: lineOf(s, m[0])})
		}
	}
	for _, m := range csprojPackage.FindAllStringSubmatch(s, -1) {
		d.Packages = append(d.Packages, resolvedDep{ArtifactID: m[1], Version: m[2], Line: lineOf(s, m[0])})
	}
	return d
}

// An aspireResource is one thing an Aspire app host adds: a project, a
// container, a database, a broker.
type aspireResource struct {
	Var     string
	Name    string
	Kind    string // Project, Redis, Postgres, Database, RabbitMQ, Container, ...
	Project string // Projects.Catalog_API -> Catalog.API
	Parent  string // for a database: the server's variable
	Line    int
	Refs    []aspireRef
}

type aspireRef struct {
	Var  string
	Line int
}

var (
	aspireAdd     = regexp.MustCompile(`(?:^|[^\w])(\w+)?\s*\.\s*Add(\w+)\s*(?:<\s*Projects\.(\w+)\s*>)?\s*\(\s*"([^"]+)"`)
	aspireVarDecl = regexp.MustCompile(`^\s*(?:var|[A-Z]\w*(?:<[^>]+>)?)\s+(\w+)\s*=\s*`)
	// WithReference and an endpoint passed through WithEnvironment are how a
	// project is told where something is. WaitFor is start-up order (eShop
	// waits on ordering-api because it runs the migrations) and
	// WithParentRelationship only groups resources in the dashboard; neither
	// says one thing talks to another.
	aspireRefCall  = regexp.MustCompile(`\.WithReference\s*\(\s*(\w+)|\.WithEnvironment\s*\(\s*"[^"]*"\s*,\s*(\w+)\s*\)`)
	aspireEndpoint = regexp.MustCompile(`^\s*var\s+(\w+)\s*=\s*(\w+)\s*\.\s*GetEndpoint\s*\(`)
	lineComment    = regexp.MustCompile(`//[^\n]*`)
)

// readAspireHost reads an Aspire app host's Program.cs for what it composes:
// eShop has no Dockerfile, no compose file and no manifest, and its whole
// topology -- 20 resources, 14 references -- is written in this one file.
func readAspireHost(file string, content []byte) []*aspireResource {
	src := string(content)
	// Blank out comments without moving any line.
	src = lineComment.ReplaceAllStringFunc(src, func(m string) string { return strings.Repeat(" ", len(m)) })
	var out []*aspireResource
	byVar := map[string]*aspireResource{}
	alias := map[string]string{}
	offset := 0
	for _, stmt := range strings.Split(src, ";") {
		stmtLine := strings.Count(src[:offset], "\n") + 1
		offset += len(stmt) + 1
		lead := len(stmt) - len(strings.TrimLeft(stmt, " \t\r\n"))
		stmtLine += strings.Count(stmt[:lead], "\n")
		body := strings.TrimSpace(stmt)
		if m := aspireEndpoint.FindStringSubmatch(body); m != nil {
			alias[m[1]] = m[2]
			continue
		}
		adds := aspireAdd.FindAllStringSubmatchIndex(body, -1)
		if len(adds) == 0 {
			continue
		}
		m := adds[0]
		sub := func(i int) string {
			if m[2*i] < 0 {
				return ""
			}
			return body[m[2*i]:m[2*i+1]]
		}
		kind := sub(2)
		switch kind {
		case "Parameter", "ConnectionString", "AzureContainerAppEnvironment", "ForwardedHeaders", "ServiceDefaults":
			continue
		}
		r := &aspireResource{Kind: kind, Name: sub(4), Line: stmtLine + strings.Count(body[:m[0]], "\n")}
		if p := sub(3); p != "" {
			r.Project = strings.ReplaceAll(p, "_", ".")
		}
		if owner := sub(1); owner != "" && owner != "builder" {
			r.Parent = owner
		}
		if d := aspireVarDecl.FindStringSubmatch(body); d != nil {
			r.Var = d[1]
		}
		for _, rm := range aspireRefCall.FindAllStringSubmatchIndex(body, -1) {
			v := ""
			for g := 1; g <= 2; g++ {
				if rm[2*g] >= 0 {
					v = body[rm[2*g]:rm[2*g+1]]
				}
			}
			if v != "" {
				r.Refs = append(r.Refs, aspireRef{Var: v, Line: stmtLine + strings.Count(body[:rm[0]], "\n")})
			}
		}
		out = append(out, r)
		if r.Var != "" {
			byVar[r.Var] = r
		}
	}
	// A reference to an endpoint is a reference to the project behind it.
	for _, r := range out {
		for i, ref := range r.Refs {
			if a, ok := alias[ref.Var]; ok {
				r.Refs[i].Var = a
			}
		}
		sort.SliceStable(r.Refs, func(i, j int) bool { return r.Refs[i].Line < r.Refs[j].Line })
	}
	return out
}
