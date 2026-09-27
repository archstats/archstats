package deployables

import (
	"fmt"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/archstats/archstats/core/file"
	"github.com/archstats/archstats/core/module"
)

// ws builds linker input from files written inline. Every path is walked;
// roles and components come from the path, as a scan would give them.
func ws(files map[string]string, modules ...*module.Module) *Input {
	in := &Input{Files: map[string][]byte{}, Kinds: map[string]string{}, Roles: map[string]string{}, Components: map[string]string{}, Modules: module.New(modules...)}
	for p, c := range files {
		in.AllFiles = append(in.AllFiles, p)
		in.Roles[p] = file.Role(p, false, false)
		kind := file.SystemKind(p, []byte(c))
		if strings.HasSuffix(p, "Program.cs") && strings.Contains(c, "DistributedApplication") {
			kind = "aspire"
		}
		if kind != "" {
			in.Files[p] = []byte(c)
			in.Kinds[p] = kind
		}
		if strings.HasSuffix(p, ".go") || strings.HasSuffix(p, ".java") || strings.HasSuffix(p, ".py") || strings.HasSuffix(p, ".cs") {
			in.Components[p] = dirOf(p)
		}
	}
	sort.Strings(in.AllFiles)
	in.RepoOf = func(p string) string {
		if i := strings.Index(p, "/"); i > 0 && strings.HasPrefix(p, "repo-") {
			return p[:i]
		}
		return "shop"
	}
	return in
}

func ids(m *Model) []string {
	var out []string
	for _, d := range m.Deployables {
		out = append(out, d.ID)
	}
	return out
}

func linkStrings(m *Model, kinds ...string) []string {
	var out []string
	for _, l := range m.Links {
		if len(kinds) > 0 && !contains(kinds, l.Kind) {
			continue
		}
		out = append(out, fmt.Sprintf("%s -%s-> %s", l.From, l.Kind, l.To))
	}
	return out
}

func findDeployable(m *Model, id string) *Deployable {
	for _, d := range m.Deployables {
		if d.ID == id {
			return d
		}
	}
	return nil
}

// A small microservices-demo: Skaffold names the images, Kubernetes runs
// them and says through env vars who calls whom.
func shopFiles() map[string]string {
	return map[string]string{
		"skaffold.yaml": `apiVersion: skaffold/v4beta1
kind: Config
build:
  artifacts:
  - image: frontend
    context: src/frontend
  - image: cartservice
    context: src/cartservice
    docker:
      dockerfile: src/Dockerfile
`,
		"src/frontend/Dockerfile":           "FROM golang:1.22 AS b\nCOPY . .\nRUN go build\nFROM gcr.io/distroless/static\nCOPY --from=b /app /app\n",
		"src/frontend/main.go":              "package main",
		"src/frontend/handlers.go":          "package main",
		"src/cartservice/src/Dockerfile":    "FROM mcr.microsoft.com/dotnet/sdk:8.0 AS b\nCOPY . .\nFROM mcr.microsoft.com/dotnet/aspnet:8.0\nCOPY --from=b /out /app\n",
		"src/cartservice/src/Cart.cs":       "class Cart {}",
		"src/cartservice/tests/CartTest.cs": "class CartTest {}",
		"kubernetes-manifests/frontend.yaml": `apiVersion: apps/v1
kind: Deployment
metadata:
  name: frontend
spec:
  template:
    metadata:
      labels:
        app: frontend
    spec:
      containers:
      - name: server
        image: frontend
        env:
        - name: CART_SERVICE_ADDR
          value: "cartservice:7070"
        - name: AD_SERVICE_ADDR
          value: "adservice:9555"
        - name: API_KEY
          value: "sk-live-123"
---
apiVersion: v1
kind: Service
metadata:
  name: frontend
spec:
  selector:
    app: frontend
`,
		"kubernetes-manifests/cartservice.yaml": `apiVersion: apps/v1
kind: Deployment
metadata:
  name: cartservice
spec:
  template:
    metadata:
      labels:
        app: cartservice
    spec:
      containers:
      - name: server
        image: cartservice
        env:
        - name: REDIS_ADDR
          value: "redis-cart:6379"
---
apiVersion: v1
kind: Service
metadata:
  name: cartservice
spec:
  selector:
    app: cartservice
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: redis-cart
spec:
  template:
    metadata:
      labels:
        app: redis-cart
    spec:
      containers:
      - name: redis
        image: redis:alpine
---
apiVersion: v1
kind: Service
metadata:
  name: redis-cart
spec:
  selector:
    app: redis-cart
`,
		".github/workflows/ci.yaml": `name: CI
on:
  push:
    paths: ['src/**']
jobs:
  build:
    steps:
      - run: skaffold build --default-repo=gcr.io/p
      - run: kubectl apply -f kubernetes-manifests/
`,
	}
}

