package deployables

import (
	stdpath "path"
	"regexp"
	"sort"
	"strings"

	"github.com/archstats/archstats/core/module"
)

// Input is what the linker needs from a scan. Paths are clean and
// repository-relative ("a/b/c", never "./c").
type Input struct {
	// The content of every file that could matter: system files, and app
	// hosts written in code.
	Files map[string][]byte
	// The system kind of each of those files.
	Kinds map[string]string
	// Every walked file, with its role and component.
	AllFiles   []string
	Roles      map[string]string
	Components map[string]string
	Modules    *module.Map
	// RepoOf names the repository a path belongs to.
	RepoOf func(string) string
}

// The rows the extension writes.
type (
	Deployable struct {
		ID, Name, Kind, Repository string
		File                       string
		Line                       int
		BuiltBy                    string
		Context                    string
		BaseImage                  string
		Runtime                    string
		// For a mobile app: android, ios, flutter or react-native.
		Platform          string
		Files, Components int

		names      []ImageName
		aliases    []string
		dockerfile string
		target     string
		contexts   []string
		modules    []string
		builds     []*imageBuild
		function   *function
		priority   int
	}
	Content struct {
		Deployable, Path, Pattern, Module, File string
		Line                                    int
		Resolution                              string
	}
	ComponentRow struct {
		Deployable, Component string
		Files                 int
	}
	Link struct {
		From, To, ToKind, Kind, Mode, Via, File string
		Line                                    int
		Resolution                              string
	}
	Unresolved struct {
		From, Ref, File string
		Line            int
		Reason          string
	}
	Pipeline struct {
		ID, Name, System, File, Repository string
		Parsed                             string
		Triggers, Paths, Stages, Tools     string
		DelegatesTo, DelegatesRef          string
		Environments                       string
		Deployables                        int
	}
	PipelineDeployable struct {
		Pipeline, Deployable, Action, File string
		Line                               int
		Resolution                         string
	}
	Environment struct {
		Deployable, Environment, Kind, Source, File string
		Line                                        int
	}
	EnvValue struct {
		Deployable, Environment, Source, Key, Value, File string
		Line                                              int
		Secret                                            int
	}
	Dependency struct {
		Deployable, Ecosystem, Name, Version, Role, Source, File string
		Line                                                     int
	}
)

// Model is everything the linker found.
type Model struct {
	Deployables         []*Deployable
	Contents            []*Content
	Components          []*ComponentRow
	Links               []*Link
	Unresolved          []*Unresolved
	Pipelines           []*Pipeline
	PipelineDeployables []*PipelineDeployable
	Environments        []*Environment
	EnvValues           []*EnvValue
	Dependencies        []*Dependency
}

type builder struct {
	in *Input
	m  *Model

	// Mobile: version catalogs by file, and Package.resolved pins.
	catalogs map[string]*catalog
	pins     map[string]string

	dockerfiles map[string]*Dockerfile
	composes    []string
	workloads   []*workload
	builds      []*imageBuild
	services    []*k8sService
	configMaps  map[string][]configEntry
	kustomizes  []*kustomization
	charts      []*chart
	values      []*valuesFile
	functions   []*function
	argo        []*argoApp
	poms        []*pom
	gradles     []*gradleProject
	csprojs     []*dotnetProject
	aspire      map[string][]*aspireResource // app host file -> resources
	configFiles []string
	pipelines   []*pipelineFact

	pendingContents map[*Deployable][]*Content
	owned           []ownedEntry

	byID        map[string]*Deployable
	buildOf     map[*imageBuild]*Deployable
	runs        map[string][]string // workload key -> deployable ids
	workloadsBy map[string]*workload
	allFileSet  map[string]bool
	dirSet      map[string]bool
	appNames    map[string][]string // spring.application.name -> deployables
	chartDeps   map[string][]string // chart dir -> deployables
	valuesOwner map[string][]string // values file -> deployables
}

// Build links a scan's system files into deployables.
func Build(in *Input) *Model {
	b := &builder{
		in: in, m: &Model{},
		dockerfiles: map[string]*Dockerfile{}, configMaps: map[string][]configEntry{},
		aspire: map[string][]*aspireResource{}, byID: map[string]*Deployable{},
		buildOf: map[*imageBuild]*Deployable{}, runs: map[string][]string{},
		workloadsBy: map[string]*workload{}, allFileSet: map[string]bool{}, dirSet: map[string]bool{},
		appNames: map[string][]string{}, chartDeps: map[string][]string{}, valuesOwner: map[string][]string{},
	}
	if in.RepoOf == nil {
		in.RepoOf = func(string) string { return "" }
	}
	if in.Modules == nil {
		in.Modules = module.New()
	}
	for _, f := range in.AllFiles {
		b.allFileSet[f] = true
		for d := dirOf(f); ; d = dirOf(d) {
			if b.dirSet[d] {
				break
			}
			b.dirSet[d] = true
			if d == "." {
				break
			}
		}
	}
	b.read()
	b.imageDeployables()
	b.functionDeployables()
	b.contents()
	b.appDeployables()
	b.mobileDeployables()
	b.finishNames()
	b.moduleExpansion()
	b.joinWorkloads()
	b.readConfigOwners()
	b.links()
	b.pipelineRows()
	b.environments()
	b.dependencies()
	b.rollups()
	b.sortAll()
	return b.m
}

