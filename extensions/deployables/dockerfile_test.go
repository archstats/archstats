package deployables

import (
	"reflect"
	"sort"
	"testing"
)

func contentPaths(df *Dockerfile, target *Stage) []string {
	var out []string
	seen := map[string]bool{}
	for _, c := range df.ImageContents(target) {
		key := c.Path
		if c.Pattern != "" {
			key += "|" + c.Pattern
		}
		if !seen[key] {
			seen[key] = true
			out = append(out, key)
		}
	}
	sort.Strings(out)
	return out
}

// LibreChat's Dockerfile.multi, trimmed to its structure: the api image
// copies api/ and config/ itself, and four workspace packages through the
// stages that built them.
const libreChatMulti = `# v0.7.8
# Base node image
FROM node:20-alpine AS base-min
WORKDIR /app
RUN apk --no-cache add curl
RUN npm config set fetch-retry-maxtimeout 600000 && \
    npm config set fetch-retries 5
COPY package*.json ./
COPY packages/data-provider/package*.json ./packages/data-provider/
COPY client/package*.json ./client/
COPY api/package*.json ./api/

FROM base-min AS base
WORKDIR /app
RUN npm ci

FROM base AS data-provider-build
WORKDIR /app/packages/data-provider
COPY packages/data-provider ./
RUN npm run build

FROM base AS mcp-build
WORKDIR /app/packages/mcp
COPY packages/mcp ./
COPY --from=data-provider-build /app/packages/data-provider/dist /app/packages/data-provider/dist
RUN npm run build

FROM base AS data-schemas-build
COPY packages/data-schemas ./
COPY --from=data-provider-build /app/packages/data-provider/dist /app/packages/data-provider/dist

FROM base AS client-build
WORKDIR /app/client
COPY client ./
COPY --from=data-provider-build /app/packages/data-provider/dist /app/packages/data-provider/dist
RUN npm run frontend

FROM base-min AS api-build
COPY --from=ghcr.io/astral-sh/uv:0.6.13 /uv /uvx /bin/
WORKDIR /app
COPY api ./api
COPY config ./config
COPY --from=data-provider-build /app/packages/data-provider/dist ./packages/data-provider/dist
COPY --from=mcp-build /app/packages/mcp/dist ./packages/mcp/dist
COPY --from=data-schemas-build /app/packages/data-schemas/dist ./packages/data-schemas/dist
COPY --from=client-build /app/client/dist ./client/dist
WORKDIR /app/api
EXPOSE 3080
CMD ["node", "server/index.js"]
`

func TestDockerfileLibreChatStages(t *testing.T) {
	df := ParseDockerfile([]byte(libreChatMulti))
	if len(df.Stages) != 7 {
		t.Fatalf("stages = %d, want 7", len(df.Stages))
	}
	api := df.Target("api-build")
	if api == nil {
		t.Fatal("no api-build stage")
	}
	got := contentPaths(df, api)
	want := []string{".|package*.json", "api", "api|package*.json", "client", "client|package*.json", "config",
		"packages/data-provider", "packages/data-provider|package*.json", "packages/data-schemas", "packages/mcp"}
	// `COPY package*.json ./` in base-min copies the root manifests, not the
	// whole context.
	if !reflect.DeepEqual(got, want) {
		t.Errorf("api-build contents = %v\nwant %v", got, want)
	}
	if base, line := df.BaseImage(api); base != "node:20-alpine" || line != 3 {
		t.Errorf("base = %s at %d", base, line)
	}
	if api.Expose[0] != "3080" || api.Entrypoint != `["node", "server/index.js"]` {
		t.Errorf("expose %v entrypoint %q", api.Expose, api.Entrypoint)
	}
	// The data-provider stage alone holds only its own package and the root
	// manifests it inherits.
	dp := df.Target("data-provider-build")
	if got := contentPaths(df, dp); !reflect.DeepEqual(got, []string{".|package*.json", "api|package*.json", "client|package*.json", "packages/data-provider", "packages/data-provider|package*.json"}) {
		t.Errorf("data-provider-build contents = %v", got)
	}
}

