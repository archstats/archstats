package deployables

import (
	"reflect"
	"sort"
	"testing"
)

func buildNames(bs []*imageBuild) []string {
	var out []string
	for _, b := range bs {
		for _, n := range b.Names {
			out = append(out, n.Repo)
		}
	}
	sort.Strings(out)
	return out
}

// The client pattern: every job is a call to a central workflow repository,
// the environment is a dispatch choice, and prod needs a go/no-go id.
const clientCICD = `name: CICD
on:
  workflow_dispatch:
    inputs:
      environment:
        description: 'Select the environment to deploy to'
        type: choice
        options:
          - dev
          - test
          - staging
          - prod
      go-nogo-id:
        type: string
  pull_request:
    branches: [develop, main]
  push:
    branches: [develop, main]
jobs:
  build:
    uses: Acme/cicd-workflows/.github/workflows/build-maven.yml@main
    secrets: inherit
    with:
      java-version: 8
  deploy:
    needs: [build]
    uses: Acme/cicd-workflows/.github/workflows/deployment-nonproduction.yml@main
    with:
      environment: ${{ inputs.environment }}
`

func TestGitHubDelegatedPipeline(t *testing.T) {
	p := readGitHubWorkflow(".github/workflows/cicd.yml", []byte(clientCICD))
	if p.Name != "CICD" || p.System != "github_actions" || p.Parsed != "full" {
		t.Errorf("pipeline = %+v", p)
	}
	if !reflect.DeepEqual(p.Triggers, []string{"workflow_dispatch", "pull_request", "push"}) {
		t.Errorf("triggers = %v", p.Triggers)
	}
	if !reflect.DeepEqual(p.Stages(), []string{"build", "deploy", "approve"}) {
		t.Errorf("stages = %v", p.Stages())
	}
	if len(p.Delegates) != 2 || p.Delegates[0].Target != "Acme/cicd-workflows/.github/workflows/build-maven.yml" || p.Delegates[0].Ref != "main" {
		t.Errorf("delegates = %+v", p.Delegates)
	}
	var envs []string
	for _, e := range p.Environments {
		envs = append(envs, e.Kind+":"+e.Name)
	}
	if !reflect.DeepEqual(envs, []string{"enumerated:dev", "enumerated:test", "enumerated:staging", "enumerated:prod"}) {
		t.Errorf("environments = %v", envs)
	}
	if len(p.Runtimes) != 1 || p.Runtimes[0].Tool != "java" || p.Runtimes[0].Version != "8" {
		t.Errorf("runtimes = %+v", p.Runtimes)
	}
	if !p.BuildTool {
		t.Error("a delegated build-maven counts as running the build tool")
	}
	if len(p.Deploys) != 1 || p.Deploys[0].Kind != "delegated" {
		t.Errorf("deploys = %+v", p.Deploys)
	}
	if !p.Tools["maven"] {
		t.Errorf("tools = %v", p.ToolList())
	}
}

// LibreChat's main-image-workflow: a matrix of targets and image names fed
// to docker/build-push-action.
const libreChatImages = `name: Docker Images Build on Main
on:
  push:
    branches: [main]
    paths:
      - 'api/**'
      - 'client/**'
      - 'packages/**'
jobs:
  build:
    runs-on: ubuntu-latest
    strategy:
      matrix:
        include:
          - target: api-build
            file: Dockerfile.multi
            image_name: librechat-api
          - target: node
            file: Dockerfile
            image_name: librechat
    steps:
      - uses: actions/checkout@v4
      - uses: docker/setup-buildx-action@v3
      - uses: docker/login-action@v3
        with:
          registry: ghcr.io
      - name: Build and push
        uses: docker/build-push-action@v5
        with:
          context: .
          file: ${{ matrix.file }}
          push: true
          tags: |
            ghcr.io/${{ github.repository_owner }}/${{ matrix.image_name }}:${{ env.LATEST_TAG }}
            ghcr.io/${{ github.repository_owner }}/${{ matrix.image_name }}:latest
          target: ${{ matrix.target }}
`

