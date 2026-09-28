package deployables

import (
	"encoding/json"
	stdpath "path"
	"regexp"
	"strings"
)

// What a deployable is made of, as far as the files say: its runtime, its
// framework, the image it rests on, the internal modules it carries and the
// libraries its manifests declare. This is not a vulnerability-grade SBOM.
// Maven and Gradle declare direct dependencies only, and resolving the rest
// needs the network; where a CycloneDX file is committed, it is read as is.
// In the client workspace the useful answer is short: 27 of 28 builds run on
// Java 8, 21 services on Spring Boot 2.3.3, and qp-common is inside 16.

var frameworks = map[string]string{
	// npm
	"react": "react", "next": "next", "vue": "vue", "nuxt": "nuxt", "@angular/core": "angular",
	"react-native": "react-native", "expo": "expo",
	"express": "express", "@nestjs/core": "nestjs", "fastify": "fastify", "koa": "koa",
	"svelte": "svelte", "@sveltejs/kit": "sveltekit", "@remix-run/node": "remix", "hono": "hono",
	// go
	"github.com/gin-gonic/gin": "gin", "github.com/labstack/echo/v4": "echo", "github.com/gofiber/fiber/v2": "fiber",
	"github.com/go-chi/chi/v5": "chi", "google.golang.org/grpc": "grpc", "github.com/gorilla/mux": "gorilla",
	// python
	"django": "django", "flask": "flask", "fastapi": "fastapi", "celery": "celery",
	// php
	"symfony/framework-bundle": "symfony", "laravel/framework": "laravel",
	// jvm
	"io.quarkus:quarkus-core": "quarkus", "io.micronaut:micronaut-core": "micronaut",
}

var runtimeImage = []struct {
	re   *regexp.Regexp
	tool string
}{
	{regexp.MustCompile(`(?:^|/)(?:eclipse-temurin|openjdk|amazoncorretto|adoptopenjdk|ibm-semeru-runtimes|sapmachine|azul/zulu-openjdk[^:]*|java|jdk|jre|maven|gradle)[^:]*:(?:[a-z-]*?)(\d+)`), "java"},
	{regexp.MustCompile(`(?:^|/)node:(\d+)`), "node"},
	{regexp.MustCompile(`(?:^|/)python:(\d+\.\d+)`), "python"},
	{regexp.MustCompile(`(?:^|/)golang:(\d+\.\d+)`), "go"},
	{regexp.MustCompile(`mcr\.microsoft\.com/dotnet/(?:aspnet|runtime|sdk|runtime-deps):(\d+\.\d+)`), "dotnet"},
	{regexp.MustCompile(`(?:^|/)php:(\d+\.\d+)`), "php"},
	{regexp.MustCompile(`(?:^|/)ruby:(\d+\.\d+)`), "ruby"},
	{regexp.MustCompile(`(?:^|/)(?:rust):(\d+\.\d+)`), "rust"},
	{regexp.MustCompile(`distroless/java(\d+)`), "java"},
	{regexp.MustCompile(`distroless/nodejs(\d+)`), "node"},
	{regexp.MustCompile(`distroless/python3`), "python"},
}

func runtimeFromImage(image string) string {
	lower := strings.ToLower(image)
	for _, r := range runtimeImage {
		if m := r.re.FindStringSubmatch(lower); m != nil {
			if len(m) > 1 && m[1] != "" {
				return r.tool + " " + m[1]
			}
			return r.tool
		}
	}
	return ""
}

func (b *builder) dependencies() {
	for _, d := range b.m.Deployables {
		b.dependenciesOf(d)
	}
}

func (b *builder) dep(d *Deployable, eco, name, version, role, source, file string, line int) {
	for _, x := range b.m.Dependencies {
		// An internal module is one module however it is reached: as a
		// Maven dependency of one module and as a Gradle project itself.
		sameEco := x.Ecosystem == eco || role == "internal"
		if x.Deployable == d.ID && sameEco && x.Name == name && x.Role == role {
			if x.Version == "" && version != "" {
				x.Version, x.Source, x.File, x.Line = version, source, file, line
			}
			return
		}
	}
	b.m.Dependencies = append(b.m.Dependencies, &Dependency{Deployable: d.ID, Ecosystem: eco, Name: name, Version: version, Role: role, Source: source, File: file, Line: line})
}