func TestDockerfileCopyLinesAreWhereTheInstructionStarts(t *testing.T) {
	df := ParseDockerfile([]byte(libreChatMulti))
	api := df.Target("api-build")
	var apiLine int
	for _, c := range api.Copies {
		if len(c.Sources) == 1 && c.Sources[0] == "api" {
			apiLine = c.Line
		}
	}
	if apiLine != 41 {
		t.Errorf("COPY api line = %d, want 41", apiLine)
	}
}

// microservices-demo's Go services: build on golang, ship on distroless.
func TestDockerfileGoBuilderAndDistroless(t *testing.T) {
	src := `FROM --platform=$BUILDPLATFORM golang:1.23.4-alpine@sha256:abc AS builder
ARG TARGETOS
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /go/bin/frontend .

FROM gcr.io/distroless/static
WORKDIR /src
COPY --from=builder /go/bin/frontend /src/server
COPY ./templates ./templates
COPY ./static ./static
EXPOSE 8080
ENTRYPOINT ["/src/server"]
`
	df := ParseDockerfile([]byte(src))
	final := df.Target("")
	if got := contentPaths(df, final); !reflect.DeepEqual(got, []string{".", "go.mod", "go.sum", "static", "templates"}) {
		t.Errorf("contents = %v", got)
	}
	if base, _ := df.BaseImage(final); base != "gcr.io/distroless/static" {
		t.Errorf("base = %s", base)
	}
	if got := df.BuildImages(final); !reflect.DeepEqual(got, []string{"gcr.io/distroless/static", "golang:1.23.4-alpine@sha256:abc"}) {
		// The final stage's own base comes first because it is visited first.
		t.Errorf("build images = %v", got)
	}
}

// A Spring Boot image that copies a jar built outside the Dockerfile.
func TestDockerfileJavaJarAndArgDefaults(t *testing.T) {
	src := `ARG JAVA_VERSION=17
FROM eclipse-temurin:${JAVA_VERSION}-jre
ARG ARTIFACT_NAME=app
ARG EXPOSED_PORT=8080
COPY target/${ARTIFACT_NAME}.jar /application.jar
EXPOSE ${EXPOSED_PORT}
ENTRYPOINT ["java", "-jar", "/application.jar"]
`
	df := ParseDockerfile([]byte(src))
	st := df.Target("")
	if st.From != "eclipse-temurin:17-jre" {
		t.Errorf("from = %q", st.From)
	}
	if st.Copies[0].Sources[0] != "target/app.jar" {
		t.Errorf("source = %q", st.Copies[0].Sources[0])
	}
	if st.Expose[0] != "8080" {
		t.Errorf("expose = %v", st.Expose)
	}
	if got := contentPaths(df, st); !reflect.DeepEqual(got, []string{"target/app.jar"}) {
		t.Errorf("contents = %v", got)
	}
}

func TestDockerfileUnknownArgIsKeptVisible(t *testing.T) {
	df := ParseDockerfile([]byte("FROM ${BASE_IMAGE}\nCOPY ${SRC} /app\n"))
	st := df.Target("")
	if st.From != "${BASE_IMAGE}" || st.Copies[0].Sources[0] != "${SRC}" {
		t.Errorf("from %q source %q", st.From, st.Copies[0].Sources[0])
	}
}

func TestDockerfileArgDefaultForms(t *testing.T) {
	args := map[string]string{"SET": "x", "EMPTY": ""}
	cases := map[string]string{
		"${SET}":         "x",
		"$SET":           "x",
		"${UNSET:-d}":    "d",
		"${EMPTY:-d}":    "d",
		"${EMPTY-d}":     "",
		"${UNSET-d}":     "d",
		"${SET:+alt}":    "alt",
		"${UNSET:+alt}":  "",
		"${UNSET}":       "${UNSET}",
		"a-${SET}-b":     "a-x-b",
		"no vars":        "no vars",
		"$SET/$UNSET/xy": "x/$UNSET/xy",
	}
	for in, want := range cases {
		if got := substitute(in, args); got != want {
			t.Errorf("%s: %q, want %q", in, got, want)
		}
	}
}

func TestDockerfileStageArgTakesGlobalDefault(t *testing.T) {
	src := "ARG VERSION=3.11\nFROM python:${VERSION}-slim AS base\nARG VERSION\nCOPY requirements-${VERSION}.txt .\n"
	df := ParseDockerfile([]byte(src))
	if got := df.Stages[0].Copies[0].Sources[0]; got != "requirements-3.11.txt" {
		t.Errorf("source = %q", got)
	}
}

