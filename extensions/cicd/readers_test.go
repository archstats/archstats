package cicd

import (
	"reflect"
	"sort"
	"strings"
	"testing"
)

// Helpers over a built model.

func jobsOf(m *Model, pipeline string) map[string]*JobRow {
	out := map[string]*JobRow{}
	for _, j := range m.Jobs {
		if j.Pipeline == pipeline {
			out[j.ID] = j
		}
	}
	return out
}

func stepsOf(m *Model, pipeline, job string) []string {
	var out []string
	for _, s := range m.Steps {
		if s.Pipeline == pipeline && s.Job == job {
			out = append(out, s.Verb+":"+s.Tool)
		}
	}
	return out
}

func pipelineRow(m *Model, id string) *PipelineRow {
	for _, p := range m.Pipelines {
		if p.ID == id {
			return p
		}
	}
	return nil
}

func handoffsOf(m *Model, pipeline string) []string {
	var out []string
	for _, h := range m.Handoffs {
		if h.Pipeline == pipeline {
			out = append(out, h.Kind+" "+h.Target+"@"+h.Ref+" "+h.Pinned)
		}
	}
	sort.Strings(out)
	return out
}

func triggersOf(m *Model, pipeline string) []string {
	var out []string
	for _, t := range m.Triggers {
		if t.Pipeline == pipeline {
			out = append(out, t.Event+":"+t.Filter)
		}
	}
	sort.Strings(out)
	return out
}

func flagsOf(m *Model) []string {
	var out []string
	for _, f := range m.Flags {
		out = append(out, f.Rule)
	}
	sort.Strings(out)
	return out
}

// ---------------------------------------------------------------------------
// GitHub Actions
// ---------------------------------------------------------------------------

func TestGitHubJobsNeedsMatrixAndPins(t *testing.T) {
	m := Build(ws(map[string]string{
		".github/workflows/ci.yml": `name: CI
on:
  push:
    branches: [main]
  pull_request:
  schedule:
    - cron: "0 3 * * *"
permissions:
  contents: read
env:
  IMAGE: acme/api
jobs:
  test:
    runs-on: ubuntu-latest
    strategy:
      matrix:
        go: ["1.22", "1.23"]
        os: [linux, mac]
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@0c52d547c9bc32b1aa3301fd7a9cb496313a4491
      - run: go test ./...
  image:
    needs: test
    runs-on: ubuntu-latest
    steps:
      - uses: docker/build-push-action@master
        with:
          context: .
          push: true
          tags: acme/api:${{ github.sha }}
  deploy:
    needs: [image]
    environment: production
    runs-on: ubuntu-latest
    steps:
      - run: helm upgrade --install api ./chart
`,
		"Dockerfile":        "FROM golang:1.22 AS build\nCOPY . .\nRUN go build\nFROM gcr.io/distroless/static\nUSER nonroot\nCOPY --from=build /app /app\n",
		".dockerignore":     "*\n",
		"main.go":           "package main",
		"chart/Chart.yaml":  "name: api\n",
		"chart/values.yaml": "image:\n  repository: acme/api\n",
	}))
	id := ".github/workflows/ci.yml"
	p := pipelineRow(m, id)
	if p == nil || p.Read != "full" || p.Jobs != 3 {
		t.Fatalf("pipeline = %+v", p)
	}
	jobs := jobsOf(m, id)
	if jobs["test"].Matrix != 4 || jobs["test"].Depth != 0 || jobs["image"].Depth != 1 || jobs["deploy"].Depth != 2 {
		t.Errorf("jobs = %+v %+v %+v", jobs["test"], jobs["image"], jobs["deploy"])
	}
	if jobs["deploy"].Environment != "production" || jobs["deploy"].Gate != "environment" {
		t.Errorf("deploy = %+v", jobs["deploy"])
	}
	if got := stepsOf(m, id, "test"); !reflect.DeepEqual(got, []string{"checkout:git", "setup:go", "test:go"}) {
		t.Errorf("test steps = %v", got)
	}
	if got := stepsOf(m, id, "image"); !reflect.DeepEqual(got, []string{"publish:docker"}) {
		t.Errorf("image steps = %v", got)
	}
	want := []string{
		"action actions/checkout@v4 version",
		"action actions/setup-go@0c52d547c9bc32b1aa3301fd7a9cb496313a4491 sha",
		"action docker/build-push-action@master branch",
	}
	if got := handoffsOf(m, id); !reflect.DeepEqual(got, want) {
		t.Errorf("handoffs = %v", got)
	}
	if got := triggersOf(m, id); !reflect.DeepEqual(got, []string{"pull_request:", "push:branches: main", "schedule:0 3 * * *"}) {
		t.Errorf("triggers = %v", got)
	}
	var outs []string
	for _, o := range m.Outputs {
		outs = append(outs, o.Job+" "+o.Kind+" "+o.Artifact)
	}
	sort.Strings(outs)
	if !reflect.DeepEqual(outs, []string{"deploy deploy api", "image image api"}) {
		t.Errorf("outputs = %v", outs)
	}
	// Pinned to a branch: flagged. Deploy to production behind an
	// environment: not flagged.
	if got := flagsOf(m); !reflect.DeepEqual(got, []string{"base_no_digest", "base_unpinned_tag", "handoff_unpinned"}) {
		t.Errorf("flags = %v", got)
	}
}