func (b *builder) dependenciesOf(d *Deployable) {
	runtime := d.Runtime
	runtimeFile, runtimeLine, runtimeSource := d.File, d.Line, "manifest"
	// The image it rests on, and the runtime its stages build and run on.
	for _, bd := range d.builds {
		df := b.dockerfiles[bd.Dockerfile]
		if df == nil {
			continue
		}
		target := df.Target(bd.Target)
		if target == nil {
			continue
		}
		if base, line := df.BaseImage(target); base != "" && !strings.EqualFold(base, "scratch") {
			name := base
			if i := strings.LastIndex(name, "@"); i >= 0 {
				name = name[:i]
			}
			version := ""
			if slash, colon := strings.LastIndex(name, "/"), strings.LastIndex(name, ":"); colon > slash {
				name, version = name[:colon], name[colon+1:]
			}
			b.dep(d, "container", name, version, "base_image", "dockerfile", bd.Dockerfile, line)
		}
		if runtime == "" {
			for _, im := range df.BuildImages(target) {
				if rt := runtimeFromImage(im); rt != "" {
					runtime, runtimeFile, runtimeSource = rt, bd.Dockerfile, "dockerfile"
					runtimeLine = target.FromLine
					break
				}
			}
		}
	}
	// Manifests of the modules it holds.
	internal := map[string]bool{}
	for _, mod := range b.in.Modules.Modules() {
		internal[mod.Name] = true
	}
	own := map[string]bool{}
	for _, name := range d.modules {
		own[name] = b.isPrimary(d, name)
	}
	for _, name := range d.modules {
		mod := b.in.Modules.ByName(name)
		if mod == nil {
			continue
		}
		if !b.isPrimary(d, mod.Name) {
			b.dep(d, mod.Kind, mod.Name, "", "internal", "manifest", mod.Manifest, 0)
		}
		if d.Kind == "mobile_app" {
			b.mobileStack(d, mod, b.isPrimary(d, mod.Name))
		}
		base := strings.ToLower(stdpath.Base(mod.Manifest))
		content := b.in.Files[mod.Manifest]
		switch {
		case base == "pom.xml":
			for _, p := range b.poms {
				if p.File != mod.Manifest {
					continue
				}
				if p.SpringBoot != "" {
					b.dep(d, "maven", "spring-boot", p.SpringBoot, "framework", "manifest", p.File, lineOf(p.content, "spring-boot"))
				}
				if runtime == "" && p.JavaVersion != "" {
					runtime, runtimeFile, runtimeLine, runtimeSource = "java "+p.JavaVersion, p.File, lineOf(p.content, p.JavaVersion), "manifest"
				}
				for _, dep := range p.Deps {
					if dep.Scope == "test" || dep.Scope == "provided" || dep.ArtifactID == "" {
						continue
					}
					role, name := "library", dep.GroupID+":"+dep.ArtifactID
					switch {
					case internal[dep.ArtifactID]:
						if own[dep.ArtifactID] {
							continue
						}
						// Named as the module is, so the manifest's version
						// and the module's own row are one row.
						role, name = "internal", dep.ArtifactID
					case frameworks[dep.GroupID+":"+dep.ArtifactID] != "":
						role = "framework"
					}
					b.dep(d, "maven", name, dep.Version, role, "manifest", p.File, dep.Line)
				}
			}
		case base == "build.gradle" || base == "build.gradle.kts":
			for _, g := range b.gradles {
				if g.File == mod.Manifest && runtime == "" && g.JavaVersion != "" {
					runtime, runtimeFile, runtimeLine, runtimeSource = "java "+g.JavaVersion, g.File, 1, "manifest"
				}
			}
		case base == "package.json":
			rt := b.npmDeps(d, mod.Manifest, content, internal, own)
			if runtime == "" && rt != "" {
				runtime, runtimeFile, runtimeLine, runtimeSource = rt, mod.Manifest, lineOf(string(content), `"engines"`), "manifest"
			}
		case base == "go.mod":
			rt, line := b.goDeps(d, mod.Manifest, content)
			if runtime == "" && rt != "" {
				runtime, runtimeFile, runtimeLine, runtimeSource = rt, mod.Manifest, line, "manifest"
			}
		case strings.HasSuffix(base, "proj"):
			for _, c := range b.csprojs {
				if c.File != mod.Manifest {
					continue
				}
				if c.Web {
					b.dep(d, "nuget", "aspnetcore", "", "framework", "manifest", c.File, 1)
				}
				if runtime == "" && c.TargetFramework != "" {
					runtime, runtimeFile, runtimeLine, runtimeSource = "dotnet "+strings.TrimPrefix(c.TargetFramework, "net"), c.File, lineOf(string(content), c.TargetFramework), "manifest"
				}
				for _, p := range c.Packages {
					role := "library"
					if internal[p.ArtifactID] {
						role = "internal"
					}
					b.dep(d, "nuget", p.ArtifactID, p.Version, role, "manifest", c.File, p.Line)
				}
			}
		case base == "composer.json":
			b.composerDeps(d, mod.Manifest, content)
		}
	}
	// A pipeline that builds it says which toolchain it builds with.
	if runtime == "" {
		for _, pd := range b.m.PipelineDeployables {
			if pd.Deployable != d.ID || pd.Action != "builds" {
				continue
			}
			for _, p := range b.pipelines {
				if p.ID == pd.Pipeline && len(p.Runtimes) > 0 {
					r := p.Runtimes[0]
					runtime, runtimeFile, runtimeLine, runtimeSource = r.Tool+" "+r.Version, p.File, r.Line, "pipeline"
				}
			}
		}
	}
	if runtime != "" {
		d.Runtime = runtime
		tool, version, _ := strings.Cut(runtime, " ")
		b.dep(d, "runtime", tool, version, "runtime", runtimeSource, runtimeFile, runtimeLine)
	}
	b.cycloneDX(d)
}

