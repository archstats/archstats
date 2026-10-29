package navigation

import (
	"path"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// A contract is what a team says its API is: an OpenAPI 3 or Swagger 2
// document. Read here so a route can be checked against it, and so a client
// generated from someone else's contract says which operations it calls.

// The largest contract read. Specs are big but never this big.
const maxSpecSize = 16 << 20

type spec struct {
	File     string
	Format   string // openapi3, swagger2
	Version  string // the document's openapi/swagger field
	Title    string
	APIVer   string // info.version
	Servers  []specServer
	Contact  bool
	Ops      []specOp
	Line     int
	InfoLine int
	// Coverage of two design rules over the whole document: operations that
	// declare an API-Version response header, and error responses written as
	// problem+json.
	VersionHeaders int
	ErrorBodies    int
	ProblemBodies  int
}

type specServer struct {
	URL  string
	Line int
}

type specOp struct {
	Method, Path, OperationID, Tag, Summary, Security string
	Deprecated                                        bool
	Line                                              int
}

var httpMethods = map[string]bool{"get": true, "put": true, "post": true, "delete": true, "options": true, "head": true, "patch": true, "trace": true}

// looksLikeSpec is a cheap test before parsing: the version key near the top.

func looksLikeSpec(content []byte) bool {
	head := string(content[:min(len(content), 4096)])
	return rx(`(?m)^\s*["']?(openapi|swagger)["']?\s*:\s*["']?[23]\.`).MatchString(head)
}

// readSpec parses a contract, or returns nil for a file that is not one.
func readSpec(p string, content []byte) *spec {
	if len(content) > maxSpecSize || !looksLikeSpec(content) {
		return nil
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(content, &doc); err != nil || len(doc.Content) == 0 || doc.Content[0].Kind != yaml.MappingNode {
		return nil
	}
	root := doc.Content[0]
	s := &spec{File: p}
	if v := child(root, "openapi"); v != nil {
		s.Format, s.Version, s.Line = "openapi3", v.Value, v.Line
	} else if v := child(root, "swagger"); v != nil {
		s.Format, s.Version, s.Line = "swagger2", v.Value, v.Line
	} else {
		return nil
	}
	if info := child(root, "info"); info != nil {
		s.InfoLine = info.Line
		if t := child(info, "title"); t != nil {
			s.Title = t.Value
		}
		if v := child(info, "version"); v != nil {
			s.APIVer = v.Value
		}
		s.Contact = child(info, "contact") != nil
	}
	if servers := child(root, "servers"); servers != nil && servers.Kind == yaml.SequenceNode {
		for _, sv := range servers.Content {
			if u := child(sv, "url"); u != nil {
				s.Servers = append(s.Servers, specServer{URL: u.Value, Line: u.Line})
			}
		}
	}
	if bp := child(root, "basePath"); bp != nil {
		scheme := "https"
		if schemes := child(root, "schemes"); schemes != nil && schemes.Kind == yaml.SequenceNode && len(schemes.Content) > 0 {
			scheme = schemes.Content[0].Value
		}
		host := "host"
		if h := child(root, "host"); h != nil {
			host = h.Value
		}
		s.Servers = append(s.Servers, specServer{URL: scheme + "://" + host + bp.Value, Line: bp.Line})
	}
	global := securityNames(child(root, "security"))
	paths := child(root, "paths")
	if paths == nil || paths.Kind != yaml.MappingNode {
		return s
	}
	for i := 0; i+1 < len(paths.Content); i += 2 {
		p, item := paths.Content[i].Value, paths.Content[i+1]
		if item.Kind != yaml.MappingNode {
			continue
		}
		for j := 0; j+1 < len(item.Content); j += 2 {
			method := strings.ToLower(item.Content[j].Value)
			if !httpMethods[method] {
				continue
			}
			op := item.Content[j+1]
			o := specOp{Method: strings.ToUpper(method), Path: p, Line: item.Content[j].Line, Security: global}
			if v := child(op, "operationId"); v != nil {
				o.OperationID = v.Value
			}
			if v := child(op, "summary"); v != nil {
				o.Summary = v.Value
			}
			if tags := child(op, "tags"); tags != nil && tags.Kind == yaml.SequenceNode && len(tags.Content) > 0 {
				o.Tag = tags.Content[0].Value
			}
			if v := child(op, "deprecated"); v != nil && v.Value == "true" {
				o.Deprecated = true
			}
			if sec := child(op, "security"); sec != nil {
				o.Security = securityNames(sec)
			}
			s.Ops = append(s.Ops, o)
			s.countResponses(child(op, "responses"))
		}
	}
	return s
}

// countResponses adds an operation's responses to the document's coverage
// of the version header and the problem+json error rules.
func (s *spec) countResponses(responses *yaml.Node) {
	if responses == nil || responses.Kind != yaml.MappingNode {
		return
	}
	versioned := false
	for i := 0; i+1 < len(responses.Content); i += 2 {
		code, r := responses.Content[i].Value, responses.Content[i+1]
		if headers := child(r, "headers"); headers != nil && headers.Kind == yaml.MappingNode {
			for j := 0; j < len(headers.Content); j += 2 {
				if strings.EqualFold(headers.Content[j].Value, "api-version") {
					versioned = true
				}
			}
		}
		if ref := child(r, "$ref"); ref != nil && strings.Contains(strings.ToLower(ref.Value), "problem") && (strings.HasPrefix(code, "4") || strings.HasPrefix(code, "5")) {
			s.ErrorBodies++
			s.ProblemBodies++
			continue
		}
		if !(strings.HasPrefix(code, "4") || strings.HasPrefix(code, "5") || code == "default") {
			continue
		}
		content := child(r, "content")
		if content == nil || content.Kind != yaml.MappingNode {
			continue
		}
		s.ErrorBodies++
		for j := 0; j < len(content.Content); j += 2 {
			if strings.Contains(content.Content[j].Value, "problem+json") {
				s.ProblemBodies++
				break
			}
		}
	}
	if versioned {
		s.VersionHeaders++
	}
}

func securityNames(n *yaml.Node) string {
	if n == nil || n.Kind != yaml.SequenceNode {
		return ""
	}
	var names []string
	for _, req := range n.Content {
		for i := 0; i < len(req.Content); i += 2 {
			if !contains(names, req.Content[i].Value) {
				names = append(names, req.Content[i].Value)
			}
		}
	}
	if len(n.Content) > 0 && len(names) == 0 {
		return "none"
	}
	sort.Strings(names)
	return strings.Join(names, ",")
}

func child(n *yaml.Node, key string) *yaml.Node {
	if n == nil || n.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(n.Content); i += 2 {
		if n.Content[i].Value == key {
			return n.Content[i+1]
		}
	}
	return nil
}

// A generator is a build pointed at a contract: openapi-generator in a POM,
// a Gradle build or openapitools.json, NSwag, an OpenApiReference in a
// .csproj, or a WSDL-to-Java plugin. Its side says whether the code it
// writes serves the contract or calls it.
type generator struct {
	Spec      string // as written: a path relative to the build file, or a URL
	Generator string
	Side      string // provided, consumed
	Kind      string // http, soap
	Line      int
}

var (
	mavenExecution  = regexp.MustCompile(`(?s)<inputSpec>\s*([^<]+?)\s*</inputSpec>`)
	mavenGenerator  = regexp.MustCompile(`<generatorName>\s*([^<]+?)\s*</generatorName>`)
	gradleInputSpec = regexp.MustCompile(`\binputSpec\s*(?:\.set\s*\(|=)\s*(?:file\s*\(\s*)?["']([^"']+)["']`)
	gradleGenerator = regexp.MustCompile(`\bgeneratorName\s*(?:\.set\s*\(|=)\s*["']([^"']+)["']`)
	jsonInputSpec   = regexp.MustCompile(`"(?:inputSpec|glob|url)"\s*:\s*"([^"]+\.(?:ya?ml|json)[^"]*)"`)
	jsonGenerator   = regexp.MustCompile(`"generatorName"\s*:\s*"([^"]+)"`)
	csprojReference = regexp.MustCompile(`<OpenApiReference\s+Include\s*=\s*"([^"]+)"[^>]*?(?:/>|>(?s:.*?)</OpenApiReference>)`)
	wsdlSource      = regexp.MustCompile(`<(?:wsdl|wsdlUrl|wsdlFile|wsdlLocation)>\s*([^<]+?)\s*</`)
	wsdlGradle      = regexp.MustCompile(`\bwsdl(?:Url|File|Location|s)?\s*(?:=|\(|:)\s*\[?\s*["']([^"']+\.wsdl[^"']*)["']`)
	nswagDocument   = regexp.MustCompile(`"fromDocument"\s*:\s*\{[^{}]*?"url"\s*:\s*"([^"]+)"`)
)

// Generators that write the server side of a contract; every other one
// writes a client.
var serverGenerators = map[string]bool{
	"spring": true, "kotlin-spring": true, "jaxrs-spec": true, "jaxrs-cxf": true, "jaxrs-cxf-cdi": true, "jaxrs-resteasy": true,
	"jaxrs-resteasy-eap": true, "jaxrs-jersey": true, "aspnetcore": true, "python-fastapi": true, "python-flask": true,
	"python-aiohttp": true, "php-laravel": true, "php-symfony": true, "php-slim4": true, "php-lumen": true, "php-mezzio-ph": true,
	"java-micronaut-server": true, "java-play-framework": true, "java-vertx-web": true, "java-helidon-server": true,
	"java-undertow-server": true, "java-msf4j": true, "java-pkmst": true, "java-inflector": true, "java-camel": true,
	"nodejs-express-server": true, "go-server": true, "go-gin-server": true, "go-echo-server": true, "go-chi-server": true,
	"kotlin-server": true, "kotlin-vertx": true, "rust-server": true, "rust-axum": true, "scala-akka-http-server": true,
	"scala-play-server": true, "scalatra": true, "erlang-server": true, "haskell": true, "graphql-nodejs-express-server": true,
}

func generatorSide(name string) string {
	n := strings.ToLower(strings.TrimSpace(name))
	if serverGenerators[n] || strings.Contains(n, "server") {
		return "provided"
	}
	return "consumed"
}

// generators reads the contracts a build file generates code from.
func generators(p string, content []byte) []generator {
	base := strings.ToLower(path.Base(p))
	text := string(content)
	line := func(off int) int { return strings.Count(text[:off], "\n") + 1 }
	var out []generator
	switch {
	case base == "pom.xml":
		for _, m := range mavenExecution.FindAllStringSubmatchIndex(text, -1) {
			// The generator named nearest the spec, in the same execution.
			window := text[max(0, m[0]-2000):min(len(text), m[1]+2000)]
			name := "java"
			if g := mavenGenerator.FindAllStringSubmatch(window, -1); len(g) > 0 {
				name = nearest(window, g, m[0]-max(0, m[0]-2000))
			}
			out = append(out, generator{Spec: text[m[2]:m[3]], Generator: name, Side: generatorSide(name), Kind: "http", Line: line(m[0])})
		}
		if strings.Contains(text, "cxf-codegen-plugin") || strings.Contains(text, "jaxws-maven-plugin") || strings.Contains(text, "maven-jaxb2-plugin") || strings.Contains(text, "jaxws-tools") {
			for _, m := range wsdlSource.FindAllStringSubmatchIndex(text, -1) {
				out = append(out, generator{Spec: text[m[2]:m[3]], Generator: "wsdl", Side: "consumed", Kind: "soap", Line: line(m[0])})
			}
		}
	case base == "build.gradle" || base == "build.gradle.kts":
		gens := gradleGenerator.FindAllStringSubmatch(text, -1)
		for _, m := range gradleInputSpec.FindAllStringSubmatchIndex(text, -1) {
			name := "java"
			if len(gens) > 0 {
				name = nearest(text, gens, m[0])
			}
			out = append(out, generator{Spec: text[m[2]:m[3]], Generator: name, Side: generatorSide(name), Kind: "http", Line: line(m[0])})
		}
		for _, m := range wsdlGradle.FindAllStringSubmatchIndex(text, -1) {
			out = append(out, generator{Spec: text[m[2]:m[3]], Generator: "wsdl", Side: "consumed", Kind: "soap", Line: line(m[0])})
		}
	case base == "openapitools.json":
		gens := jsonGenerator.FindAllStringSubmatch(text, -1)
		for _, m := range jsonInputSpec.FindAllStringSubmatchIndex(text, -1) {
			name := "typescript-axios"
			if len(gens) > 0 {
				name = nearest(text, gens, m[0])
			}
			out = append(out, generator{Spec: text[m[2]:m[3]], Generator: name, Side: generatorSide(name), Kind: "http", Line: line(m[0])})
		}
	case strings.HasSuffix(base, ".csproj"):
		for _, m := range csprojReference.FindAllStringSubmatchIndex(text, -1) {
			side := "consumed"
			if strings.Contains(text[m[0]:m[1]], "Server") {
				side = "provided"
			}
			out = append(out, generator{Spec: text[m[2]:m[3]], Generator: "nswag", Side: side, Kind: "http", Line: line(m[0])})
		}
	case strings.HasSuffix(base, ".nswag") || base == "nswag.json":
		for _, m := range nswagDocument.FindAllStringSubmatchIndex(text, -1) {
			out = append(out, generator{Spec: text[m[2]:m[3]], Generator: "nswag", Side: "consumed", Kind: "http", Line: line(m[0])})
		}
	}
	return out
}

// nearest is the value among matches written closest to an offset.
func nearest(text string, matches [][]string, offset int) string {
	best, dist := matches[0][1], -1
	for _, m := range matches {
		at := strings.Index(text, m[0])
		d := at - offset
		if d < 0 {
			d = -d
		}
		if dist < 0 || d < dist {
			best, dist = m[1], d
		}
	}
	return strings.TrimSpace(best)
}

// specPath resolves a generator's spec as written to a workspace path:
// Maven's ${project.basedir} and Gradle's $rootDir and $projectDir are the
// build file's directory.
func specPath(buildFile, written string) string {
	w := strings.TrimSpace(written)
	for _, v := range []string{"${project.basedir}/", "${basedir}/", "${project.basedir}", "$rootDir/", "${rootDir}/", "$projectDir/", "${projectDir}/", "$buildDir/../", "file://"} {
		w = strings.TrimPrefix(w, v)
	}
	w = strings.TrimPrefix(w, "./")
	if strings.Contains(w, "://") || strings.HasPrefix(w, "$") {
		return ""
	}
	return path.Clean(path.Join(path.Dir(buildFile), w))
}
