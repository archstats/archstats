package cicd

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
		kind, keep := KindOf(p, []byte(c))
		if keep {
			in.Files[p] = []byte(c)
		}
		if kind != "" {
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
	for _, d := range m.Artifacts {
		out = append(out, d.ID)
	}
	return out
}

func findArtifact(m *Model, id string) *Artifact {
	for _, d := range m.Artifacts {
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
	fe := findArtifact(m, "frontend")
	// Two Go files; the Dockerfile builds the image and is not in the count.
	if fe.BuiltBy != "skaffold" || fe.Runtime != "go 1.22" || fe.BaseImage != "gcr.io/distroless/static" || fe.Files != 2 {
		t.Errorf("frontend = %+v", fe)
	}
	cart := findArtifact(m, "cartservice")
	// Tests do not ship: the image holds Cart.cs, and CartTest.cs is a test.
	if cart.Files != 1 || cart.Runtime != "dotnet 8.0" {
		t.Errorf("cartservice = %+v", cart)
	}
	var reasons []string
	for _, u := range m.Unresolved {
		reasons = append(reasons, u.Ref+":"+u.Reason)
	}
	sort.Strings(reasons)
	if !reflect.DeepEqual(reasons, []string{"redis:alpine:external"}) {
		t.Errorf("unresolved = %v", reasons)
	}
	var outs []string
	for _, o := range m.Outputs {
		if o.Artifact != "" {
			outs = append(outs, o.Artifact+" "+o.Kind+" "+o.Resolution)
		}
	}
	sort.Strings(outs)
	if !reflect.DeepEqual(outs, []string{"cartservice deploy path", "cartservice image repository", "frontend deploy path", "frontend image repository"}) {
		t.Errorf("outputs = %v", outs)
	}
}

// Nothing from a secret key survives into the model.
func TestSecretValuesNeverReachTheModel(t *testing.T) {
	m := Build(ws(shopFiles()))
	dump := renderModel(m)
	if strings.Contains(dump, "sk-live-123") {
		t.Fatal("a secret value leaked into the model")
	}
}

// renderModel writes every exported row of a model.
func renderModel(m *Model) string {
	var b strings.Builder
	for _, d := range m.Artifacts {
		// Exported fields only: the unexported ones hold pointers.
		fmt.Fprintf(&b, "%s %s %s %s %s:%d %s %s %s %s %s %d %d\n", d.ID, d.Name, d.Kind, d.Repository, d.File, d.Line, d.BuiltBy, d.Context, d.Dockerfile, d.BaseImage, d.Runtime, d.Files, d.Components)
	}
	for _, rows := range []interface{}{m.Contents, m.Components, m.Unresolved, m.Pipelines, m.Unread, m.Triggers, m.Jobs, m.Steps, m.Handoffs, m.Outputs, m.Dockerfiles, m.Stages, m.Flags} {
		v := reflect.ValueOf(rows)
		for i := 0; i < v.Len(); i++ {
			fmt.Fprintf(&b, "%+v\n", v.Index(i).Elem().Interface())
		}
	}
	return b.String()
}

// The model is the same every time: maps are never ranged into output.
func TestBuildIsDeterministic(t *testing.T) {
	first := renderModel(Build(ws(shopFiles())))
	for i := 0; i < 20; i++ {
		if got := renderModel(Build(ws(shopFiles()))); got != first {
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
	booking := findArtifact(m, "qp-booking")
	if booking.Kind != "app" || booking.BuiltBy != "delegated" || booking.Runtime != "java 8" {
		t.Errorf("booking = %+v", booking)
	}
	audit := findArtifact(m, "qp-audit")
	if audit.BuiltBy != "maven" {
		t.Errorf("audit builds itself with mvn in its Jenkinsfile: %+v", audit)
	}
	// Both carry the shared library, through their declared dependency.
	var common []string
	for _, c := range m.Contents {
		if c.Module == "qp-common" {
			common = append(common, c.Artifact+":"+c.Resolution)
		}
	}
	if !reflect.DeepEqual(common, []string{"qp-audit:module_dependency", "qp-booking:module_dependency"}) {
		t.Errorf("qp-common contents = %v", common)
	}
	// The pipeline hands its build to a central workflow, which is named
	// with the ref it is pinned to.
	var hs []string
	for _, h := range m.Handoffs {
		hs = append(hs, h.Kind+" "+h.Target+"@"+h.Ref+" "+h.Pinned)
	}
	for _, want := range []string{
		"reusable_workflow Acme/cicd-workflows/.github/workflows/build-maven.yml@main branch",
		"shared_library jsl@ none",
	} {
		if !containsStr(hs, want) {
			t.Errorf("missing %q in %v", want, hs)
		}
	}
	if s := renderModel(m); strings.Contains(s, "hunter2") {
		t.Fatal("a password reached the model")
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
	if s := renderModel(m); strings.Contains(s, "u:p@") {
		t.Error("credentials in a URL must not be kept")
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
	if findArtifact(m, "webapp").Runtime != "dotnet 9.0" {
		t.Errorf("runtime = %q", findArtifact(m, "webapp").Runtime)
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