func TestGitHubMatrixBuildPushAction(t *testing.T) {
	p := readGitHubWorkflow(".github/workflows/main-image-workflow.yml", []byte(libreChatImages))
	if len(p.Builds) != 2 {
		t.Fatalf("builds = %+v", p.Builds)
	}
	byTarget := map[string]*imageBuild{}
	for _, b := range p.Builds {
		byTarget[b.Target] = b
	}
	api := byTarget["api-build"]
	if api == nil || api.Dockerfile != "Dockerfile.multi" || api.Context != "." || api.Names[0].Repo != "librechat-api" {
		t.Errorf("api build = %+v", api)
	}
	if n := byTarget["node"]; n == nil || n.Dockerfile != "Dockerfile" || n.Names[0].Repo != "librechat" {
		t.Errorf("node build = %+v", n)
	}
	if !reflect.DeepEqual(p.Paths, []string{"api/**", "client/**", "packages/**"}) {
		t.Errorf("paths = %v", p.Paths)
	}
	if !reflect.DeepEqual(p.Stages(), []string{"package", "publish"}) {
		t.Errorf("stages = %v", p.Stages())
	}
}

// The GCP integration repository: a matrix of maps, and a docker build in a
// run step that names the context through the matrix.
const nestedMatrix = `on:
  push:
    paths:
      - svc/trigger-service/**
      - svc/upload-job/**
jobs:
  build:
    strategy:
      matrix:
        project: [{ id: p-test }, { id: p-prod }]
        environment:
          - { image: "trigger-service", dir: "trigger-service" }
          - { image: "upload-job", dir: "upload-job" }
    defaults:
      run:
        working-directory: svc
    steps:
      - run: |
          docker build -t europe-docker.pkg.dev/${{matrix.project.id}}/repo/${{ matrix.environment.image }} ${{ matrix.environment.dir }}
          docker push europe-docker.pkg.dev/${{matrix.project.id}}/repo/${{ matrix.environment.image }}
`

func TestGitHubNestedMatrixDockerRun(t *testing.T) {
	p := readGitHubWorkflow(".github/workflows/docker.yml", []byte(nestedMatrix))
	// 2 projects x 2 images, deduplicated later by the linker.
	if len(p.Builds) != 4 {
		t.Fatalf("builds = %d", len(p.Builds))
	}
	names := map[string]string{}
	for _, b := range p.Builds {
		names[b.Names[0].Repo] = b.Context + "|" + b.Dockerfile
	}
	want := map[string]string{"trigger-service": "svc/trigger-service|svc/trigger-service/Dockerfile", "upload-job": "svc/upload-job|svc/upload-job/Dockerfile"}
	if !reflect.DeepEqual(names, want) {
		t.Errorf("builds = %v", names)
	}
}

func TestGitHubShellDeploys(t *testing.T) {
	src := `on: [push]
jobs:
  deploy:
    environment: production
    steps:
      - run: helm upgrade --install api ./charts/api -f charts/api/values-prod.yaml --namespace shop --wait
      - run: kubectl apply -f k8s/overlays/prod/
      - run: gcloud run deploy orders --image gcr.io/p/orders:1 --region europe-west4
      - run: aws lambda update-function-code --function-name cart-add --zip-file fileb://x.zip
      - uses: azure/k8s-deploy@v5
        with:
          manifests: |
            deploy/a.yaml
            deploy/b.yaml
`
	p := readGitHubWorkflow(".github/workflows/deploy.yml", []byte(src))
	var got []string
	for _, d := range p.Deploys {
		got = append(got, d.Kind+":"+d.Target)
	}
	want := []string{"helm_values:charts/api/values-prod.yaml", "helm:charts/api", "kubectl:k8s/overlays/prod", "cloud:orders", "cloud:cart-add", "kubectl:deploy/a.yaml", "kubectl:deploy/b.yaml"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("deploys = %v\nwant %v", got, want)
	}
	if len(p.Environments) != 1 || p.Environments[0].Name != "production" {
		t.Errorf("environments = %+v", p.Environments)
	}
	for _, tool := range []string{"helm", "kubectl", "gcloud", "aws"} {
		if !p.Tools[tool] {
			t.Errorf("tool %s missing: %v", tool, p.ToolList())
		}
	}
}

func TestGitHubPreviewPattern(t *testing.T) {
	src := `name: Deploy Preview Environment
on:
  pull_request:
jobs:
  preview:
    environment:
      name: pr-${{ github.event.number }}
    steps:
      - run: helm upgrade --install pr-${{ github.event.number }} ./chart
`
	p := readGitHubWorkflow(".github/workflows/preview.yml", []byte(src))
	if len(p.Environments) != 1 || p.Environments[0].Kind != "pattern" || p.Environments[0].Name != "per pull request" {
		t.Errorf("environments = %+v", p.Environments)
	}
}