// isPrimary says whether a module is the deployable's own rather than one it
// carries: the module whose folder is the deployable's context or holds it.
func (b *builder) isPrimary(d *Deployable, name string) bool {
	mod := b.in.Modules.ByName(name)
	if mod == nil {
		return false
	}
	md := cleanDir(mod.Dir)
	for _, c := range b.m.Contents {
		if c.Deployable == d.ID && c.Module == name && c.Resolution != "module_dependency" {
			return true
		}
	}
	return md == d.Context
}

type packageJSON struct {
	Name         string            `json:"name"`
	Dependencies map[string]string `json:"dependencies"`
	Engines      map[string]string `json:"engines"`
}

var semverish = regexp.MustCompile(`\d+(\.\d+)*`)

func (b *builder) npmDeps(d *Deployable, manifest string, content []byte, internal, own map[string]bool) string {
	var pj packageJSON
	if json.Unmarshal(content, &pj) != nil {
		return ""
	}
	locked := b.npmLocked(dirOf(manifest))
	for _, name := range sortedKeys(pj.Dependencies) {
		version := locked[name]
		if version == "" {
			version = pj.Dependencies[name]
		}
		role := "library"
		switch {
		case internal[name]:
			if own[name] {
				continue
			}
			role = "internal"
		case frameworks[name] != "":
			role = "framework"
		}
		b.dep(d, "npm", name, version, role, sourceOf(locked[name]), manifest, lineOf(string(content), `"`+name+`"`))
	}
	if node := pj.Engines["node"]; node != "" {
		if m := semverish.FindString(node); m != "" {
			return "node " + strings.Split(m, ".")[0]
		}
	}
	return ""
}

func sourceOf(locked string) string {
	if locked != "" {
		return "lockfile"
	}
	return "manifest"
}

// npmLocked reads exact versions from the nearest package-lock.json,
// walking up to the workspace root the way npm workspaces keep one lock.
func (b *builder) npmLocked(dir string) map[string]string {
	out := map[string]string{}
	for d := dir; ; d = dirOf(d) {
		if raw, ok := b.in.Files[joinPath(d, "package-lock.json")]; ok {
			var lock struct {
				Packages     map[string]struct{ Version string } `json:"packages"`
				Dependencies map[string]struct{ Version string } `json:"dependencies"`
			}
			if json.Unmarshal(raw, &lock) == nil {
				for k, v := range lock.Packages {
					if i := strings.LastIndex(k, "node_modules/"); i >= 0 {
						name := k[i+len("node_modules/"):]
						if _, have := out[name]; !have || !strings.Contains(k[:i], "node_modules") {
							out[name] = v.Version
						}
					}
				}
				for k, v := range lock.Dependencies {
					if _, have := out[k]; !have {
						out[k] = v.Version
					}
				}
			}
			return out
		}
		if d == "." {
			return out
		}
	}
}