func TestShopDeployablesContentsAndLinks(t *testing.T) {
	m := Build(ws(shopFiles()))
	if got := ids(m); !reflect.DeepEqual(got, []string{"cartservice", "frontend"}) {
		t.Fatalf("deployables = %v", got)
	}
	fe := findDeployable(m, "frontend")
	// Two Go files; the Dockerfile builds the image and is not in the count.
	if fe.BuiltBy != "skaffold" || fe.Runtime != "go 1.22" || fe.BaseImage != "gcr.io/distroless/static" || fe.Files != 2 {
		t.Errorf("frontend = %+v", fe)
	}
	cart := findDeployable(m, "cartservice")
	// Tests do not ship: the image holds Cart.cs, and CartTest.cs is a test.
	if cart.Files != 1 || cart.Runtime != "dotnet 8.0" {
		t.Errorf("cartservice = %+v", cart)
	}
	want := []string{"cartservice -calls-> redis-cart", "frontend -calls-> cartservice"}
	if got := linkStrings(m, "calls"); !reflect.DeepEqual(got, want) {
		t.Errorf("calls = %v", got)
	}
	var reasons []string
	for _, u := range m.Unresolved {
		reasons = append(reasons, u.Ref+":"+u.Reason)
	}
	sort.Strings(reasons)
	if !reflect.DeepEqual(reasons, []string{"adservice:not_found", "redis:alpine:external"}) {
		t.Errorf("unresolved = %v", reasons)
	}
	var pd []string
	for _, p := range m.PipelineDeployables {
		pd = append(pd, p.Deployable+" "+p.Action+" "+p.Resolution)
	}
	if !reflect.DeepEqual(pd, []string{"cartservice builds repository", "cartservice deploys path", "frontend builds repository", "frontend deploys path"}) {
		t.Errorf("pipeline links = %v", pd)
	}
}

// Nothing from a secret key survives into the model.
func TestSecretValuesNeverReachTheModel(t *testing.T) {
	m := Build(ws(shopFiles()))
	dump := fmt.Sprintf("%+v %+v %+v %+v", m.Links, m.Unresolved, m.EnvValues, m.Dependencies)
	for _, d := range m.Links {
		dump += fmt.Sprintf("%+v", *d)
	}
	for _, v := range m.EnvValues {
		dump += fmt.Sprintf("%+v", *v)
	}
	if strings.Contains(dump, "sk-live-123") {
		t.Fatal("a secret value leaked into the model")
	}
}

// The model is the same every time: maps are never ranged into output.
func TestBuildIsDeterministic(t *testing.T) {
	render := func() string {
		m := Build(ws(shopFiles()))
		var b strings.Builder
		for _, d := range m.Deployables {
			// Exported fields only: the unexported ones hold pointers.
			fmt.Fprintf(&b, "%s %s %s %s %s:%d %s %s %s %s %d %d\n", d.ID, d.Name, d.Kind, d.Repository, d.File, d.Line, d.BuiltBy, d.Context, d.BaseImage, d.Runtime, d.Files, d.Components)
		}
		for _, c := range m.Contents {
			fmt.Fprintf(&b, "%+v\n", *c)
		}
		for _, l := range m.Links {
			fmt.Fprintf(&b, "%+v\n", *l)
		}
		for _, u := range m.Unresolved {
			fmt.Fprintf(&b, "%+v\n", *u)
		}
		for _, p := range m.PipelineDeployables {
			fmt.Fprintf(&b, "%+v\n", *p)
		}
		for _, dep := range m.Dependencies {
			fmt.Fprintf(&b, "%+v\n", *dep)
		}
		return b.String()
	}
	first := render()
	for i := 0; i < 20; i++ {
		if got := render(); got != first {
			t.Fatalf("run %d differs:\n%s\nvs\n%s", i, got, first)
		}
	}
}