func TestDockerfileContinuationsCommentsAndCase(t *testing.T) {
	src := `from alpine:3.19 as Base
# a comment
copy --chown=app:app \
    # a comment inside the continuation is dropped
    src/one \

    src/two \
    /app/
Run echo hi
`
	df := ParseDockerfile([]byte(src))
	st := df.Target("base")
	if st == nil {
		t.Fatal("stage names compare case-insensitively")
	}
	c := st.Copies[0]
	if !reflect.DeepEqual(c.Sources, []string{"src/one", "src/two"}) || c.Dest != "/app/" || c.Line != 3 {
		t.Errorf("copy = %+v", c)
	}
}

func TestDockerfileEscapeDirective(t *testing.T) {
	src := "# escape=`\nFROM mcr.microsoft.com/windows/servercore\nCOPY C:\\src\\app `\n     C:\\app\\\n"
	df := ParseDockerfile([]byte(src))
	c := df.Target("").Copies[0]
	if c.Sources[0] != `C:\src\app` || c.Dest != `C:\app\` {
		t.Errorf("copy = %+v", c)
	}
}

func TestDockerfileEscapeDirectiveOnlyAtTheTop(t *testing.T) {
	src := "FROM alpine\n# escape=`\nCOPY a \\\n  b /x\n"
	df := ParseDockerfile([]byte(src))
	c := df.Target("").Copies[0]
	if !reflect.DeepEqual(c.Sources, []string{"a", "b"}) {
		t.Errorf("a directive after an instruction is a comment; copy = %+v", c)
	}
}

func TestDockerfileHeredocsAreSkipped(t *testing.T) {
	src := `FROM debian:bookworm
RUN <<EOF
apt-get update
COPY this-is-not-an-instruction /nope
EOF
COPY <<-"CONF" /etc/app.conf
	key=value
	CONF
COPY app /app
`
	df := ParseDockerfile([]byte(src))
	st := df.Target("")
	if len(st.Copies) != 1 || st.Copies[0].Sources[0] != "app" || st.Copies[0].Line != 9 {
		t.Errorf("copies = %+v", st.Copies)
	}
}

func TestDockerfileJSONFormAndAdd(t *testing.T) {
	src := `FROM node:20
COPY ["package.json", "yarn.lock", "./"]
ADD https://example.com/tool.tgz /opt/
ADD git@github.com:acme/lib.git /lib
ADD vendor.tar.gz /vendor
COPY --from=0 /x /y
`
	df := ParseDockerfile([]byte(src))
	st := df.Target("")
	if !reflect.DeepEqual(st.Copies[0].Sources, []string{"package.json", "yarn.lock"}) {
		t.Errorf("json form = %+v", st.Copies[0])
	}
	if !st.Copies[1].Remote || len(st.Copies[1].Sources) != 0 || !st.Copies[2].Remote {
		t.Errorf("remote ADDs = %+v %+v", st.Copies[1], st.Copies[2])
	}
	if st.Copies[3].Sources[0] != "vendor.tar.gz" {
		t.Errorf("local ADD = %+v", st.Copies[3])
	}
	// --from=0 names this very stage's index only if it came before; here it
	// is the stage itself, which cannot be a source, so nothing is followed.
	if got := contentPaths(df, st); !reflect.DeepEqual(got, []string{"package.json", "vendor.tar.gz", "yarn.lock"}) {
		t.Errorf("contents = %v", got)
	}
}

func TestDockerfileFromByIndex(t *testing.T) {
	src := "FROM maven:3.9 \nCOPY pom.xml src /build/\nFROM eclipse-temurin:21\nCOPY --from=0 /build/target/app.jar /app.jar\n"
	df := ParseDockerfile([]byte(src))
	if got := contentPaths(df, df.Target("")); !reflect.DeepEqual(got, []string{"pom.xml", "src"}) {
		t.Errorf("contents = %v", got)
	}
}