func TestGitHubReusableWorkflowAcrossRepositories(t *testing.T) {
	in := ws(map[string]string{
		"repo-app/.github/workflows/ci.yml": `on: push
jobs:
  build:
    uses: acme/workflows/.github/workflows/build.yml@v2
`,
		"repo-workflows/.github/workflows/build.yml": `on:
  workflow_call:
jobs:
  mvn:
    runs-on: ubuntu-latest
    steps:
      - run: mvn -B package
`,
	})
	in.Remotes = map[string]string{"repo-app": "acme/app", "repo-workflows": "acme/workflows"}
	in.AllFiles = append(in.AllFiles, "repo-app/x", "repo-workflows/x")
	m := Build(in)
	var local string
	for _, h := range m.Handoffs {
		if h.Pipeline == "repo-app/.github/workflows/ci.yml" {
			local = h.LocalPipeline
		}
	}
	if local != "repo-workflows/.github/workflows/build.yml" {
		t.Errorf("reusable workflow resolved to %q", local)
	}
	if p := pipelineRow(m, "repo-workflows/.github/workflows/build.yml"); p == nil || p.Kind != "reusable_workflow" {
		t.Errorf("called workflow = %+v", p)
	}
}

func TestGitHubWorkflowRunStartsPipeline(t *testing.T) {
	m := Build(ws(map[string]string{
		".github/workflows/build.yml":  "name: Build\non: push\njobs:\n  b:\n    runs-on: x\n    steps:\n      - run: make\n",
		".github/workflows/deploy.yml": "name: Deploy\non:\n  workflow_run:\n    workflows: [Build]\n    types: [completed]\njobs:\n  d:\n    runs-on: x\n    steps:\n      - run: ./deploy.sh\n",
	}))
	got := handoffsOf(m, ".github/workflows/build.yml")
	if !reflect.DeepEqual(got, []string{"starts_pipeline .github/workflows/deploy.yml@ "}) {
		t.Errorf("handoffs = %v", got)
	}
}

func TestGitHubPullRequestTargetCheckoutAndNoPermissions(t *testing.T) {
	m := Build(ws(map[string]string{
		".github/workflows/pr.yml": `on: pull_request_target
jobs:
  t:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          ref: ${{ github.event.pull_request.head.sha }}
      - run: curl -sSL https://example.com/install.sh | bash
`,
	}))
	if got := flagsOf(m); !reflect.DeepEqual(got, []string{"curl_pipe_shell", "no_permissions_block", "pr_target_checkout_head"}) {
		t.Errorf("flags = %v", got)
	}
}