func TestGitHubDockerRunFlags(t *testing.T) {
	src := `on: push
jobs:
  b:
    steps:
      - run: >
          docker buildx build --platform linux/amd64 --build-arg VERSION=1
          --target runtime -f docker/api.Dockerfile --push -t registry.io/acme/api:${{ github.sha }} .
      - run: docker build -t local-only $CONTEXT
      - run: echo "docker build is mentioned in prose" && docker build --tag=acme/worker ./worker
`
	p := readGitHubWorkflow(".github/workflows/b.yml", []byte(src))
	if got := buildNames(p.Builds); !reflect.DeepEqual(got, []string{"api", "worker"}) {
		t.Fatalf("builds = %v", got)
	}
	for _, b := range p.Builds {
		switch b.Names[0].Repo {
		case "api":
			if b.Dockerfile != "docker/api.Dockerfile" || b.Context != "." || b.Target != "runtime" {
				t.Errorf("api = %+v", b)
			}
		case "worker":
			if b.Dockerfile != "worker/Dockerfile" || b.Context != "worker" {
				t.Errorf("worker = %+v", b)
			}
		}
	}
}

func TestGitHubLocalReusableAndComposite(t *testing.T) {
	src := `on: push
jobs:
  call:
    uses: ./.github/workflows/build.yml
  steps-job:
    steps:
      - uses: ./.github/actions/setup
      - uses: other/repo/.github/workflows/scan.yml@a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0
`
	p := readGitHubWorkflow(".github/workflows/ci.yml", []byte(src))
	if !reflect.DeepEqual(p.Calls, []string{".github/workflows/build.yml"}) {
		t.Errorf("calls = %v", p.Calls)
	}
	if len(p.Delegates) != 1 || p.Delegates[0].Ref != "a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0" {
		t.Errorf("delegates = %+v", p.Delegates)
	}
}

func TestGitHubMalformed(t *testing.T) {
	p := readGitHubWorkflow(".github/workflows/bad.yml", []byte("on: [push\njobs: {"))
	if p.Parsed != "none" {
		t.Errorf("parsed = %q", p.Parsed)
	}
}

func TestMatrixCombos(t *testing.T) {
	var docs = yamlDocs([]byte("os: [linux, mac]\njava: [8, 17]\ninclude:\n  - os: windows\n    java: 21\n"))
	combos := matrixCombos(docs[0])
	if len(combos) != 5 {
		t.Fatalf("combos = %v", combos)
	}
	if expandGH("${{ matrix.os }}-${{ matrix.java }}", combos[4], nil) != "windows-21" {
		t.Errorf("include expansion = %q", expandGH("${{ matrix.os }}-${{ matrix.java }}", combos[4], nil))
	}
	if expandGH("${{ env.REG }}/x ${{ secrets.T }}", nil, map[string]string{"REG": "ghcr.io"}) != "ghcr.io/x ${{ secrets.T }}" {
		t.Error("env expands, secrets stay")
	}
}

// A declarative Jenkinsfile with a shared library, as eight of the client
// repositories have.
const jenkinsDeclarative = `@Library('jsl@v2') _
// comment: stage('Not a stage')
def app = jsl.com.pricing.Pricing.new()
pipeline {
  agent any
  stages {
    stage('Checkout') { steps { checkout scm } }
    stage('Build') {
      steps {
        sh 'mvn -B clean package -DskipTests'
      }
    }
    stage('Unit Tests') { steps { sh "mvn test" } }
    stage('Sonar') { steps { sh '''
      mvn sonar:sonar \
        -Dsonar.host.url=$SONAR
    ''' } }
    stage('Docker') {
      steps {
        script { docker.build("acme/booking:${env.BUILD_NUMBER}", "-f docker/Dockerfile .") }
      }
    }
    stage('Deploy to DEV') { steps { sh 'helm upgrade --install booking charts/booking -f charts/booking/values-dev.yaml' } }
  }
}
`

func TestJenkinsDeclarative(t *testing.T) {
	p := readJenkinsfile("Jenkinsfile", []byte(jenkinsDeclarative))
	if p.Parsed != "full" || p.System != "jenkins" {
		t.Errorf("pipeline = %+v", p)
	}
	if len(p.Delegates) != 1 || p.Delegates[0].Target != "jenkins-library:jsl" || p.Delegates[0].Ref != "v2" || p.Delegates[0].Line != 1 {
		t.Errorf("delegates = %+v", p.Delegates)
	}
	if !reflect.DeepEqual(p.Stages(), []string{"build", "test", "scan", "package", "deploy"}) {
		t.Errorf("stages = %v", p.Stages())
	}
	if got := buildNames(p.Builds); !reflect.DeepEqual(got, []string{"booking"}) {
		t.Errorf("builds = %v", got)
	} else if p.Builds[0].Dockerfile != "docker/Dockerfile" || p.Builds[0].Context != "." {
		t.Errorf("docker.build = %+v", p.Builds[0])
	}
	var deploys []string
	for _, d := range p.Deploys {
		deploys = append(deploys, d.Kind+":"+d.Target)
	}
	if !reflect.DeepEqual(deploys, []string{"helm_values:charts/booking/values-dev.yaml", "helm:charts/booking"}) {
		t.Errorf("deploys = %v", deploys)
	}
	if !p.BuildTool {
		t.Error("mvn package is a build")
	}
}