// The client pattern: a Spring Boot service, no Dockerfile, a pipeline that
// hands its build and deployment to a central repository, per-environment
// values files for a chart that lives elsewhere, and a shared library in
// another repository of the same workspace.
func clientFiles() (map[string]string, []*module.Module) {
	files := map[string]string{
		"repo-booking/pom.xml": `<project><groupId>com.acme</groupId><artifactId>qp-booking</artifactId>
<parent><groupId>org.springframework.boot</groupId><artifactId>spring-boot-starter-parent</artifactId><version>2.3.3.RELEASE</version></parent>
<properties><java.version>1.8</java.version></properties>
<dependencies>
  <dependency><groupId>com.acme</groupId><artifactId>qp-common</artifactId><version>1.0</version></dependency>
  <dependency><groupId>org.springframework.kafka</groupId><artifactId>spring-kafka</artifactId></dependency>
</dependencies>
<build><plugins><plugin><artifactId>spring-boot-maven-plugin</artifactId></plugin></plugins></build></project>`,
		"repo-booking/src/main/java/com/acme/booking/App.java":   "class App {}",
		"repo-booking/src/main/resources/application.properties": "spring.application.name=qp-booking\nqp.audit.url=http://qp-audit:8080/audit\ndocs.url=https://qp-document-storage/api\ntopic.name=qp_topic\nspring.datasource.url=jdbc:postgresql://db.acme.internal:5432/quotes\nspring.datasource.password=hunter2\n",
		"repo-booking/helm-release/nonprod-dev.yaml":             "replicaCount: 1\nclusterId: nonprod\nextraVars:\n  - name: DB_PASSWORD\n    value: hunter2\n",
		"repo-booking/helm-release/prod.yaml":                    "replicaCount: 3\nclusterId: prod\n",
		"repo-booking/.github/workflows/cicd.yml": `name: CICD
on:
  workflow_dispatch:
    inputs:
      environment:
        type: choice
        options: [dev, prod]
  push:
jobs:
  build:
    uses: Acme/cicd-workflows/.github/workflows/build-maven.yml@main
    with:
      java-version: 8
  deploy:
    uses: Acme/cicd-workflows/.github/workflows/deployment-nonproduction.yml@main
`,
		"repo-audit/pom.xml": `<project><groupId>com.acme</groupId><artifactId>qp-audit</artifactId>
<dependencies><dependency><groupId>com.acme</groupId><artifactId>qp-common</artifactId></dependency></dependencies>
<build><plugins><plugin><artifactId>spring-boot-maven-plugin</artifactId></plugin></plugins></build></project>`,
		"repo-audit/src/main/java/com/acme/audit/App.java":    "class App {}",
		"repo-audit/src/main/resources/application.yml":       "spring:\n  application:\n    name: qp-audit\ntopic:\n  name: qp_topic\nspring.datasource.url: jdbc:postgresql://db.acme.internal:5432/quotes\n",
		"repo-audit/Jenkinsfile":                              "@Library('jsl') _\npipeline {\n  stages {\n    stage('Build') { steps { sh 'mvn package' } }\n  }\n}\n",
		"repo-common/pom.xml":                                 `<project><groupId>com.acme</groupId><artifactId>qp-common</artifactId></project>`,
		"repo-common/src/main/java/com/acme/common/Util.java": "class Util {}",
	}
	mods := []*module.Module{
		{Name: "qp-booking", Dir: "repo-booking", Manifest: "repo-booking/pom.xml", Kind: "maven", DependsOn: []string{"qp-common", "spring-kafka"}},
		{Name: "qp-audit", Dir: "repo-audit", Manifest: "repo-audit/pom.xml", Kind: "maven", DependsOn: []string{"qp-common"}},
		{Name: "qp-common", Dir: "repo-common", Manifest: "repo-common/pom.xml", Kind: "maven"},
	}
	return files, mods
}