// skipped reports files that describe no deployable of this workspace:
// tests, other people's code, generated files, dev containers.
func (b *builder) skipped(f string) bool {
	switch b.in.Roles[f] {
	case "test", "third_party", "generated":
		return true
	}
	return strings.HasPrefix(f, ".devcontainer/") || strings.Contains(f, "/.devcontainer/")
}

var (
	rootProjectName = regexp.MustCompile(`rootProject\.name\s*=\s*["']([^"']+)["']`)
	composeName     = regexp.MustCompile(`^(docker-)?compose([._-][^/]*)?\.ya?ml$`)
	k8sHead         = regexp.MustCompile(`(?m)^apiVersion:\s*\S+`)
	k8sKindHead     = regexp.MustCompile(`(?m)^kind:\s*[A-Z]`)
	composeHead     = regexp.MustCompile(`(?m)^services:\s*$`)
	skaffoldHead    = regexp.MustCompile(`(?m)^apiVersion:\s*skaffold/`)
	samHead         = regexp.MustCompile(`(?m)^\s*"?Transform"?\s*:.*AWS::Serverless|AWS::Lambda::Function`)
	templatedYAML   = regexp.MustCompile(`\{\{[-\s]`)
)

func (b *builder) read() {
	files := sortedKeys(b.in.Files)
	pomFiles := map[string][]byte{}
	for _, f := range files {
		kind := b.in.Kinds[f]
		content := b.in.Files[f]
		base := strings.ToLower(stdpath.Base(f))
		if kind == "ci" {
			switch ciSystemOf(f) {
			case "github_actions":
				b.pipelines = append(b.pipelines, readGitHubWorkflow(f, content))
			case "jenkins":
				b.pipelines = append(b.pipelines, readJenkinsfile(f, content))
			case "gitlab":
				b.pipelines = append(b.pipelines, readGitLabCI(f, content))
			case "github_action":
				// A composite action is part of the pipelines that use it.
			default:
				b.pipelines = append(b.pipelines, readOtherCI(f, ciSystemOf(f)))
			}
			continue
		}
		if b.skipped(f) {
			continue
		}
		switch kind {
		case "container":
			b.dockerfiles[f] = ParseDockerfile(content)
		case "deploy", "infra":
			switch {
			case composeName.MatchString(base) || composeHead.Match(content) && !k8sHead.Match(content) && isComposeShaped(content):
				b.composes = append(b.composes, f)
			case base == "chart.yaml" || base == "chart.yml":
				if c := readChart(f, content); c != nil {
					b.charts = append(b.charts, c)
				}
			case strings.HasPrefix(base, "kustomization"):
				if k := readKustomization(f, content); k != nil {
					b.kustomizes = append(b.kustomizes, k)
				}
			case skaffoldHead.Match(content):
				b.builds = append(b.builds, readSkaffold(f, content)...)
			case base == "serverless.yml" || base == "serverless.yaml":
				b.functions = append(b.functions, readServerless(f, content)...)
			case samHead.Match(content):
				b.functions = append(b.functions, readSAM(f, content)...)
			case k8sHead.Match(content) && k8sKindHead.Match(content) && !templatedYAML.Match(content):
				k := readKubernetes(f, content)
				b.workloads = append(b.workloads, k.Workloads...)
				b.services = append(b.services, k.Services...)
				for _, cm := range k.ConfigMaps {
					b.configMaps[cm.Name] = append(b.configMaps[cm.Name], cm.Data...)
				}
				b.argo = append(b.argo, k.Argo...)
			case (strings.HasSuffix(base, ".yaml") || strings.HasSuffix(base, ".yml")) && !templatedYAML.Match(content) && !strings.Contains(f, "/templates/"):
				b.values = append(b.values, readValues(f, content))
			}
		case "build":
			switch {
			case base == "pom.xml":
				pomFiles[f] = content
			case base == "build.gradle" || base == "build.gradle.kts":
				name := stdpath.Base(dirOf(f))
				if dirOf(f) == "." {
					name = ""
				}
				// The module map's name is the project path
				// (`feature:foryou:impl`); a deployable must name its module
				// the same way or the two never join.
				if b.in.Modules != nil {
					if mod := b.in.Modules.Of(f); mod != nil && mod.Dir == dirOf(f) && mod.Kind == "gradle" {
						name = mod.Name
					}
				}
				if name != "" {
					b.gradles = append(b.gradles, readGradle(f, content, name))
				}
			case strings.HasSuffix(base, ".csproj") || strings.HasSuffix(base, ".fsproj") || strings.HasSuffix(base, ".vbproj"):
				b.csprojs = append(b.csprojs, readCsproj(f, content))
			}
		case "config":
			b.configFiles = append(b.configFiles, f)
		case "aspire":
			b.aspire[f] = readAspireHost(f, content)
		}
	}
	b.poms = readPoms(pomFiles)
	for _, p := range b.poms {
		b.builds = append(b.builds, p.Images...)
	}
	for _, g := range b.gradles {
		b.builds = append(b.builds, g.Images...)
	}
	for _, c := range b.csprojs {
		b.builds = append(b.builds, c.Images...)
	}
	for _, f := range b.composes {
		vars := map[string]string{}
		if env, ok := b.in.Files[joinPath(dirOf(f), ".env")]; ok {
			vars = dotenvMap(parseDotenv(env))
		}
		ws, bs := readCompose(f, b.in.Files[f], vars)
		b.workloads = append(b.workloads, ws...)
		b.builds = append(b.builds, bs...)
	}
	for _, k := range b.kustomizes {
		for name, entries := range k.ConfigMap {
			b.configMaps[name] = append(b.configMaps[name], entries...)
		}
	}
	for _, p := range b.pipelines {
		for _, bd := range p.Builds {
			if bd.Context != "" || bd.BuiltBy == "jib" {
				b.builds = append(b.builds, bd)
			}
		}
	}
	for _, w := range b.workloads {
		b.workloadsBy[w.Key] = w
	}
}