// ---------------------------------------------------------------------------
// GitLab
// ---------------------------------------------------------------------------

func TestGitLabStagesExtendsIncludesAndReference(t *testing.T) {
	m := Build(ws(map[string]string{
		".gitlab-ci.yml": `include:
  - local: ci/templates.yml
  - project: acme/ci
    file: /deploy.yml
  - template: Security/SAST.gitlab-ci.yml
  - component: gitlab.com/acme/components/lint@1.2.0
stages: [build, test, deploy]
variables:
  IMAGE: $CI_REGISTRY_IMAGE:$CI_COMMIT_SHA
workflow:
  rules:
    - if: $CI_PIPELINE_SOURCE == "merge_request_event"
    - if: $CI_COMMIT_BRANCH == $CI_DEFAULT_BRANCH
    - if: $CI_COMMIT_TAG
build:
  extends: .docker
  stage: build
  script:
    - docker build -t $IMAGE .
    - docker push $IMAGE
unit:
  stage: test
  image: node:20
  script:
    - !reference [.setup, script]
    - npm test
lint:
  stage: test
  needs: []
  script: [npm run lint]
deploy:
  stage: deploy
  environment:
    name: production
  when: manual
  script:
    - helm upgrade --install app ./chart
child:
  stage: deploy
  trigger:
    include: ci/child.yml
`,
		"ci/templates.yml": `.docker:
  image: docker:27
  services: [docker:27-dind]
.setup:
  script:
    - npm ci
`,
		"ci/child.yml":  "child-job:\n  script: [echo hi]\n",
		"Dockerfile":    "FROM node:20\nUSER node\nCOPY . .\n",
		".dockerignore": "node_modules\n",
		"index.js":      "",
	}))
	id := "./.gitlab-ci.yml"
	id = ".gitlab-ci.yml"
	jobs := jobsOf(m, id)
	if len(jobs) != 5 {
		t.Fatalf("jobs = %v", jobs)
	}
	if jobs["unit"].Needs != "build" || jobs["lint"].Needs != "" || jobs["deploy"].Needs != "lint, unit" {
		t.Errorf("needs: unit=%q lint=%q deploy=%q", jobs["unit"].Needs, jobs["lint"].Needs, jobs["deploy"].Needs)
	}
	if jobs["build"].Runner != "docker:27 · docker-in-docker" || jobs["build"].Group != "build" {
		t.Errorf("build = %+v", jobs["build"])
	}
	if jobs["deploy"].Gate != "manual" || jobs["deploy"].Environment != "production" {
		t.Errorf("deploy = %+v", jobs["deploy"])
	}
	if got := stepsOf(m, id, "unit"); !reflect.DeepEqual(got, []string{"build:npm", "test:npm"}) {
		t.Errorf("unit steps (with !reference) = %v", got)
	}
	want := []string{
		"child_pipeline ci/child.yml@ ",
		"component gitlab.com/acme/components/lint@1.2.0 version",
		"include acme/ci/deploy.yml@ none",
		"template Security/SAST.gitlab-ci.yml@implicit implicit",
	}
	if got := handoffsOf(m, id); !reflect.DeepEqual(got, want) {
		t.Errorf("handoffs = %v", got)
	}
	if got := triggersOf(m, id); !reflect.DeepEqual(got, []string{"pull_request:merge requests", "push:default branch", "tag:"}) {
		t.Errorf("triggers = %v", got)
	}
	// The child pipeline is read too.
	if p := pipelineRow(m, "ci/child.yml"); p == nil || p.Kind != "child" {
		t.Errorf("child = %+v", p)
	}
	// The image is called after the project.
	built := false
	for _, o := range m.Outputs {
		if o.Kind == "image" && o.Pipeline == id && strings.HasPrefix(o.Target, "shop") {
			built = true
		}
	}
	if !built {
		t.Errorf("outputs = %+v", m.Outputs)
	}
	if got := flagsOf(m); !reflect.DeepEqual(got, []string{"base_no_digest", "remote_include_unpinned"}) {
		t.Errorf("flags = %v", got)
	}
}