func TestJenkinsScriptedIsPartial(t *testing.T) {
	p := readJenkinsfile("Jenkinsfile", []byte("node {\n  stage('Build') { sh 'make' }\n}\n"))
	if p.Parsed != "partial" || !reflect.DeepEqual(p.Stages(), []string{"build"}) {
		t.Errorf("parsed %q stages %v", p.Parsed, p.Stages())
	}
}

const gitlabCI = `stages: [build, test, deploy]
include:
  - template: Jobs/SAST.gitlab-ci.yml
  - project: 'acme/ci-templates'
    ref: v3
    file: '/templates/deploy.yml'
  - local: '/ci/lint.yml'
.base:
  image: golang:1.22
build:
  stage: build
  extends: .base
  script:
    - go build ./...
    - docker build -t registry.acme.io/tools/glab .
test:
  stage: test
  script: [go test ./...]
  rules:
    - changes: [cmd/**/*, internal/**/*]
review:
  stage: deploy
  script: [kubectl apply -f deploy/]
  environment:
    name: review/$CI_COMMIT_REF_SLUG
production:
  stage: deploy
  script: [kubectl apply -f deploy/]
  environment: production
`

func TestGitLabCI(t *testing.T) {
	p := readGitLabCI(".gitlab-ci.yml", []byte(gitlabCI))
	var delegates []string
	for _, d := range p.Delegates {
		delegates = append(delegates, d.Target+"@"+d.Ref)
	}
	if !reflect.DeepEqual(delegates, []string{"gitlab-template:Jobs/SAST.gitlab-ci.yml@", "acme/ci-templates/templates/deploy.yml@v3"}) {
		t.Errorf("delegates = %v", delegates)
	}
	if !reflect.DeepEqual(p.Stages(), []string{"build", "test", "scan", "package", "deploy"}) {
		t.Errorf("stages = %v", p.Stages())
	}
	var envs []string
	for _, e := range p.Environments {
		envs = append(envs, e.Kind+":"+e.Name)
	}
	if !reflect.DeepEqual(envs, []string{"pattern:per branch or merge request", "enumerated:production"}) {
		t.Errorf("environments = %v", envs)
	}
	if !reflect.DeepEqual(p.Paths, []string{"cmd/**/*", "internal/**/*"}) {
		t.Errorf("paths = %v", p.Paths)
	}
	if got := buildNames(p.Builds); !reflect.DeepEqual(got, []string{"glab"}) {
		t.Errorf("builds = %v", got)
	}
}

func TestCISystemOf(t *testing.T) {
	cases := map[string]string{
		".github/workflows/ci.yml":      "github_actions",
		"Jenkinsfile":                   "jenkins",
		"ci/release.jenkinsfile":        "jenkins",
		".gitlab-ci.yml":                "gitlab",
		".gitlab/ci/test.gitlab-ci.yml": "gitlab",
		"azure-pipelines.yml":           "azure_pipelines",
		".circleci/config.yml":          "circleci",
		".travis.yml":                   "travis",
		"bitbucket-pipelines.yml":       "bitbucket",
		"cloudbuild.yaml":               "cloud_build",
		".github/actions/x/action.yml":  "github_action",
	}
	for in, want := range cases {
		if got := ciSystemOf(in); got != want {
			t.Errorf("%s: %s, want %s", in, got, want)
		}
	}
}

func TestStageOfTemplate(t *testing.T) {
	cases := map[string][]string{
		"Acme/cicd/.github/workflows/build-maven.yml":              {"build"},
		"Acme/cicd/.github/workflows/deployment-nonproduction.yml": {"deploy"},
		"Acme/cicd/.github/workflows/static-analysis-maven.yml":    {"scan"},
		"Acme/ghas/.github/workflows/codeQL.yml":                   {"scan"},
		"Acme/cicd/.github/workflows/rollback-kubernetes.yml":      {"deploy"},
		"Acme/cicd/.github/workflows/foss-requests.yml":            {"scan"},
	}
	for in, want := range cases {
		if got := stageOfTemplate(in); !reflect.DeepEqual(got, want) {
			t.Errorf("%s: %v, want %v", in, got, want)
		}
	}
}
