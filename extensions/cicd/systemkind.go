package cicd

import (
	"bytes"
	"path"
	"regexp"
	"strings"

	"github.com/archstats/archstats/core/file"
)

// What each file is to building, testing, shipping and running the
// software, by the names and formats of the tools that do it. Registered with
// core as Conventions, so every file's system kind is read once, in the walk.
//
// Measured before it was written: GitHub workflows are in 14 of 19 checkouts
// on one machine, and a 26-repository enterprise workspace keeps its whole
// deployment in 74 per-environment values files. These files were walked and
// then ignored.

// Conventions are CI/CD's: pipelines, containers, deployment, infrastructure,
// runtime configuration, and the files build tools build from and resolve to.
func Conventions() file.Conventions {
	return file.Conventions{Name: "cicd", SystemKind: SystemKind}
}

var lockfileNames = map[string]bool{
	"package-lock.json": true, "npm-shrinkwrap.json": true, "yarn.lock": true,
	"pnpm-lock.yaml": true, "bun.lock": true, "go.sum": true, "cargo.lock": true,
	"composer.lock": true, "poetry.lock": true, "pipfile.lock": true, "uv.lock": true,
	"packages.lock.json": true, "gradle.lockfile": true, "gemfile.lock": true,
	"mix.lock": true, "pubspec.lock": true, "podfile.lock": true, "package.resolved": true,
}

var buildNames = map[string]bool{
	"pom.xml": true, "build.gradle": true, "build.gradle.kts": true,
	"settings.gradle": true, "settings.gradle.kts": true, "libs.versions.toml": true,
	"package.json": true, "go.mod": true, "go.work": true, "composer.json": true,
	"pyproject.toml": true, "setup.py": true, "setup.cfg": true, "pipfile": true,
	"cargo.toml": true, "gemfile": true, "build.sbt": true, "cmakelists.txt": true,
	"makefile": true, "gnumakefile": true, "build": true, "build.bazel": true,
	"workspace": true, "workspace.bazel": true, "module.bazel": true,
	"nx.json": true, "turbo.json": true, "lerna.json": true, "pnpm-workspace.yaml": true,
	"directory.build.props": true, "directory.packages.props": true, "global.json": true,
	".goreleaser.yml": true, ".goreleaser.yaml": true, "earthfile": true, "justfile": true,
	"taskfile.yml": true, "taskfile.yaml": true,
	"pubspec.yaml": true, "package.swift": true, "podfile": true, "cartfile": true,
	"wails.json": true, "tauri.conf.json": true,
}

var buildExts = map[string]bool{
	".csproj": true, ".fsproj": true, ".vbproj": true, ".sln": true, ".slnx": true,
	".pbxproj": true,
}

// requirements.txt, requirements-dev.txt, requirements/base.txt
var requirementsFile = regexp.MustCompile(`(^|/)requirements([-_.][^/]*)?\.txt$|(^|/)requirements/[^/]+\.txt$`)

var ciNames = map[string]bool{
	".gitlab-ci.yml": true, ".gitlab-ci.yaml": true, ".travis.yml": true,
	"bitbucket-pipelines.yml": true, ".drone.yml": true, "buildspec.yml": true,
	"buildspec.yaml": true, "appveyor.yml": true, ".appveyor.yml": true,
	"codefresh.yml": true, "wercker.yml": true, "bitrise.yml": true,
	".rancher-pipeline.yml": true, ".rancher-pipeline.yaml": true,
}

var ciPaths = regexp.MustCompile(`(^|/)\.github/workflows/[^/]+\.ya?ml$|(^|/)\.github/actions/.+/action\.ya?ml$|(^|/)action\.ya?ml$|(^|/)\.gitlab/ci/.+\.ya?ml$|(^|/)\.circleci/config\.ya?ml$|(^|/)\.buildkite/[^/]+\.ya?ml$|(^|/)\.tekton/.+\.ya?ml$|(^|/)azure-pipelines[^/]*\.ya?ml$|(^|/)\.azure-pipelines/.+\.ya?ml$|(^|/)cloudbuild[^/]*\.ya?ml$|(^|/)\.woodpecker(\.ya?ml|/[^/]+\.ya?ml)$`)

// Jenkinsfile, Jenkinsfile.release, release.jenkinsfile, Jenkinsfile-nightly
var jenkinsFile = regexp.MustCompile(`^jenkinsfile([._-][^/]*)?$|\.jenkinsfile$|\.groovy\.jenkins$`)

// Dockerfile, Dockerfile.multi, dev.Dockerfile, Containerfile
var containerFile = regexp.MustCompile(`^(dockerfile|containerfile)([._-][^/]*)?$|\.(dockerfile|containerfile)$`)

// docker-compose.yml, compose.yaml, docker-compose.override.yml, compose.full.yaml
var composeFile = regexp.MustCompile(`^(docker-)?compose([._-][^/]*)?\.ya?ml$`)