func TestClientDelegatedServices(t *testing.T) {
	files, mods := clientFiles()
	m := Build(ws(files, mods...))
	if got := ids(m); !reflect.DeepEqual(got, []string{"qp-audit", "qp-booking"}) {
		t.Fatalf("deployables = %v (a library is not a deployable)", got)
	}
	booking := findDeployable(m, "qp-booking")
	if booking.Kind != "app" || booking.BuiltBy != "delegated" || booking.Runtime != "java 8" {
		t.Errorf("booking = %+v", booking)
	}
	audit := findDeployable(m, "qp-audit")
	if audit.BuiltBy != "maven" {
		t.Errorf("audit builds itself with mvn in its Jenkinsfile: %+v", audit)
	}
	// Both carry the shared library, through their declared dependency.
	var common []string
	for _, c := range m.Contents {
		if c.Module == "qp-common" {
			common = append(common, c.Deployable+":"+c.Resolution)
		}
	}
	if !reflect.DeepEqual(common, []string{"qp-audit:module_dependency", "qp-booking:module_dependency"}) {
		t.Errorf("qp-common contents = %v", common)
	}
	got := linkStrings(m)
	for _, want := range []string{
		"qp-booking -calls-> qp-audit",
		"qp-audit -messages-> qp-booking",
		"qp-audit -shares_datastore-> qp-booking",
		"qp-audit -shares_module-> qp-common",
		"qp-booking -shares_module-> qp-common",
	} {
		if !contains(got, want) {
			t.Errorf("missing %q in %v", want, got)
		}
	}
	// A service named like a service but absent from the workspace is
	// reported, not linked to something outside.
	outside := false
	for _, u := range m.Unresolved {
		if u.From == "qp-booking" && u.Ref == "qp-document-storage" && u.Reason == "not_found" {
			outside = true
		}
	}
	if !outside {
		t.Errorf("qp-document-storage should be unresolved: %+v", m.Unresolved)
	}
	var envs []string
	for _, e := range m.Environments {
		envs = append(envs, e.Deployable+":"+e.Environment+":"+e.Source)
	}
	sort.Strings(envs)
	want := []string{"qp-booking:dev:workflow", "qp-booking:nonprod-dev:helm_values", "qp-booking:prod:helm_values", "qp-booking:prod:workflow"}
	if !reflect.DeepEqual(envs, want) {
		t.Errorf("environments = %v", envs)
	}
	for _, v := range m.EnvValues {
		if strings.Contains(v.Value, "hunter2") {
			t.Fatal("a password reached the environment values")
		}
	}
	var deps []string
	for _, d := range m.Dependencies {
		if d.Deployable == "qp-booking" {
			deps = append(deps, d.Role+":"+d.Name+"@"+d.Version)
		}
	}
	sort.Strings(deps)
	wantDeps := []string{"framework:spring-boot@2.3.3.RELEASE", "internal:qp-common@1.0", "library:org.springframework.kafka:spring-kafka@", "runtime:java@8"}
	if !reflect.DeepEqual(deps, wantDeps) {
		t.Errorf("dependencies = %v", deps)
	}
}

func TestPreviewEnvironmentIsAPattern(t *testing.T) {
	files := map[string]string{
		"Dockerfile": "FROM node:20\nCOPY . .\n",
		"index.js":   "",
		".github/workflows/preview.yml": `on: pull_request
jobs:
  deploy:
    environment:
      name: pr-${{ github.event.number }}
    steps:
      - run: docker build -t acme/web .
      - run: helm upgrade --install pr-${{ github.event.number }} ./chart
`,
		"chart/Chart.yaml":  "name: web\n",
		"chart/values.yaml": "image:\n  repository: acme/web\n",
	}
	m := Build(ws(files))
	if got := ids(m); !reflect.DeepEqual(got, []string{"web"}) {
		t.Fatalf("deployables = %v", got)
	}
	if len(m.Environments) != 1 || m.Environments[0].Kind != "pattern" || m.Environments[0].Environment != "per pull request" {
		t.Errorf("environments = %+v", m.Environments)
	}
}

