package cicd

import (
	"regexp"
	"strings"
)

// The runtime an artifact runs on, as far as its files say: the image its
// Dockerfile builds on, or the toolchain the pipeline that builds it sets up.

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

// runtimes fills each artifact's runtime from the images its Dockerfile
// stages build and run on, then from the pipeline that builds it.
func (b *builder) runtimes() {
	for _, d := range b.m.Artifacts {
		if d.Runtime != "" {
			continue
		}
		for _, bd := range d.builds {
			df := b.dockerfiles[bd.Dockerfile]
			if df == nil {
				continue
			}
			target := df.Target(bd.Target)
			if target == nil {
				continue
			}
			if base, _ := df.BaseImage(target); base != "" && d.BaseImage == "" && !strings.EqualFold(base, "scratch") {
				d.BaseImage = base
			}
			for _, im := range df.BuildImages(target) {
				if rt := runtimeFromImage(im); rt != "" {
					d.Runtime = rt
					break
				}
			}
			if d.Runtime != "" {
				break
			}
		}
		if d.Runtime != "" {
			continue
		}
		d.Runtime = b.manifestRuntime(d)
		if d.Runtime != "" {
			continue
		}
		for _, o := range b.m.Outputs {
			if o.Artifact != d.ID || o.Kind == "deploy" {
				continue
			}
			for _, p := range b.pipelines {
				if p.ID == o.Pipeline && len(p.Runtimes) > 0 {
					d.Runtime = p.Runtimes[0].Tool + " " + p.Runtimes[0].Version
				}
			}
		}
	}
}

var secretWords = regexp.MustCompile(`(?i)(pass(word|wd|phrase)?|pwd|secret|token|api[-_.]?key|private[-_.]?key|credential|access[-_.]?key|auth[-_.]?key|signing[-_.]?key|client[-_.]?secret|connection[-_.]?string|sas|cert(ificate)?[-_.]?key)`)

// isSecretKey says whether a configuration key names a secret, so its value
// is never kept.
func isSecretKey(key string) bool {
	if !secretWords.MatchString(key) {
		return false
	}
	// `valueFrom.secretKeyRef.name` and `existingSecret` name a secret kept
	// elsewhere; they hold none, and which secret a service uses is worth
	// seeing.
	lower := strings.ToLower(key)
	for _, ref := range []string{"secretkeyref.name", "secretkeyref.key", "secretref.name", "secretname", "existingsecret", "secret_name", "secret-name"} {
		if strings.HasSuffix(lower, ref) {
			return false
		}
	}
	return true
}

// redactURL keeps a URL's scheme, host, port and path and drops any user or
// password written into it.
func redactURL(v string) string {
	if i := strings.Index(v, "://"); i >= 0 {
		rest := v[i+3:]
		if at := strings.Index(rest, "@"); at >= 0 && (strings.Index(rest, "/") < 0 || at < strings.Index(rest, "/")) {
			return v[:i+3] + rest[at+1:]
		}
	}
	return v
}

var (
	goModVersion = regexp.MustCompile(`(?m)^go\s+(\d+\.\d+)`)
	enginesNode  = regexp.MustCompile(`"engines"\s*:\s*\{[^}]*"node"\s*:\s*"[^0-9]*(\d+)`)
)

// manifestRuntime reads the runtime the artifact's own modules declare:
// java.version in a POM, the Gradle toolchain, a .NET target framework, the
// go directive, package.json engines.
func (b *builder) manifestRuntime(d *Artifact) string {
	for _, name := range d.modules {
		mod := b.in.Modules.ByName(name)
		if mod == nil {
			continue
		}
		for _, p := range b.poms {
			if p.File == mod.Manifest && p.JavaVersion != "" {
				return "java " + p.JavaVersion
			}
		}
		for _, g := range b.gradles {
			if g.File == mod.Manifest && g.JavaVersion != "" {
				return "java " + g.JavaVersion
			}
		}
		for _, c := range b.csprojs {
			if c.File == mod.Manifest && c.TargetFramework != "" {
				return "dotnet " + strings.TrimPrefix(c.TargetFramework, "net")
			}
		}
		content := b.in.Files[mod.Manifest]
		if m := goModVersion.FindSubmatch(content); m != nil {
			return "go " + string(m[1])
		}
		if m := enginesNode.FindSubmatch(content); m != nil {
			return "node " + string(m[1])
		}
	}
	return ""
}