var deployNames = map[string]bool{
	"chart.yaml": true, "chart.yml": true, "fleet.yaml": true, "fleet.yml": true,
	"kustomization.yaml": true, "kustomization.yml": true, "kustomization": true,
	"helmfile.yaml": true, "helmfile.yml": true, "serverless.yml": true,
	"serverless.yaml": true, "procfile": true, "fly.toml": true, "vercel.json": true,
	"netlify.toml": true, "render.yaml": true, "app.yaml": true, "tiltfile": true,
	"devspace.yaml": true, "garden.yml": true, "okteto.yml": true, "samconfig.toml": true,
	"cdk.json": true, "pulumi.yaml": true, "pulumi.yml": true, "bunnyshell.yaml": true,
}

// values.yaml, values-prod.yaml, values.dev.yml, prod.values.yaml
var valuesFileName = regexp.MustCompile(`^(values([._-][^/]*)?|[^/]+[._-]values)\.ya?ml$`)

// Folders whose YAML is deployment by convention, whatever the file is named:
// the client pattern `helm-release/prod.yaml`, kustomize overlays, `k8s/`.
var deployDir = regexp.MustCompile(`(^|/)[^/]*\b(helm|charts?|k8s|kubernetes|kube|manifests|deploy|deployments?|overlays|kustomize|argocd|argo|gitops|environments|skaffold|openshift)\b[^/]*(/|$)`)

var infraExts = map[string]bool{".tf": true, ".tfvars": true, ".hcl": true, ".bicep": true}

// application.yml, application-prod.properties, bootstrap.yml,
// appsettings.Development.json, .env, .env.example, config/prod.env
var configFile = regexp.MustCompile(`^(application|bootstrap)([._-][^/]*)?\.(ya?ml|properties)$|^appsettings([._-][^/]*)?\.json$|^\.env(\.[^/]*)?$|\.env$`)

// SystemKind classifies a file by its path and, for YAML and JSON whose name
// says nothing, by the first few kilobytes of its content. Paths are as the
// walker names them ("./x" at the root). An empty result means the file
// describes no part of building or running the software.
func SystemKind(filePath string, content []byte) string {
	p := strings.TrimPrefix(filePath, "./")
	base := strings.ToLower(path.Base(p))
	lowerPath := strings.ToLower(p)
	ext := path.Ext(base)

	switch {
	case lockfileNames[base]:
		return file.SystemKindLockfile
	case ciNames[base], ciPaths.MatchString(lowerPath), jenkinsFile.MatchString(base):
		return file.SystemKindCI
	case containerFile.MatchString(base):
		return file.SystemKindContainer
	case composeFile.MatchString(base), deployNames[base]:
		return file.SystemKindDeploy
	case buildNames[base], buildExts[ext], requirementsFile.MatchString(lowerPath):
		return file.SystemKindBuild
	case base == "terragrunt.hcl", infraExts[ext] && ext != ".hcl":
		return file.SystemKindInfra
	case configFile.MatchString(base):
		return file.SystemKindConfig
	}

	if ext == ".yaml" || ext == ".yml" || ext == ".json" || ext == ".template" {
		if kind := sniff(content); kind != "" {
			return kind
		}
	}
	if ext == ".yaml" || ext == ".yml" {
		if valuesFileName.MatchString(base) {
			return file.SystemKindDeploy
		}
		if deployDir.MatchString(path.Dir(lowerPath)) {
			return file.SystemKindDeploy
		}
	}
	return ""
}

var (
	cfnMarker        = regexp.MustCompile(`(?m)^\s*"?AWSTemplateFormatVersion"?\s*:`)
	samMarker        = regexp.MustCompile(`(?m)^\s*"?Transform"?\s*:.*AWS::Serverless`)
	skaffoldMarker   = regexp.MustCompile(`(?m)^apiVersion:\s*skaffold/`)
	k8sAPIVersion    = regexp.MustCompile(`(?m)^apiVersion:\s*\S+`)
	k8sKind          = regexp.MustCompile(`(?m)^kind:\s*[A-Z][A-Za-z]+`)
	composeServices  = regexp.MustCompile(`(?m)^services:\s*$`)
	composeService   = regexp.MustCompile(`(?m)^  [A-Za-z0-9_.-]+:\s*$\n(?:(?:    .*|\s*)\n)*?    (image|build):`)
	workflowJobs     = regexp.MustCompile(`(?m)^jobs:\s*$`)
	workflowTriggers = regexp.MustCompile(`(?m)^(on|"on"|'on'|true):`)
)

// sniff reads the head of a YAML or JSON file for the markers of formats that
// are named anything at all.
func sniff(content []byte) string {
	head := content
	if len(head) > 8192 {
		head = head[:8192]
	}
	switch {
	case samMarker.Match(head):
		return file.SystemKindDeploy
	case cfnMarker.Match(head), bytes.Contains(head, []byte(`"Type": "AWS::`)):
		return file.SystemKindInfra
	case skaffoldMarker.Match(head):
		return file.SystemKindDeploy
	case k8sAPIVersion.Match(head) && k8sKind.Match(head):
		return file.SystemKindDeploy
	case composeServices.Match(head) && composeService.Match(head):
		return file.SystemKindDeploy
	case workflowJobs.Match(head) && workflowTriggers.Match(head):
		return file.SystemKindCI
	}
	return ""
}