func TestDockerfileCopyFromAnotherImageIsNotContext(t *testing.T) {
	src := "FROM alpine\nCOPY --from=busybox:1.36 /bin/wget /bin/\nCOPY app /app\n"
	df := ParseDockerfile([]byte(src))
	if got := contentPaths(df, df.Target("")); !reflect.DeepEqual(got, []string{"app"}) {
		t.Errorf("contents = %v", got)
	}
}

func TestDockerfileFromStageInheritsItsContents(t *testing.T) {
	src := "FROM python:3.12 AS deps\nCOPY requirements.txt .\nFROM deps AS app\nCOPY src ./src\n"
	df := ParseDockerfile([]byte(src))
	if got := contentPaths(df, df.Target("app")); !reflect.DeepEqual(got, []string{"requirements.txt", "src"}) {
		t.Errorf("contents = %v", got)
	}
	if base, _ := df.BaseImage(df.Target("app")); base != "python:3.12" {
		t.Errorf("base = %s", base)
	}
}

func TestDockerfileUnknownTargetAndEmptyFile(t *testing.T) {
	df := ParseDockerfile([]byte(libreChatMulti))
	if df.Target("nope") != nil {
		t.Error("an unknown target must not fall back to the last stage")
	}
	empty := ParseDockerfile([]byte("# only a comment\n\n"))
	if len(empty.Stages) != 0 || empty.Target("") != nil || empty.ImageContents(nil) != nil {
		t.Error("no FROM, no stages")
	}
	noFrom := ParseDockerfile([]byte("COPY a /b\nRUN x\n"))
	if len(noFrom.Stages) != 0 {
		t.Error("instructions before FROM belong to no stage")
	}
}

func TestDockerfileCRLFAndTrailingContinuation(t *testing.T) {
	src := "FROM alpine\r\nCOPY a \\\r\n  b /c\r\nCOPY d /e \\"
	df := ParseDockerfile([]byte(src))
	st := df.Target("")
	if len(st.Copies) != 2 || !reflect.DeepEqual(st.Copies[0].Sources, []string{"a", "b"}) {
		t.Errorf("copies = %+v", st.Copies)
	}
}

func TestDockerfileScratchHasNoBuildImage(t *testing.T) {
	src := "FROM golang:1.22 AS b\nCOPY . .\nFROM scratch\nCOPY --from=b /app /app\n"
	df := ParseDockerfile([]byte(src))
	if got := df.BuildImages(df.Target("")); !reflect.DeepEqual(got, []string{"golang:1.22"}) {
		t.Errorf("build images = %v", got)
	}
	if base, _ := df.BaseImage(df.Target("")); base != "scratch" {
		t.Errorf("base = %s", base)
	}
}

func TestSplitSource(t *testing.T) {
	cases := map[string][2]string{
		".":                       {".", ""},
		"./":                      {".", ""},
		"":                        {".", ""},
		"src":                     {"src", ""},
		"./src/":                  {"src", ""},
		"src/*.py":                {"src", "*.py"},
		"package*.json":           {".", "package*.json"},
		"packages/*/package.json": {"packages", "*/package.json"},
		"a/b/../c":                {"a/c", ""},
		"/abs/path":               {"abs/path", ""},
		"target/app.jar":          {"target/app.jar", ""},
		"[ab]c":                   {".", "[ab]c"},
		"dir/file?.txt":           {"dir", "file?.txt"},
		"conf/*/":                 {"conf", "*"},
	}
	for in, want := range cases {
		p, pat := splitSource(in)
		if p != want[0] || pat != want[1] {
			t.Errorf("%q: (%q, %q), want %v", in, p, pat, want)
		}
	}
}

func TestDockerfileSubstituteInCopyUsesStageArgs(t *testing.T) {
	src := "FROM node AS a\nARG DIR=web\nCOPY ${DIR}/src /app\nFROM node AS b\nCOPY ${DIR}/src /app\n"
	df := ParseDockerfile([]byte(src))
	if df.Stages[0].Copies[0].Sources[0] != "web/src" {
		t.Errorf("stage a: %q", df.Stages[0].Copies[0].Sources[0])
	}
	if df.Stages[1].Copies[0].Sources[0] != "${DIR}/src" {
		t.Errorf("a stage ARG does not leak into the next stage: %q", df.Stages[1].Copies[0].Sources[0])
	}
}
