package dotnet

import (
	"bytes"
	"path"
	"regexp"
	"sort"
	"strings"

	"github.com/archstats/archstats/extensions/cicd/buildtool"
)

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
	Images          []*buildtool.Image
	Packages        []resolvedDep
}

func readCsproj(file string, content []byte) *dotnetProject {
	s := string(content)
	base := file[strings.LastIndex(file, "/")+1:]
	name := strings.TrimSuffix(base, base[strings.LastIndex(base, "."):])
	d := &dotnetProject{File: file, Dir: buildtool.DirOf(file), Name: name}
	if m := csprojSdk.FindStringSubmatch(s); m != nil {
		d.Sdk = m[1]
	}
	d.Web = strings.HasPrefix(d.Sdk, "Microsoft.NET.Sdk.Web") || strings.HasPrefix(d.Sdk, "Microsoft.NET.Sdk.Worker") || strings.HasPrefix(d.Sdk, "Microsoft.NET.Sdk.BlazorWebAssembly")
	d.AspireHost = csprojAspireHost.MatchString(s)
	if m := csprojTarget.FindStringSubmatch(s); m != nil {
		d.TargetFramework = strings.TrimSpace(m[1])
	}
	if m := csprojContainer.FindStringSubmatch(s); m != nil {
		if n := buildtool.NormalizeImage(m[1]); n.Repo != "" {
			d.Images = append(d.Images, &buildtool.Image{Names: []buildtool.ImageName{n}, Context: d.Dir, Module: name, BuiltBy: "dotnet-publish", File: file, Line: buildtool.LineOf(s, m[0])})
		}
	}
	for _, m := range csprojPackage.FindAllStringSubmatch(s, -1) {
		d.Packages = append(d.Packages, resolvedDep{ArtifactID: m[1], Version: m[2], Line: buildtool.LineOf(s, m[0])})
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

// A resolvedDep is a package a project references, at the version it pins.
type resolvedDep struct {
	ArtifactID, Version string
	Line                int
}

// Tool reads .NET projects and Aspire app hosts: the images `dotnet publish`
// builds, the web and worker projects that run, the projects an Aspire host
// composes, and the target framework.
func Tool() buildtool.Tool { return tool{} }

type tool struct{}

func (tool) Name() string { return "dotnet" }

var aspireMarker = []byte("DistributedApplication.CreateBuilder")

// Kind is "aspire" for an app host's Program.cs, which no system kind keeps.
func (tool) Kind(p string, content []byte) string {
	if strings.HasSuffix(strings.ToLower(p), ".cs") && bytes.Contains(content, aspireMarker) {
		return "aspire"
	}
	return ""
}

func (tool) Read(in buildtool.Input) buildtool.Facts {
	facts := buildtool.Facts{Runtimes: map[string]string{}}
	var projects []*dotnetProject
	aspireProjects := map[string]*aspireResource{}
	for _, f := range buildtool.SortedKeys(in.Files) {
		base := path.Base(f)
		switch {
		case in.Kinds[f] == "build" && (strings.HasSuffix(base, ".csproj") || strings.HasSuffix(base, ".fsproj") || strings.HasSuffix(base, ".vbproj")):
			projects = append(projects, readCsproj(f, in.Files[f]))
		case in.Kinds[f] == "aspire":
			for _, r := range readAspireHost(f, in.Files[f]) {
				if r.Kind == "Project" && r.Project != "" {
					aspireProjects[strings.ToLower(r.Project)] = r
				}
			}
		}
	}
	for _, c := range projects {
		facts.Images = append(facts.Images, c.Images...)
		if c.TargetFramework != "" {
			facts.Runtimes[c.File] = "dotnet " + strings.TrimPrefix(c.TargetFramework, "net")
		}
		if c.AspireHost {
			continue
		}
		switch r := aspireProjects[strings.ToLower(c.Name)]; {
		case r != nil:
			facts.Apps = append(facts.Apps, buildtool.App{Name: r.Name, Module: c.Name, Dir: c.Dir, File: c.File, Line: 1, BuiltBy: "aspire"})
		case c.Web:
			facts.Apps = append(facts.Apps, buildtool.App{Name: c.Name, Module: c.Name, Dir: c.Dir, File: c.File, Line: 1, BuiltBy: "dotnet", NeedsDelivery: true})
		}
	}
	return facts
}
