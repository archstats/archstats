package cicd

import (
	"reflect"
	"testing"
)

// Each case is a fact that reads like a risk and is not one, found in real
// repositories, next to the risk the same rule must still report.

func flagRows(m *Model, rule string) []*FlagRow {
	var out []*FlagRow
	for _, f := range m.Flags {
		if f.Rule == rule {
			out = append(out, f)
		}
	}
	return out
}

func TestFlagsShellLinesThatAreNotSecretsOrInsecure(t *testing.T) {
	m := Build(ws(map[string]string{
		".github/workflows/ci.yml": `on: push
permissions: {contents: read}
jobs:
  t:
    runs-on: ubuntu-latest
    steps:
      - run: curl -f -k --retry 30 https://localhost:8443/fineract-provider/actuator/health
      - run: sed -i 's|URL.*$|URL = "redis://redis:6379"|g' config.toml
      - run: psql postgres://root:mysecret@127.0.0.1:5432/app -c 'select 1'
      - run: useradd -m --password "fakepassword" testuser
      - run: echo "Skipping docker push to save quota"
`,
	}))
	if got := flagsOf(m); len(got) != 0 {
		t.Errorf("flags = %v", got)
	}

	m = Build(ws(map[string]string{
		".github/workflows/ci.yml": `on: push
permissions: {contents: read}
jobs:
  t:
    runs-on: ubuntu-latest
    steps:
      - run: curl -k https://repo.acme.com/tool.tgz -o tool.tgz
      - run: git clone https://deploy:hunter2secret@git.acme.com/infra.git
      - run: docker push acme/web
`,
	}))
	if got := flagsOf(m); !reflect.DeepEqual(got, []string{"push_latest_only", "tls_verify_off", "token_literal"}) {
		t.Errorf("flags = %v", got)
	}
}

func TestFlagsGitHubWorkflowsThatAreNotIssues(t *testing.T) {
	m := Build(ws(map[string]string{
		// Called only: the caller's permissions are the ones it runs with.
		".github/workflows/build.yml": `on: workflow_call
jobs:
  b:
    runs-on: ubuntu-latest
    steps:
      - run: make
`,
		// pull_request_target for Dependabot's own pull requests.
		".github/workflows/deps.yml": `on: pull_request_target
permissions: {contents: read}
jobs:
  regen:
    if: github.event.pull_request.user.login == 'dependabot[bot]'
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          ref: ${{ github.head_ref }}
`,
		// A workflow kept as a reference to copy never runs where it is.
		"docs/skills/deploy/references/azure.yml": `on: push
jobs:
  deploy:
    runs-on: ubuntu-latest
    steps:
      - run: curl -sSL https://example.com/install.sh | bash
`,
		// Versions tagged per action in one repository, and a deploy whose
		// gate may sit in a workflow outside the workspace.
		".github/workflows/release.yml": `on: push
permissions: {contents: read}
jobs:
  dns:
    runs-on: ubuntu-latest
    steps:
      - uses: acme/infra/.github/actions/dns@dns/v1
      - uses: acme/apim/actions/register@selfserv_v1.0.0
  deploy-prod:
    uses: acme/cicd/.github/workflows/deployment-production.yml@v3
    with:
      environment: prod
`,
	}))
	if got := flagsOf(m); len(got) != 0 {
		t.Errorf("flags = %v", got)
	}

	m = Build(ws(map[string]string{
		".github/workflows/ci.yml": `on: [push, workflow_call]
jobs:
  first:
    runs-on: ubuntu-latest
    steps:
      - uses: acme/setup@main
  last:
    runs-on: ubuntu-latest
    steps:
      - run: make
`,
		".github/workflows/pr.yml": `on: pull_request_target
permissions: {contents: read}
jobs:
  t:
    if: github.actor == 'dependabot[bot]'
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          ref: ${{ github.event.pull_request.head.sha }}
`,
	}))
	if got := flagsOf(m); !reflect.DeepEqual(got, []string{"handoff_unpinned", "no_permissions_block", "pr_target_checkout_head"}) {
		t.Errorf("flags = %v", got)
	}
	// About the workflow, not the job read last.
	if f := flagRows(m, "no_permissions_block"); len(f) != 1 || f[0].SubjectKind != "pipeline" {
		t.Errorf("no_permissions_block = %+v", f)
	}
}