func TestComposeDeclaredAndDependsOn(t *testing.T) {
	files := map[string]string{
		".env": "REGISTRY=ghcr.io/acme\nAPI_PORT=8080\nAPI_ADDR=api:${API_PORT}\n",
		"docker-compose.yml": `services:
  api:
    image: ${REGISTRY}/demo:latest-api
    build:
      context: ./api
    environment:
      - DATABASE_URL=postgres://u:p@db:5432/shop
  web:
    image: ${REGISTRY}/demo:latest-web
    build: ./web
    environment:
      - API_ADDR
    depends_on:
      api:
        condition: service_healthy
      db: {}
  db:
    image: postgres:16
`,
		"api/Dockerfile": "FROM python:3.12-slim\nCOPY . /app\n",
		"api/app.py":     "",
		"web/Dockerfile": "FROM node:22\nCOPY src /app/src\n",
		"web/src/i.js":   "",
	}
	m := Build(ws(files))
	if got := ids(m); !reflect.DeepEqual(got, []string{"api", "web"}) {
		t.Fatalf("deployables = %v (the images share the repository `demo` and go by their service names)", got)
	}
	got := linkStrings(m)
	for _, want := range []string{"web -calls-> api", "web -depends_on-> api", "web -depends_on-> db", "api -uses_datastore-> db/shop"} {
		if !contains(got, want) {
			t.Errorf("missing %q in %v", want, got)
		}
	}
	for _, l := range m.Links {
		if l.Kind == "uses_datastore" && strings.Contains(l.To+l.Via, "u:p") {
			t.Error("credentials in a URL must not be kept")
		}
	}
}

func TestAspireAppHost(t *testing.T) {
	files := map[string]string{
		"src/AppHost/AppHost.csproj": `<Project Sdk="Microsoft.NET.Sdk"><Sdk Name="Aspire.AppHost.Sdk" Version="9" /></Project>`,
		"src/AppHost/Program.cs": `var builder = DistributedApplication.CreateBuilder(args);
var pg = builder.AddPostgres("postgres");
var db = pg.AddDatabase("catalogdb");
var bus = builder.AddRabbitMQ("eventbus");
var catalog = builder.AddProject<Projects.Catalog_API>("catalog-api").WithReference(db).WithReference(bus);
builder.AddProject<Projects.WebApp>("webapp").WithReference(catalog);
`,
		"src/Catalog.API/Catalog.API.csproj": `<Project Sdk="Microsoft.NET.Sdk.Web"><PropertyGroup><TargetFramework>net9.0</TargetFramework></PropertyGroup></Project>`,
		"src/Catalog.API/Api.cs":             "class Api {}",
		"src/WebApp/WebApp.csproj":           `<Project Sdk="Microsoft.NET.Sdk.Web"><PropertyGroup><TargetFramework>net9.0</TargetFramework></PropertyGroup></Project>`,
		"src/WebApp/Page.cs":                 "class Page {}",
	}
	mods := []*module.Module{
		{Name: "Catalog.API", Dir: "src/Catalog.API", Manifest: "src/Catalog.API/Catalog.API.csproj", Kind: "dotnet"},
		{Name: "WebApp", Dir: "src/WebApp", Manifest: "src/WebApp/WebApp.csproj", Kind: "dotnet"},
		{Name: "AppHost", Dir: "src/AppHost", Manifest: "src/AppHost/AppHost.csproj", Kind: "dotnet"},
	}
	m := Build(ws(files, mods...))
	if got := ids(m); !reflect.DeepEqual(got, []string{"catalog-api", "webapp"}) {
		t.Fatalf("deployables = %v", got)
	}
	want := []string{"catalog-api -messages-> eventbus", "catalog-api -uses_datastore-> postgres/catalogdb", "webapp -calls-> catalog-api"}
	if got := linkStrings(m); !reflect.DeepEqual(got, want) {
		t.Errorf("links = %v", got)
	}
	if findDeployable(m, "webapp").Runtime != "dotnet 9.0" {
		t.Errorf("runtime = %q", findDeployable(m, "webapp").Runtime)
	}
}