var (
	goDirective = regexp.MustCompile(`(?m)^go\s+(\d+\.\d+)`)
	goRequire   = regexp.MustCompile(`(?m)^\s*(?:require\s+)?([a-zA-Z0-9.\-_~/]+\.[a-z]{2,}[^\s]*)\s+(v[^\s]+)(\s*//\s*indirect)?`)
)

func (b *builder) goDeps(d *Deployable, manifest string, content []byte) (string, int) {
	s := string(content)
	for _, m := range goRequire.FindAllStringSubmatchIndex(s, -1) {
		if m[6] >= 0 {
			continue // indirect
		}
		name, version := s[m[2]:m[3]], s[m[4]:m[5]]
		role := "library"
		if frameworks[name] != "" {
			role = "framework"
		}
		b.dep(d, "go", name, version, role, "manifest", manifest, strings.Count(s[:m[0]], "\n")+1)
	}
	if m := goDirective.FindStringSubmatchIndex(s); m != nil {
		return "go " + s[m[2]:m[3]], strings.Count(s[:m[0]], "\n") + 1
	}
	return "", 0
}

func (b *builder) composerDeps(d *Deployable, manifest string, content []byte) {
	var cj struct {
		Require map[string]string `json:"require"`
	}
	if json.Unmarshal(content, &cj) != nil {
		return
	}
	locked := map[string]string{}
	if raw, ok := b.in.Files[joinPath(dirOf(manifest), "composer.lock")]; ok {
		var lock struct {
			Packages []struct{ Name, Version string } `json:"packages"`
		}
		if json.Unmarshal(raw, &lock) == nil {
			for _, p := range lock.Packages {
				locked[p.Name] = p.Version
			}
		}
	}
	for _, name := range sortedKeys(cj.Require) {
		if name == "php" {
			if m := semverish.FindString(cj.Require[name]); m != "" && d.Runtime == "" {
				b.dep(d, "runtime", "php", m, "runtime", "manifest", manifest, lineOf(string(content), `"php"`))
			}
			continue
		}
		if strings.HasPrefix(name, "ext-") {
			continue
		}
		version := locked[name]
		if version == "" {
			version = cj.Require[name]
		}
		role := "library"
		if frameworks[name] != "" {
			role = "framework"
		}
		b.dep(d, "composer", name, version, role, sourceOf(locked[name]), manifest, lineOf(string(content), `"`+name+`"`))
	}
}

// cycloneDX reads a committed CycloneDX file inside what the deployable
// holds, or one whose subject is named like it.
func (b *builder) cycloneDX(d *Deployable) {
	for _, f := range sortedKeys(b.in.Kinds) {
		if b.in.Kinds[f] != "sbom" {
			continue
		}
		var bom struct {
			BomFormat string `json:"bomFormat"`
			Metadata  struct {
				Component struct{ Name string } `json:"component"`
			} `json:"metadata"`
			Components []struct {
				Name, Version, Group, Purl, Type string
			} `json:"components"`
		}
		if json.Unmarshal(b.in.Files[f], &bom) != nil || bom.BomFormat != "CycloneDX" {
			continue
		}
		mine := normalizeName(bom.Metadata.Component.Name) == d.Name
		if !mine {
			for _, c := range b.m.Contents {
				if c.Deployable == d.ID && covers(c, f) {
					mine = true
					break
				}
			}
		}
		if !mine {
			continue
		}
		for _, c := range bom.Components {
			eco := "unknown"
			if strings.HasPrefix(c.Purl, "pkg:") {
				eco = strings.SplitN(strings.TrimPrefix(c.Purl, "pkg:"), "/", 2)[0]
			}
			name := c.Name
			if c.Group != "" {
				name = c.Group + ":" + c.Name
			}
			b.dep(d, eco, name, c.Version, "library", "cyclonedx", f, 0)
		}
	}
}