func TestFlagsDockerfilesThatDoNotRunAsRoot(t *testing.T) {
	m := Build(ws(map[string]string{
		// USER set in the stage the final one is built from.
		"svc/a/Dockerfile": "FROM python:3.12-slim AS base\nRUN useradd app\nUSER app\nFROM base\nCOPY src /src\n",
		"svc/a/src/x.py":   "",
		// A base image that is not root by its own tag.
		"svc/b/Dockerfile": "FROM golang:1.22 AS build\nRUN go build -o /app\nFROM gcr.io/distroless/static-debian12:nonroot\nCOPY --from=build /app /app\n",
		// Files exported with --output: nothing runs.
		"svc/c/Dockerfile": "FROM golang:1.22 AS build\nRUN go build -o /out/app\nFROM scratch\nCOPY --from=build /out /\n",
		// A Dockerfile for working on the code.
		"svc/d/Dockerfile.dev": "FROM node\nCOPY . .\n",
		// A pinned download, and the whole repository as the context with
		// its .dockerignore at the root.
		"docker/node/Dockerfile": "FROM node:20.11@sha256:" + testSHA256 + "\nADD --checksum=sha256:" + testSHA256 + " https://example.com/agent.jar /agent.jar\nCOPY . .\nUSER node\n",
		".dockerignore":          ".git\n",
	}))
	if got := flagsOf(m); !reflect.DeepEqual(got, []string{"base_no_digest", "base_no_digest", "base_no_digest", "base_no_digest"}) {
		t.Errorf("flags = %v", got)
	}

	m = Build(ws(map[string]string{
		"Dockerfile": "FROM python:3.12-slim AS base\nFROM base\nADD https://example.com/agent.jar /agent.jar\n",
	}))
	if f := flagRows(m, "runs_as_root"); len(f) != 1 || f[0].Detail != "no USER, on python:3.12-slim" {
		t.Errorf("runs_as_root = %+v", f)
	}
	if f := flagRows(m, "add_remote_url"); len(f) != 1 || f[0].Detail != "https://example.com/agent.jar" {
		t.Errorf("add_remote_url = %+v", f)
	}
}

func TestFlagsPointAtTheFileTheLineIsIn(t *testing.T) {
	m := Build(ws(map[string]string{
		".gitlab-ci.yml": `include:
  - local: .gitlab/ci/global.gitlab-ci.yml
stages: [build]
build:
  stage: build
  script:
    - !reference [.qemu, script]
    - make
`,
		".gitlab/ci/global.gitlab-ci.yml": `.qemu:
  script:
    - docker run --rm --privileged multiarch/qemu-user-static --reset
`,
	}))
	f := flagRows(m, "docker_privileged")
	if len(f) != 1 || f[0].File != ".gitlab/ci/global.gitlab-ci.yml" || f[0].Line != 3 {
		for _, r := range f {
			t.Errorf("docker_privileged = %+v", r)
		}
		if len(f) == 0 {
			t.Error("no docker_privileged flag")
		}
	}
}

func TestFlagsJenkinsLibraryOnlyWhenTheFileNamesABranch(t *testing.T) {
	m := Build(ws(map[string]string{
		"a/Jenkinsfile": "@Library('helpers') _\nnode { stage('x') { sh 'make' } }\n",
		"b/Jenkinsfile": "@Library('helpers@master') _\nnode { stage('x') { sh 'make' } }\n",
	}))
	if f := flagRows(m, "library_unpinned"); len(f) != 1 || f[0].File != "b/Jenkinsfile" {
		t.Errorf("library_unpinned = %+v", f)
	}
}

const testSHA256 = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