// ---------------------------------------------------------------------------
// Rancher
// ---------------------------------------------------------------------------

// The pattern a GitLab pipeline deploys to Rancher with: log in with a
// context per environment, then apply manifests with rancher kubectl.
func TestRancherCLIDeployPerEnvironment(t *testing.T) {
	m := Build(ws(map[string]string{
		".gitlab-ci.yml": `stages: [build, deploy]
build:
  stage: build
  image: docker:27
  script:
    - docker build -t harbor.example/app/web:latest .
    - docker push harbor.example/app/web:latest
.rancher:
  image: harbor.example/tools/rancher-cli
  before_script:
    - rancher login $RANCHER_SERVER --token $RANCHER_TOKEN --context $RANCHER_PROJECT_$ENV_SUFFIX --skip-verify
deploy-dev:
  extends: .rancher
  stage: deploy
  script:
    - rancher login $RANCHER_SERVER_NP --token $RANCHER_TOKEN_NP --context $RANCHER_PROJECT_DEV --skip-verify
    - rancher kubectl --insecure-skip-tls-verify apply -f k8s/deployment.yaml
deploy-prd:
  extends: .rancher
  stage: deploy
  script:
    - rancher login $RANCHER_SERVER_P --token $RANCHER_TOKEN_P --context $RANCHER_PROJECT_PRD --skip-verify
    - rancher kubectl --insecure-skip-tls-verify apply -f k8s/deployment.yaml
`,
		"Dockerfile":          "FROM nginx:1.27\nCOPY site /usr/share/nginx/html\n",
		"site/index.html":     "",
		"k8s/deployment.yaml": "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: web\nspec:\n  template:\n    spec:\n      containers:\n      - name: web\n        image: harbor.example/app/web:latest\n",
	}))
	var deploys []string
	for _, o := range m.Outputs {
		if o.Kind == "deploy" {
			deploys = append(deploys, o.Job+" "+o.Platform+" "+o.Environment+" "+o.Context+" → "+o.Artifact)
		}
	}
	sort.Strings(deploys)
	want := []string{
		"deploy-dev rancher dev $RANCHER_PROJECT_DEV → web",
		"deploy-prd rancher prd $RANCHER_PROJECT_PRD → web",
	}
	if !reflect.DeepEqual(deploys, want) {
		t.Errorf("deploys = %v", deploys)
	}
	got := flagsOf(m)
	for _, rule := range []string{"push_latest_only", "tls_verify_off", "ungated_prod_deploy"} {
		if !containsStr(got, rule) {
			t.Errorf("missing flag %s in %v", rule, got)
		}
	}
}

func TestFleetBundleTargets(t *testing.T) {
	m := Build(ws(map[string]string{
		"apps/web/fleet.yaml": `defaultNamespace: web
helm:
  chart: ./chart
targetCustomizations:
- name: staging
  clusterSelector:
    matchLabels:
      env: staging
- name: production
  clusterGroup: prod
`,
		"apps/web/chart/Chart.yaml":  "name: web\n",
		"apps/web/chart/values.yaml": "image:\n  repository: acme/web\n",
		"fleet/gitrepo.yaml": `apiVersion: fleet.cattle.io/v1alpha1
kind: GitRepo
metadata:
  name: apps
spec:
  repo: https://github.com/acme/shop
  branch: release
  paths: [apps]
`,
		"Dockerfile": "FROM node:20\nUSER node\nCOPY src /src\n",
		"src/i.js":   "",
	}))
	id := "apps/web/fleet.yaml"
	p := pipelineRow(m, id)
	if p == nil || p.System != "rancher_fleet" || p.Kind != "gitops" || p.Jobs != 2 {
		t.Fatalf("fleet = %+v", p)
	}
	jobs := jobsOf(m, id)
	if jobs["production"].Runner != "fleet · cluster group prod" || jobs["production"].Environment != "production" || jobs["staging"].Environment != "staging" {
		t.Errorf("production = %+v", jobs["production"])
	}
	if got := handoffsOf(m, id); !reflect.DeepEqual(got, []string{"helm_chart apps/web/chart@ "}) {
		t.Errorf("handoffs = %v", got)
	}
}