func TestAmbiguousImageIsRefused(t *testing.T) {
	files := map[string]string{
		"a/Dockerfile": "FROM alpine\nCOPY . .\n",
		"a/x.go":       "",
		"b/Dockerfile": "FROM alpine\nCOPY . .\n",
		"b/y.go":       "",
		"docker-compose.yml": `services:
  a:
    image: acme/api
    build: ./a
  b:
    image: other/api
    build: ./b
`,
		"k8s/run.yaml": "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: api\nspec:\n  template:\n    spec:\n      containers:\n      - image: registry.io/api:1\n",
	}
	m := Build(ws(files))
	found := false
	for _, u := range m.Unresolved {
		if u.Reason == "ambiguous" && strings.HasPrefix(u.Ref, "registry.io/api:1") {
			found = true
		}
	}
	if !found {
		t.Errorf("two images called api must not be chosen between: %+v", m.Unresolved)
	}
}

func TestDevcontainerAndTestDockerfilesAreNotDeployables(t *testing.T) {
	files := map[string]string{
		".devcontainer/Dockerfile":  "FROM mcr.microsoft.com/devcontainers/go\n",
		"tests/fixtures/Dockerfile": "FROM alpine\n",
		"service/Dockerfile":        "FROM alpine\nCOPY app /app\n",
		"service/app/main.go":       "",
	}
	m := Build(ws(files))
	if got := ids(m); !reflect.DeepEqual(got, []string{"service"}) {
		t.Errorf("deployables = %v", got)
	}
}

func TestGlobRegexp(t *testing.T) {
	cases := []struct {
		glob, path string
		want       bool
	}{
		{"src/**", "src/a/b.go", true},
		{"src/**", "srcx/a.go", false},
		{"**/*.md", "docs/a.md", true},
		{"**/*.md", "a.md", true},
		{"api/*.go", "api/x.go", true},
		{"api/*.go", "api/sub/x.go", false},
		{"packages/**/src/**", "packages/a/src/x.ts", true},
	}
	for _, c := range cases {
		if got := globRegexp(c.glob).MatchString(c.path); got != c.want {
			t.Errorf("%s ~ %s = %v", c.glob, c.path, got)
		}
	}
}

func TestDockerfileName(t *testing.T) {
	cases := map[string]string{
		"src/cartservice/src/Dockerfile":       "cartservice",
		"src/cartservice/src/Dockerfile.debug": "cartservice-debug",
		"Dockerfile":                           "root",
		"Dockerfile.multi":                     "multi",
		"docker/api.Dockerfile":                "api",
		"services/web/docker/Dockerfile":       "web",
	}
	for in, want := range cases {
		if got := dockerfileName(in); got != want {
			t.Errorf("%s: %s, want %s", in, got, want)
		}
	}
}

func TestValuesEnvironment(t *testing.T) {
	cases := []struct {
		file  string
		chart bool
		want  string
	}{
		{"chart/values.yaml", true, ""},
		{"chart/values-prod.yaml", true, "prod"},
		{"chart/values.staging.yaml", true, "staging"},
		{"chart/dev.values.yaml", true, "dev"},
		{"helm-release/nonprod-dev.yaml", false, "nonprod-dev"},
		{"deploy/environments/uat.yaml", false, "uat"},
		{"chart/templates-ish.yaml", true, ""},
		{"config/features.yaml", false, ""},
	}
	for _, c := range cases {
		if got := valuesEnvironment(c.file, c.chart); got != c.want {
			t.Errorf("%s: %q, want %q", c.file, got, c.want)
		}
	}
}

func TestNoTargetIsTheLastStage(t *testing.T) {
	files := map[string]string{
		"Dockerfile":                    "FROM node:20 AS base\nCOPY package.json .\nFROM base AS node\nCOPY api ./api\n",
		"api/i.js":                      "",
		".github/workflows/release.yml": "on: push\njobs:\n  b:\n    steps:\n      - run: docker build --target node -t acme/librechat .\n      - run: docker build --target node -t acme/librechat:v2 .\n",
		".github/workflows/old.yml":     "on: push\njobs:\n  b:\n    steps:\n      - run: docker build -t other/runner-image .\n",
	}
	m := Build(ws(files))
	if got := ids(m); !reflect.DeepEqual(got, []string{"librechat"}) {
		t.Errorf("one image under two names, called what most builds call it: %v", got)
	}
}