// isComposeShaped says whether a file named like anything is a compose file:
// `services` must be a mapping of services that run or build an image. A
// Helm values file has a top-level `services:` too -- the client's 74
// values files all do -- as a list of ports.
func isComposeShaped(content []byte) bool {
	docs := yamlDocs(content)
	if len(docs) == 0 {
		return false
	}
	for _, p := range pairs(get(docs[0], "services")) {
		if get(p.Value, "image") != nil || get(p.Value, "build") != nil {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Deployables
// ---------------------------------------------------------------------------

// Where a name came from decides which name a deployable goes by: what a
// Skaffold or pipeline build calls the image beats what a compose file calls
// the service, which beats the folder a Dockerfile sits in.
var builtByPriority = map[string]int{
	"skaffold": 1, "jib": 1, "buildpacks": 1, "ko": 1, "bazel": 1,
	"pipeline": 2, "maven-docker": 3, "spring-boot": 3, "dotnet-publish": 3,
	"compose": 4, "dockerfile": 6,
}

func (b *builder) exists(p string) bool {
	return b.allFileSet[p] || b.dirSet[p]
}

func (b *builder) imageDeployables() {
	type key struct{ df, target, ctx, module string }
	groups := map[key]*Deployable{}
	var order []key
	referenced := map[string]bool{}
	// A module is named by artifactId in a pom and by its path in Skaffold's
	// jib.project; both mean the same module.
	for _, bd := range b.builds {
		if bd.Module == "" || b.in.Modules.ByName(bd.Module) != nil {
			continue
		}
		if dir := joinPath(bd.Context, bd.Module); dir != "" {
			if mod := b.in.Modules.Of(dir + "/x"); mod != nil && cleanDir(mod.Dir) == dir {
				bd.Module = mod.Name
			}
		}
	}
	for _, bd := range b.builds {
		if (bd.BuiltBy == "jib" || bd.BuiltBy == "skaffold") && bd.Context == "" {
			// `mvn jib:build` or `skaffold build` in a pipeline builds what
			// the Jib modules or Skaffold config declare; linked with the
			// pipeline, not a build of its own.
			continue
		}
		target := strings.ToLower(bd.Target)
		if df := b.dockerfiles[bd.Dockerfile]; df != nil && target == "" && len(df.Stages) > 0 {
			// No --target builds the last stage: the same image as a build
			// that names that stage. LibreChat's build.yml and its release
			// workflows build one image under two names.
			target = df.Stages[len(df.Stages)-1].Name
		}
		k := key{df: bd.Dockerfile, target: target, ctx: bd.Context}
		if bd.Dockerfile == "" {
			// Jib and buildpacks build a module, wherever the declaring file
			// sets its context: bank-of-anthos declares each Jib image in
			// Skaffold (context ../../../) and in the module's own pom.
			k.module = bd.Module
			if bd.Module != "" {
				k.ctx = ""
			}
		} else {
			if _, ok := b.dockerfiles[bd.Dockerfile]; !ok {
				reason := "not_found"
				if !b.exists(bd.Dockerfile) && !b.allFileSet[bd.Dockerfile] {
					reason = "not_found"
				}
				b.unresolved("", bd.Dockerfile, bd.File, bd.Line, reason)
				continue
			}
			referenced[bd.Dockerfile] = true
		}
		d := groups[k]
		if d == nil {
			d = &Deployable{Kind: "image", dockerfile: bd.Dockerfile, target: bd.Target, priority: 99}
			groups[k] = d
			order = append(order, k)
		}
		d.builds = append(d.builds, bd)
		b.buildOf[bd] = d
	}
	// A Dockerfile nothing refers to is still an image this workspace
	// builds, known only by where it sits.
	for _, f := range sortedKeys(b.dockerfiles) {
		if referenced[f] || len(b.dockerfiles[f].Stages) == 0 {
			continue
		}
		bd := &imageBuild{Dockerfile: f, Context: dirOf(f), BuiltBy: "dockerfile", File: f, Line: b.dockerfiles[f].Stages[0].FromLine}
		k := key{df: f, ctx: dirOf(f)}
		d := &Deployable{Kind: "image", dockerfile: f, priority: 99}
		d.builds = append(d.builds, bd)
		groups[k] = d
		order = append(order, k)
		b.buildOf[bd] = d
	}
	sort.SliceStable(order, func(i, j int) bool {
		a, c := order[i], order[j]
		if a.df != c.df {
			return a.df < c.df
		}
		if a.target != c.target {
			return a.target < c.target
		}
		if a.ctx != c.ctx {
			return a.ctx < c.ctx
		}
		return a.module < c.module
	})
	for _, k := range order {
		d := groups[k]
		sort.SliceStable(d.builds, func(i, j int) bool {
			pi, pj := builtByPriority[d.builds[i].BuiltBy], builtByPriority[d.builds[j].BuiltBy]
			if pi != pj {
				return pi < pj
			}
			if d.builds[i].File != d.builds[j].File {
				return d.builds[i].File < d.builds[j].File
			}
			return d.builds[i].Line < d.builds[j].Line
		})
		first := d.builds[0]
		d.BuiltBy = first.BuiltBy
		d.priority = builtByPriority[first.BuiltBy]
		if k.df != "" {
			d.File, d.Line = k.df, 1
			if df := b.dockerfiles[k.df]; df != nil && len(df.Stages) > 0 {
				d.Line = df.Stages[0].FromLine
			}
		} else {
			d.File, d.Line = first.File, first.Line
		}
		for _, bd := range d.builds {
			for _, n := range bd.Names {
				if n.Repo != "" && !containsName(d.names, n) {
					d.names = append(d.names, n)
				}
			}
			for _, a := range bd.Aliases {
				d.aliases = appendUnique(d.aliases, a)
			}
			if bd.Context != "" {
				d.contexts = appendUnique(d.contexts, bd.Context)
			}
			if bd.Module != "" {
				d.modules = appendUnique(d.modules, bd.Module)
			}
		}
		d.Context = k.ctx
		b.m.Deployables = append(b.m.Deployables, d)
	}
}

func containsName(ns []ImageName, n ImageName) bool {
	for _, x := range ns {
		if x == n {
			return true
		}
	}
	return false
}

func appendUnique(xs []string, x string) []string {
	for _, y := range xs {
		if y == x {
			return xs
		}
	}
	return append(xs, x)
}

func (b *builder) functionDeployables() {
	for _, f := range b.functions {
		d := &Deployable{Kind: "function", BuiltBy: f.BuiltBy, File: f.File, Line: f.Line, Context: f.CodeDir, function: f, priority: 1}
		d.aliases = []string{f.Name}
		d.Runtime = functionRuntime(f.Runtime)
		b.m.Deployables = append(b.m.Deployables, d)
	}
}

var runtimeName = regexp.MustCompile(`^(nodejs|python|java|dotnet|go|ruby|provided)(\d[\d.]*)?`)

func functionRuntime(rt string) string {
	m := runtimeName.FindStringSubmatch(strings.ToLower(rt))
	if m == nil {
		return rt
	}
	tool := strings.TrimSuffix(m[1], "js")
	if tool == "dotnet" || tool == "provided" {
		return strings.TrimSpace(tool + " " + m[2])
	}
	return strings.TrimSpace(tool + " " + m[2])
}

// ---------------------------------------------------------------------------
// What goes into each image
// ---------------------------------------------------------------------------

var buildOutputSegment = map[string]bool{"target": true, "build": true, "dist": true, "out": true, "bin": true, "obj": true, "libs": true, "publish": true}

func (b *builder) addContent(d *Deployable, c *Content) {
	c.Deployable = ""
	for _, have := range b.pendingContents[d] {
		if have.Path == c.Path && have.Pattern == c.Pattern {
			return
		}
	}
	b.pendingContents[d] = append(b.pendingContents[d], c)
}

func (b *builder) contents() {
	b.pendingContents = map[*Deployable][]*Content{}
	for _, d := range b.m.Deployables {
		if d.function != nil {
			b.addContent(d, &Content{Path: d.function.CodeDir, File: d.function.File, Line: d.function.Line, Resolution: "declared"})
			continue
		}
		for _, bd := range d.builds {
			if bd.Dockerfile == "" {
				dir := bd.Context
				if bd.Module != "" {
					if mod := b.in.Modules.ByName(bd.Module); mod != nil {
						dir = cleanDir(mod.Dir)
					}
				}
				b.addContent(d, &Content{Path: dir, Module: bd.Module, File: bd.File, Line: bd.Line, Resolution: "declared"})
				continue
			}
			df := b.dockerfiles[bd.Dockerfile]
			target := df.Target(bd.Target)
			if target == nil {
				b.unresolved("", bd.Dockerfile+" --target "+bd.Target, bd.File, bd.Line, "not_found")
				continue
			}
			if d.BaseImage == "" {
				d.BaseImage, _ = df.BaseImage(target)
			}
			for _, src := range df.ImageContents(target) {
				p := joinPath(bd.Context, src.Path)
				if src.Path == "." {
					p = bd.Context
				}
				if p == "" {
					continue
				}
				c := &Content{Path: p, Pattern: src.Pattern, File: bd.Dockerfile, Line: src.Line, Resolution: "path"}
				// A jar or a dist folder is what a build made of a module;
				// the module is what the image holds.
				// Build output is what the build tree does not hold: target/
				// and dist/ are ignored by git, while a folder of CMake
				// scripts called build/ is source like any other.
				if src.Pattern == "" && !b.exists(p) {
					if !isBuildOutput(p) {
						continue
					}
					mod := b.moduleOwning(p)
					if mod == nil || cleanDir(mod.Dir) == "." {
						// A jar the root module builds says nothing narrower
						// than the whole repository; leave it out.
						continue
					}
					c.Path, c.Pattern, c.Module, c.Resolution = cleanDir(mod.Dir), "", mod.Name, "build_output"
				}
				b.addContent(d, c)
			}
			// Built by a module's own plugin: the module is in it whatever
			// the Dockerfile copies.
			if bd.Module != "" {
				if mod := b.in.Modules.ByName(bd.Module); mod != nil {
					b.addContent(d, &Content{Path: cleanDir(mod.Dir), Module: mod.Name, File: bd.File, Line: bd.Line, Resolution: "declared"})
				}
			}
		}
	}
}

func cleanDir(d string) string {
	if d == "" {
		return "."
	}
	return d
}

func isBuildOutput(p string) bool {
	for _, seg := range strings.Split(p, "/") {
		if buildOutputSegment[seg] {
			return true
		}
	}
	return strings.HasSuffix(p, ".jar") || strings.HasSuffix(p, ".war") || strings.Contains(p, "${")
}

// moduleOwning is the module whose directory holds p, longest first.
func (b *builder) moduleOwning(p string) *module.Module {
	if mod := b.in.Modules.Of(p); mod != nil {
		return mod
	}
	return b.in.Modules.Of(p + "/x")
}

// ---------------------------------------------------------------------------
// Executable modules no image holds
// ---------------------------------------------------------------------------

func (b *builder) contained(dir string) bool {
	for _, d := range b.m.Deployables {
		for _, c := range b.pendingContents[d] {
			if c.Pattern == "" && (c.Path == "." || c.Path == dir || strings.HasPrefix(dir, c.Path+"/")) {
				return true
			}
		}
	}
	return false
}

func (b *builder) repoHasDelivery(file string) bool {
	repo := b.in.RepoOf(file)
	for f, k := range b.in.Kinds {
		if (k == "deploy" || k == "ci") && b.in.RepoOf(f) == repo {
			return true
		}
	}
	return false
}

func (b *builder) appDeployables() {
	type app struct {
		name, module, dir, file, builtBy string
		line                             int
	}
	var apps []app
	aspireProjects := map[string]*aspireResource{}
	for _, host := range sortedKeys(b.aspire) {
		for _, r := range b.aspire[host] {
			if r.Kind == "Project" && r.Project != "" {
				aspireProjects[strings.ToLower(r.Project)] = r
			}
		}
	}
	for _, p := range b.poms {
		if p.Executable && !b.contained(p.Dir) && b.repoHasDelivery(p.File) {
			apps = append(apps, app{name: p.ArtifactID, module: p.ArtifactID, dir: p.Dir, file: p.File, line: p.Line, builtBy: "maven"})
		}
	}
	for _, g := range b.gradles {
		if g.Executable && !b.contained(g.Dir) && b.repoHasDelivery(g.File) {
			name := g.Name
			// A single-project build is named by settings.gradle, not by the
			// folder it was cloned into.
			for _, settings := range []string{"settings.gradle", "settings.gradle.kts"} {
				if m := rootProjectName.FindSubmatch(b.in.Files[joinPath(g.Dir, settings)]); m != nil {
					name = string(m[1])
				}
			}
			apps = append(apps, app{name: name, module: g.Name, dir: g.Dir, file: g.File, line: 1, builtBy: "gradle"})
		}
	}
	for _, c := range b.csprojs {
		r := aspireProjects[strings.ToLower(c.Name)]
		if c.AspireHost || b.contained(c.Dir) {
			continue
		}
		switch {
		case r != nil:
			apps = append(apps, app{name: r.Name, module: c.Name, dir: c.Dir, file: c.File, line: 1, builtBy: "aspire"})
		case c.Web && b.repoHasDelivery(c.File):
			apps = append(apps, app{name: c.Name, module: c.Name, dir: c.Dir, file: c.File, line: 1, builtBy: "dotnet"})
		}
	}
	for _, a := range apps {
		d := &Deployable{Kind: "app", BuiltBy: a.builtBy, File: a.file, Line: a.line, Context: a.dir, priority: 5}
		d.aliases = []string{a.name}
		if a.name != a.module {
			d.aliases = append(d.aliases, a.module)
		}
		d.modules = []string{a.module}
		b.m.Deployables = append(b.m.Deployables, d)
		b.addContent(d, &Content{Path: a.dir, Module: a.module, File: a.file, Line: a.line, Resolution: "declared"})
	}
}

// ---------------------------------------------------------------------------
// Names and identities
// ---------------------------------------------------------------------------

func (b *builder) finishNames() {
	// How many deployables answer to each image repository name: the OTel
	// demo builds 25 images all called `demo`.
	repoCount := map[string]int{}
	for _, d := range b.m.Deployables {
		seen := map[string]bool{}
		for _, n := range d.names {
			if !seen[n.Repo] {
				seen[n.Repo] = true
				repoCount[n.Repo]++
			}
		}
	}
	for _, d := range b.m.Deployables {
		// The name most of its builds give it: LibreChat's api image is
		// librechat-api to the release workflows and lc-dev-api to one dev
		// workflow.
		freq := map[string]int{}
		for _, bd := range d.builds {
			seen := map[string]bool{}
			for _, n := range bd.Names {
				if n.Repo != "" && !seen[n.Repo] {
					seen[n.Repo] = true
					freq[n.Repo]++
				}
			}
		}
		name := ""
		best := 0
		for _, n := range d.names {
			if repoCount[n.Repo] == 1 && freq[n.Repo] > best {
				name, best = n.Repo, freq[n.Repo]
			}
		}
		if name == "" && len(d.aliases) > 0 {
			name = normalizeName(d.aliases[0])
		}
		if name == "" && len(d.names) > 0 {
			n := d.names[0]
			name = n.Repo
			if n.Tag != "" {
				name += ":" + n.Tag
			}
		}
		if name == "" && d.dockerfile != "" {
			name = dockerfileName(d.dockerfile)
			if d.target != "" {
				name += "-" + strings.ToLower(d.target)
			}
		}
		if name == "" {
			name = stdpath.Base(d.Context)
		}
		d.Name = name
		d.Repository = b.in.RepoOf(d.File)
	}
	// Ids are names, made unique by repository and then by where they are
	// declared.
	count := map[string]int{}
	for _, d := range b.m.Deployables {
		count[d.Name]++
	}
	sort.SliceStable(b.m.Deployables, func(i, j int) bool {
		a, c := b.m.Deployables[i], b.m.Deployables[j]
		if a.Name != c.Name {
			return a.Name < c.Name
		}
		if a.File != c.File {
			return a.File < c.File
		}
		return a.target < c.target
	})
	used := map[string]bool{}
	for _, d := range b.m.Deployables {
		id := d.Name
		if count[d.Name] > 1 && d.Repository != "" {
			id = d.Repository + "/" + d.Name
		}
		for i := 2; used[id]; i++ {
			id = d.Name + " (" + d.File + ")"
			if used[id] {
				id = d.Name + " (" + d.File + ") " + itoa(i)
			}
		}
		used[id] = true
		d.ID = id
		b.byID[id] = d
	}
	for d, cs := range b.pendingContents {
		for _, c := range cs {
			c.Deployable = d.ID
			if c.Module == "" && c.Pattern == "" {
				if mod := b.in.Modules.Of(c.Path + "/x"); mod != nil && cleanDir(mod.Dir) == c.Path {
					c.Module = mod.Name
				}
			}
		}
	}
}

// dockerfileName names an image by its Dockerfile: the folder it sits in, or
// the suffix of Dockerfile.worker, or the prefix of api.Dockerfile.
func dockerfileName(f string) string {
	base := stdpath.Base(f)
	lower := strings.ToLower(base)
	variant := ""
	switch {
	case strings.HasPrefix(lower, "dockerfile.") || strings.HasPrefix(lower, "dockerfile-") || strings.HasPrefix(lower, "dockerfile_"):
		variant = lower[len("dockerfile."):]
	case strings.HasSuffix(lower, ".dockerfile"):
		variant = strings.TrimSuffix(lower, ".dockerfile")
	}
	folder := ""
	dir := dirOf(f)
	for dir != "." {
		name := stdpath.Base(dir)
		switch strings.ToLower(name) {
		case "src", "docker", "build", "deploy", "container", "containers", "images", "image":
			dir = dirOf(dir)
			continue
		}
		folder = strings.ToLower(name)
		break
	}
	switch {
	case folder != "" && variant != "":
		return folder + "-" + variant
	case variant != "":
		return variant
	case folder != "":
		return folder
	}
	return "root"
}

// ---------------------------------------------------------------------------
// Internal modules a deployable carries through its dependencies
// ---------------------------------------------------------------------------

func (b *builder) moduleExpansion() {
	for _, d := range b.m.Deployables {
		cs := b.pendingContents[d]
		have := map[string]bool{}
		var queue []*module.Module
		for _, c := range cs {
			if c.Pattern != "" {
				continue
			}
			for _, mod := range b.in.Modules.Modules() {
				md := cleanDir(mod.Dir)
				if md == c.Path || c.Path == "." && md != "." || strings.HasPrefix(md, c.Path+"/") || strings.HasPrefix(c.Path, md+"/") && md != "." {
					// A mobile app at a repository's root holds its native
					// projects and its own packages, not every server beside
					// it: bluesky's app carries no Go web server.
					if d.Kind == "mobile_app" && !mobilePart(d, mod) {
						continue
					}
					if !have[mod.Name] {
						have[mod.Name] = true
						queue = append(queue, mod)
					}
				}
			}
		}
		for len(queue) > 0 {
			mod := queue[0]
			queue = queue[1:]
			d.modules = appendUnique(d.modules, mod.Name)
			for _, dep := range mod.DependsOn {
				target := b.in.Modules.ByName(dep)
				if target == nil || have[target.Name] {
					continue
				}
				have[target.Name] = true
				queue = append(queue, target)
				b.pendingContents[d] = append(b.pendingContents[d], &Content{
					Deployable: d.ID, Path: cleanDir(target.Dir), Module: target.Name,
					File: mod.Manifest, Line: lineOf(string(b.in.Files[mod.Manifest]), dep), Resolution: "module_dependency",
				})
			}
		}
		sort.Strings(d.modules)
	}
	for _, d := range b.m.Deployables {
		b.m.Contents = append(b.m.Contents, b.pendingContents[d]...)
	}
}

// ---------------------------------------------------------------------------
// What runs which image
// ---------------------------------------------------------------------------

func (b *builder) imageIndex() *nameIndex {
	idx := newNameIndex()
	for _, d := range b.m.Deployables {
		for _, n := range d.names {
			idx.addImage(n, d.ID)
		}
	}
	return idx
}

func (b *builder) joinWorkloads() {
	idx := b.imageIndex()
	join := func(ref imageRef) []string {
		if ref.Name.Repo == "" {
			b.unresolved(ref.Workload, ref.Raw, ref.File, ref.Line, "interpolated")
			return nil
		}
		l := idx.findImage(ref.Name)
		switch {
		case l.found():
			return []string{l.ID}
		case len(l.Ambiguous) > 0:
			b.unresolved(ref.Workload, ref.Raw+" (could be "+strings.Join(l.Ambiguous, ", ")+")", ref.File, ref.Line, "ambiguous")
		default:
			reason := "not_built_here"
			if wellKnownImages[ref.Name.Repo] {
				reason = "external"
			}
			b.unresolved(ref.Workload, ref.Raw, ref.File, ref.Line, reason)
		}
		return nil
	}
	for _, w := range b.workloads {
		var ids []string
		if w.Build != nil {
			if d := b.buildOf[w.Build]; d != nil {
				ids = appendUnique(ids, d.ID)
			}
		}
		for _, ref := range w.Images {
			if w.Build != nil && len(ids) > 0 {
				// The same block builds it: the image name is this build's.
				continue
			}
			for _, id := range join(ref) {
				ids = appendUnique(ids, id)
			}
		}
		b.runs[w.Key] = ids
	}
	// Kustomize image overrides say which image a base's workloads really
	// run; they join like any other reference.
	for _, k := range b.kustomizes {
		for _, im := range k.Images {
			ref := im.NewName
			if ref == "" {
				continue
			}
			join(imageRef{Raw: ref, Name: NormalizeImage(ref), Workload: k.File, File: k.File, Line: im.Line, Source: "kustomize"})
		}
	}
	// Helm values: the chart runs whatever its values name.
	for _, v := range b.values {
		var ids []string
		for _, ref := range v.Images {
			ref.Workload = v.File
			for _, id := range join(ref) {
				ids = appendUnique(ids, id)
			}
		}
		b.valuesOwner[v.File] = ids
	}
	for _, c := range b.charts {
		var ids []string
		for _, v := range b.values {
			if v.Dir == c.Dir {
				for _, id := range b.valuesOwner[v.File] {
					ids = appendUnique(ids, id)
				}
			}
		}
		b.chartDeps[c.Dir] = ids
	}
}

func (b *builder) unresolved(from, ref, file string, line int, reason string) {
	b.m.Unresolved = append(b.m.Unresolved, &Unresolved{From: from, Ref: ref, File: file, Line: line, Reason: reason})
}

// ---------------------------------------------------------------------------
// Roll-ups
// ---------------------------------------------------------------------------

// covers says whether a content row holds a file.
func covers(c *Content, f string) bool {
	if c.Pattern != "" {
		base := c.Path
		rel := f
		if base != "." {
			if !strings.HasPrefix(f, base+"/") {
				return false
			}
			rel = strings.TrimPrefix(f, base+"/")
		}
		ok, _ := stdpath.Match(c.Pattern, rel)
		return ok
	}
	return c.Path == "." || f == c.Path || strings.HasPrefix(f, c.Path+"/")
}

func (b *builder) contentsOf(id string) []*Content {
	var out []*Content
	for _, c := range b.m.Contents {
		if c.Deployable == id {
			out = append(out, c)
		}
	}
	return out
}

func (b *builder) rollups() {
	byDeployable := map[string][]*Content{}
	for _, c := range b.m.Contents {
		byDeployable[c.Deployable] = append(byDeployable[c.Deployable], c)
	}
	for _, d := range b.m.Deployables {
		perComp := map[string]int{}
		files := 0
		for _, f := range b.in.AllFiles {
			if b.in.Roles[f] != "production" {
				continue
			}
			// The Dockerfile and pom.xml in a service's folder build it;
			// they are not what it runs. Configuration is.
			if k := b.in.Kinds[f]; k != "" && k != "config" {
				continue
			}
			held := false
			for _, c := range byDeployable[d.ID] {
				if covers(c, f) {
					held = true
					break
				}
			}
			if !held {
				continue
			}
			files++
			if comp := b.in.Components[f]; comp != "" {
				perComp[comp]++
			}
		}
		d.Files = files
		d.Components = len(perComp)
		for _, comp := range sortedKeys(perComp) {
			b.m.Components = append(b.m.Components, &ComponentRow{Deployable: d.ID, Component: comp, Files: perComp[comp]})
		}
	}
}

func (b *builder) sortAll() {
	m := b.m
	sort.SliceStable(m.Deployables, func(i, j int) bool { return m.Deployables[i].ID < m.Deployables[j].ID })
	sort.SliceStable(m.Contents, func(i, j int) bool {
		a, c := m.Contents[i], m.Contents[j]
		if a.Deployable != c.Deployable {
			return a.Deployable < c.Deployable
		}
		if a.Path != c.Path {
			return a.Path < c.Path
		}
		return a.Pattern < c.Pattern
	})
	sort.SliceStable(m.Links, func(i, j int) bool {
		a, c := m.Links[i], m.Links[j]
		if a.From != c.From {
			return a.From < c.From
		}
		if a.To != c.To {
			return a.To < c.To
		}
		if a.Kind != c.Kind {
			return a.Kind < c.Kind
		}
		return a.Via < c.Via
	})
	sort.SliceStable(m.Unresolved, func(i, j int) bool {
		a, c := m.Unresolved[i], m.Unresolved[j]
		if a.File != c.File {
			return a.File < c.File
		}
		if a.Line != c.Line {
			return a.Line < c.Line
		}
		return a.Ref < c.Ref
	})
	// Unresolved references repeat when a matrix or several readers meet the
	// same line; one row each is enough.
	var uniq []*Unresolved
	seen := map[string]bool{}
	for _, u := range m.Unresolved {
		k := u.From + "|" + u.Ref + "|" + u.File + "|" + itoa(u.Line) + "|" + u.Reason
		if !seen[k] {
			seen[k] = true
			uniq = append(uniq, u)
		}
	}
	m.Unresolved = uniq
	sort.SliceStable(m.Pipelines, func(i, j int) bool { return m.Pipelines[i].ID < m.Pipelines[j].ID })
	sort.SliceStable(m.PipelineDeployables, func(i, j int) bool {
		a, c := m.PipelineDeployables[i], m.PipelineDeployables[j]
		if a.Pipeline != c.Pipeline {
			return a.Pipeline < c.Pipeline
		}
		if a.Deployable != c.Deployable {
			return a.Deployable < c.Deployable
		}
		return a.Action < c.Action
	})
	sort.SliceStable(m.Environments, func(i, j int) bool {
		a, c := m.Environments[i], m.Environments[j]
		if a.Deployable != c.Deployable {
			return a.Deployable < c.Deployable
		}
		if a.Environment != c.Environment {
			return a.Environment < c.Environment
		}
		return a.Source < c.Source
	})
	sort.SliceStable(m.EnvValues, func(i, j int) bool {
		a, c := m.EnvValues[i], m.EnvValues[j]
		if a.Deployable != c.Deployable {
			return a.Deployable < c.Deployable
		}
		if a.Key != c.Key {
			return a.Key < c.Key
		}
		return a.Environment < c.Environment
	})
	sort.SliceStable(m.Dependencies, func(i, j int) bool {
		a, c := m.Dependencies[i], m.Dependencies[j]
		if a.Deployable != c.Deployable {
			return a.Deployable < c.Deployable
		}
		if a.Role != c.Role {
			return a.Role < c.Role
		}
		if a.Ecosystem != c.Ecosystem {
			return a.Ecosystem < c.Ecosystem
		}
		return a.Name < c.Name
	})
}