func TestFleetGitRepoInWorkspace(t *testing.T) {
	in := ws(map[string]string{
		"repo-apps/web/fleet.yaml":  "defaultNamespace: web\n",
		"repo-apps/web/deploy.yaml": "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: x\n",
		"repo-ops/gitrepo.yaml":     "apiVersion: fleet.cattle.io/v1alpha1\nkind: GitRepo\nmetadata:\n  name: apps\nspec:\n  repo: https://github.com/acme/apps\n  branch: release\n  paths: [web]\n",
	})
	in.Remotes = map[string]string{"repo-apps": "acme/apps", "repo-ops": "acme/ops"}
	in.AllFiles = append(in.AllFiles, "repo-apps/x", "repo-ops/x")
	m := Build(in)
	got := triggersOf(m, "repo-apps/web/fleet.yaml")
	if len(got) != 1 || !strings.HasPrefix(got[0], "push:release · GitRepo apps") {
		t.Errorf("triggers = %v", got)
	}
}

func TestRancherPipelinesLegacy(t *testing.T) {
	m := Build(ws(map[string]string{
		".rancher-pipeline.yml": `stages:
- name: Build
  steps:
  - runScriptConfig:
      image: maven:3-jdk-8
      shellScript: mvn package
- name: Publish
  steps:
  - publishImageConfig:
      dockerfilePath: ./Dockerfile
      buildContext: .
      tag: acme/app:${CICD_EXECUTION_SEQUENCE}
- name: Deploy
  steps:
  - applyYamlConfig:
      path: ./deployment.yaml
branch:
  include: [master]
`,
		"Dockerfile":      "FROM openjdk:8\nUSER app\nCOPY target/app.jar /app.jar\n",
		"deployment.yaml": "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: app\nspec:\n  template:\n    spec:\n      containers:\n      - image: acme/app\n",
	}))
	id := ".rancher-pipeline.yml"
	jobs := jobsOf(m, id)
	if len(jobs) != 3 || jobs["Publish"].Needs != "Build" || jobs["Deploy"].Needs != "Publish" || jobs["Build"].Runner != "maven:3-jdk-8" {
		t.Fatalf("jobs = %+v", jobs)
	}
	var outs []string
	for _, o := range m.Outputs {
		outs = append(outs, o.Kind+" "+o.Platform+" "+o.Artifact)
	}
	sort.Strings(outs)
	if !reflect.DeepEqual(outs, []string{"deploy rancher app", "image  app"}) {
		t.Errorf("outputs = %v", outs)
	}
}

// ---------------------------------------------------------------------------
// Jenkins
// ---------------------------------------------------------------------------

func TestJenkinsDeclarativeParallelInputAndLibrary(t *testing.T) {
	m := Build(ws(map[string]string{
		"Jenkinsfile": `@Library('acme-lib@v1.4') _
pipeline {
  agent { kubernetes { yaml '''
spec:
  containers:
  - name: maven
    image: maven:3.9
''' } }
  triggers { cron('H 2 * * *') }
  environment { REG = 'harbor.acme' ; PW = credentials('reg') }
  stages {
    stage('Build') { steps { sh 'mvn -B package' } }
    stage('Checks') {
      parallel {
        stage('Unit') { steps { sh 'mvn test' } }
        stage('Sonar') { steps { withSonarQubeEnv('s') { sh 'mvn sonar:sonar' } } }
      }
    }
    stage('Image') {
      steps {
        script {
          def img = docker.build("${REG}/app:${env.BUILD_NUMBER}")
          img.push()
        }
      }
    }
    stage('Deploy prod') {
      when { branch 'main' }
      input { message 'Ship it?' }
      steps { deployToK8s(env: 'prod') }
    }
  }
  post { always { junit 'target/*.xml' } }
}
`,
	}))
	id := "./Jenkinsfile"
	id = "Jenkinsfile"
	p := pipelineRow(m, id)
	if p == nil || p.Read != "full" {
		t.Fatalf("pipeline = %+v", p)
	}
	jobs := jobsOf(m, id)
	if jobs["Unit"].Needs != "Build" || jobs["Sonar"].Needs != "Build" || jobs["Image"].Needs != "Sonar, Unit" {
		t.Errorf("needs: unit=%q sonar=%q image=%q", jobs["Unit"].Needs, jobs["Sonar"].Needs, jobs["Image"].Needs)
	}
	if jobs["Unit"].Group != "Checks" || jobs["Build"].Runner != "kubernetes · maven:3.9" {
		t.Errorf("unit=%+v build=%+v", jobs["Unit"], jobs["Build"])
	}
	if d := jobs["Deploy prod"]; d.Gate != "approval" || d.Condition != "branch main" || d.Environment != "prod" {
		t.Errorf("deploy = %+v", d)
	}
	if got := stepsOf(m, id, "Image"); !reflect.DeepEqual(got, []string{"package:docker", "publish:docker"}) {
		t.Errorf("image steps = %v", got)
	}
	if got := stepsOf(m, id, "Sonar"); !reflect.DeepEqual(got, []string{"scan:maven"}) {
		t.Errorf("sonar steps = %v", got)
	}
	want := []string{"library_step deployToK8s@v1.4 version", "shared_library acme-lib@v1.4 version"}
	if got := handoffsOf(m, id); !reflect.DeepEqual(got, want) {
		t.Errorf("handoffs = %v", got)
	}
	if got := triggersOf(m, id); !reflect.DeepEqual(got, []string{"schedule:H 2 * * *"}) {
		t.Errorf("triggers = %v", got)
	}
}

// BroadleafCommerce's Jenkinsfile: the whole pipeline is a call to a library
// the Jenkins server loads for every job.
func TestJenkinsLibraryOnly(t *testing.T) {
	m := Build(ws(map[string]string{
		"Jenkinsfile": "properties(commonModuleJobProps())\nbuildBroadleafModule(params, false, 'maven-jdk17')\n",
	}))
	p := pipelineRow(m, "Jenkinsfile")
	if p == nil || p.Read != "full" || p.Jobs != 1 || p.Steps != 1 {
		t.Fatalf("pipeline = %+v", p)
	}
	if got := handoffsOf(m, "Jenkinsfile"); !reflect.DeepEqual(got, []string{"library_step buildBroadleafModule@implicit implicit"}) {
		t.Errorf("handoffs = %v", got)
	}
	for _, j := range m.Jobs {
		if j.Name != "buildBroadleafModule" {
			t.Errorf("job = %+v", j)
		}
	}
}

func TestJenkinsScriptedLoopsAndParallelMap(t *testing.T) {
	m := Build(ws(map[string]string{
		"Jenkinsfile": `def envs = ['h2', 'pg']
node('linux') {
  stage('Checkout') { checkout scm }
  stage('Test') {
    def branches = [:]
    envs.each { db ->
      branches[db] = {
        stage("Test ${db}") { sh "./gradlew test -Pdb=${db}" }
      }
    }
    parallel branches
  }
  stage('Publish') {
    sh './gradlew publish'
  }
}
`,
	}))
	jobs := jobsOf(m, "Jenkinsfile")
	tj := jobs["Test ${db}"]
	if tj == nil || tj.Matrix != -1 || tj.Needs != "Checkout" || tj.Runner != "linux" {
		t.Fatalf("templated stage = %+v (jobs %v)", tj, jobs)
	}
	if jobs["Publish"].Needs != "Test ${db}" {
		t.Errorf("publish needs %q", jobs["Publish"].Needs)
	}
	if p := pipelineRow(m, "Jenkinsfile"); p.Read != "full" {
		t.Errorf("read = %s", p.Read)
	}
}

func TestJenkinsDynamicParallelIsUnread(t *testing.T) {
	m := Build(ws(map[string]string{
		"Jenkinsfile": "node {\n  stage('Build') {\n    parallel(loadBranches())\n  }\n}\n",
	}))
	if p := pipelineRow(m, "Jenkinsfile"); p.Read != "partial" || len(m.Unread) != 1 || m.Unread[0].Reason != "dynamic_stages" {
		t.Errorf("read = %+v unread = %+v", p, m.Unread)
	}
}

func TestJenkinsSharedLibraryInWorkspace(t *testing.T) {
	m := Build(ws(map[string]string{
		"repo-app/Jenkinsfile": "@Library('acme') _\nbuildService(name: 'app')\n",
		"repo-lib/vars/buildService.groovy": `def call(Map cfg) {
  pipeline {
    agent any
    stages {
      stage('Build') { steps { sh 'mvn -B package' } }
      stage('Deploy') { steps { sh 'helm upgrade --install x ./chart' } }
    }
  }
}
`,
	}))
	var local string
	for _, h := range m.Handoffs {
		if h.Kind == "library_step" {
			local = h.LocalPipeline
		}
	}
	if local != "repo-lib/vars/buildService.groovy" {
		t.Errorf("library step resolved to %q", local)
	}
	if p := pipelineRow(m, "repo-lib/vars/buildService.groovy"); p == nil || p.Kind != "library" || p.Jobs != 2 {
		t.Errorf("library = %+v", p)
	}
}

// ---------------------------------------------------------------------------
// Tier 2
// ---------------------------------------------------------------------------

func TestAzurePipelines(t *testing.T) {
	m := Build(ws(map[string]string{
		"azure-pipelines.yml": `trigger: [main]
resources:
  repositories:
    - repository: templates
      type: git
      name: acme/templates
      ref: refs/tags/v3
pool:
  vmImage: ubuntu-latest
stages:
- stage: Build
  jobs:
  - job: Compile
    steps:
    - task: Maven@4
      inputs: {goals: package}
    - task: Docker@2
      inputs:
        command: buildAndPush
        repository: acme/api
        Dockerfile: Dockerfile
- stage: Deploy
  jobs:
  - deployment: Prod
    environment: production
    strategy:
      runOnce:
        deploy:
          steps:
          - task: KubernetesManifest@1
            inputs:
              manifests: k8s/deploy.yaml
  - template: jobs/smoke.yml@templates
`,
	}))
	jobs := jobsOf(m, "azure-pipelines.yml")
	if jobs["Deploy.Prod"] == nil || jobs["Deploy.Prod"].Needs != "Build.Compile" || jobs["Deploy.Prod"].Environment != "production" {
		t.Errorf("jobs = %+v", jobs)
	}
	if got := handoffsOf(m, "azure-pipelines.yml"); !reflect.DeepEqual(got, []string{"template templates/jobs/smoke.yml@v3 version"}) {
		t.Errorf("handoffs = %v", got)
	}
}

func TestBitbucketPipelines(t *testing.T) {
	m := Build(ws(map[string]string{
		"bitbucket-pipelines.yml": `image: node:20
pipelines:
  branches:
    main:
      - step:
          name: Build
          script: [npm ci, npm run build]
      - parallel:
          - step: {name: Lint, script: [npm run lint]}
          - step: {name: Test, script: [npm test]}
      - step:
          name: Deploy
          deployment: production
          trigger: manual
          script:
            - pipe: atlassian/aws-s3-deploy:1.1.0
`,
	}))
	jobs := jobsOf(m, "bitbucket-pipelines.yml")
	d := jobs["branches: main › Deploy"]
	if d == nil || d.Needs != "branches: main › Lint, branches: main › Test" || d.Gate != "manual" || d.Environment != "production" {
		t.Errorf("deploy = %+v", d)
	}
	if got := handoffsOf(m, "bitbucket-pipelines.yml"); !reflect.DeepEqual(got, []string{"pipe atlassian/aws-s3-deploy@1.1.0 version"}) {
		t.Errorf("handoffs = %v", got)
	}
}

func TestCircleCI(t *testing.T) {
	m := Build(ws(map[string]string{
		".circleci/config.yml": `version: 2.1
orbs:
  node: circleci/node@5.2.0
jobs:
  build:
    docker: [{image: cimg/node:20.0}]
    steps: [checkout, {run: npm ci}, {run: {name: Test, command: npm test}}]
workflows:
  main:
    jobs:
      - build
      - node/test
      - hold: {type: approval, requires: [build]}
      - build:
          name: release
          requires: [hold]
          filters: {branches: {only: main}}
`,
	}))
	jobs := jobsOf(m, ".circleci/config.yml")
	if jobs["main › hold"].Gate != "approval" || jobs["main › release"].Needs != "main › hold" {
		t.Errorf("jobs = %+v", jobs)
	}
	if got := handoffsOf(m, ".circleci/config.yml"); !reflect.DeepEqual(got, []string{"orb circleci/node node/test@5.2.0 version"}) {
		t.Errorf("handoffs = %v", got)
	}
}

// ---------------------------------------------------------------------------
// Dockerfiles and flags
// ---------------------------------------------------------------------------

func TestDockerfileFlags(t *testing.T) {
	m := Build(ws(map[string]string{
		"Dockerfile": "ARG API_TOKEN\nFROM node:latest\nENV DB_PASSWORD=x NODE_ENV=production\nADD https://example.com/x.tgz /x\nCOPY . .\nEXPOSE 8080\n",
		"index.js":   "",
	}))
	if got := flagsOf(m); !reflect.DeepEqual(got, []string{"add_remote_url", "base_unpinned_tag", "copy_all_no_dockerignore", "runs_as_root", "secret_like_key", "secret_like_key"}) {
		t.Errorf("flags = %v", got)
	}
	if len(m.Dockerfiles) != 1 || m.Dockerfiles[0].Ports != "8080" || m.Dockerfiles[0].Dockerignore != 0 {
		t.Errorf("dockerfiles = %+v", m.Dockerfiles)
	}
	if s := renderModel(m); strings.Contains(s, "DB_PASSWORD=x") {
		t.Error("an ENV value reached the model")
	}
}

// A clean Dockerfile and a clean pipeline raise nothing.
func TestCleanRaisesNoFlags(t *testing.T) {
	m := Build(ws(map[string]string{
		"Dockerfile":    "FROM node:20.11-alpine@sha256:" + strings.Repeat("a", 64) + "\nUSER node\nCOPY src /app/src\n",
		"src/i.js":      "",
		".dockerignore": "node_modules\n",
		".github/workflows/ci.yml": `on: push
permissions: {contents: read}
jobs:
  t:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - run: npm ci && npm test
`,
	}))
	if got := flagsOf(m); len(got) != 0 {
		t.Errorf("flags = %v", got)
	}
}

func TestTokenLiteralAndPrivileged(t *testing.T) {
	m := Build(ws(map[string]string{
		"Jenkinsfile": "node {\n  stage('x') {\n    sh 'docker login -u ci -p s3cretpass registry.acme && docker run --privileged acme/tool'\n    sh 'rancher login https://rancher --token $TOKEN'\n  }\n}\n",
	}))
	if got := flagsOf(m); !reflect.DeepEqual(got, []string{"docker_privileged", "token_literal"}) {
		t.Errorf("flags = %v", got)
	}
	if s := renderModel(m); strings.Contains(s, "s3cretpass") {
		t.Error("a password reached the model")
	}
}
