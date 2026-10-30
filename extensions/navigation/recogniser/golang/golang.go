// Package golang reads Go for navigation: net/http and the routers built on
// it, cobra commands, GORM, and Go's HTTP, messaging and data clients.
package golang

import (
	"regexp"
	"strings"

	r "github.com/archstats/archstats/extensions/navigation/recogniser"
)

const golang = "go"

var goType = regexp.MustCompile(`^type\s+([A-Za-z_]\w*)(?:\[[^\]]*\])?\s+(struct|interface|func|map|\[\]|chan|[A-Za-z_*])`)

// Plugin is Go.
func Plugin() r.Plugin {
	return r.Plugin{
		Name: "go",
		Languages: []*r.Language{{
			Name: golang, Extensions: []string{".go"}, Kind: r.Code,
			Syntax: r.Syntax{
				LineComments:  []string{"//"},
				BlockComments: [][2]string{{"/*", "*/"}},
				Strings:       true,
				CharLiterals:  true,
				Backticks:     r.RawBackticks,
				TypeName: func(line string) string {
					if m := goType.FindStringSubmatch(line); m != nil {
						return m[1]
					}
					return ""
				},
				DollarInterpolation: true,
				Statements:          r.NewlineStatementsUnlessDot,
			},
		}},
		Recognisers: []*r.Recogniser{{
			Languages:   []string{golang},
			CallEntries: routes,
			MainEntries: mainEntries,
			Entities:    entities,
			Declared:    declared,
			Constructed: func(typ string) string {
				typ = strings.Replace(typ, ".New", ".", 1)
				return strings.TrimPrefix(typ, "New")
			},
			Modules:     r.ModuleConstructed,
			Constants:   func(s *r.Source, sy *r.Symbols) { r.ConcatConstants(s, sy, goConst) },
			ClientBases: clientBases,
			Calls:       calls,
			// net/http's package functions are read on their own.
			NotClients: []string{"http"},
			OutRules:   outRules,
			CallRules:  callRules,
		}},
	}
}

var (
	goRouteCall  = regexp.MustCompile(`\b([A-Za-z_]\w*)\.(GET|POST|PUT|PATCH|DELETE|HEAD|OPTIONS|Any|Handle|HandleFunc|Get|Post|Put|Patch|Delete|Head|Options|Match|Method|MethodFunc|Static)\s*\(`)
	goGroup      = regexp.MustCompile(`\b([A-Za-z_]\w*)\s*:?=\s*([A-Za-z_]\w*)\.(Group|PathPrefix|Prefix|Party)\s*\(\s*"([^"]*)"`)
	cobraCommand = regexp.MustCompile(`cobra\.Command\s*\{`)
)

// routes are the routes Go's routers declare, and cobra's commands.
func routes(s *r.Source) []r.Entry {
	framework := "net/http"
	for _, f := range []struct{ imp, name string }{{"gin-gonic/gin", "gin"}, {"labstack/echo", "echo"}, {"go-chi/chi", "chi"}, {"gofiber/fiber", "fiber"}, {"gorilla/mux", "gorilla"}, {"kataras/iris", "iris"}, {"julienschmidt/httprouter", "httprouter"}} {
		if strings.Contains(s.Raw, f.imp) {
			framework = f.name
			break
		}
	}
	prefixes := map[string]string{}
	for _, m := range goGroup.FindAllStringSubmatch(s.Code, -1) {
		prefixes[m[1]] = r.JoinRoute(prefixes[m[2]], m[4])
	}
	var out []r.Entry
	for _, loc := range goRouteCall.FindAllStringSubmatchIndex(s.Code, -1) {
		receiver, verb := s.Code[loc[2]:loc[3]], s.Code[loc[4]:loc[5]]
		inner, end := r.Balanced(s.Code, loc[1]-1)
		args := r.SplitArgs(inner)
		if verb == "Match" || verb == "Method" || verb == "MethodFunc" {
			if len(args) < 3 {
				continue
			}
			args = args[1:]
		}
		if len(args) < 2 {
			continue
		}
		p := r.Unquote(args[0])
		method := strings.ToUpper(verb)
		if verb == "Match" || verb == "Method" || verb == "MethodFunc" {
			method = strings.ToUpper(r.Unquote(strings.Split(inner, ",")[0]))
		}
		// Go 1.22's patterns carry the method: "GET /orders/{id}".
		if f := strings.Fields(p); len(f) == 2 && strings.HasPrefix(f[1], "/") {
			method, p = f[0], f[1]
		}
		if !strings.HasPrefix(p, "/") {
			continue
		}
		switch method {
		case "HANDLE", "HANDLEFUNC", "ANY":
			method = "ANY"
			// gorilla: .HandleFunc("/x", h).Methods("GET")
			if m := regexp.MustCompile(`^\s*\.Methods\(\s*"([A-Z]+)"`).FindStringSubmatch(s.Code[end:min(len(s.Code), end+60)]); m != nil {
				method = m[1]
			}
		case "STATIC":
			continue
		}
		e := r.Entry{Kind: r.KindHTTP, Method: method, Path: r.JoinRoute(prefixes[receiver], p), Framework: framework, Line: s.Line(loc[0])}
		if h := args[len(args)-1]; !strings.HasPrefix(h, "func") {
			e.Handler = r.LastName(h)
		}
		out = append(out, e)
	}
	for _, loc := range cobraCommand.FindAllStringIndex(s.Code, -1) {
		body, _ := r.Balanced(s.Code, loc[1]-1)
		if m := regexp.MustCompile(`\bUse:\s*"([^"]+)"`).FindStringSubmatch(body); m != nil {
			e := r.Entry{Kind: r.KindCLI, Path: strings.Fields(m[1])[0], Framework: "cobra", Line: s.Line(loc[0])}
			if h := regexp.MustCompile(`\bRunE?:\s*([A-Za-z_][\w.]*)`).FindStringSubmatch(body); h != nil {
				e.Handler = r.LastName(h[1])
			}
			out = append(out, e)
		}
	}
	return out
}

var (
	goMain    = regexp.MustCompile(`(?m)^func\s+main\s*\(\s*\)`)
	goPackage = regexp.MustCompile(`(?m)^package\s+main\b`)
)

func mainEntries(s *r.Source) []r.Entry {
	if !goPackage.MatchString(s.Code) {
		return nil
	}
	if loc := goMain.FindStringIndex(s.Code); loc != nil {
		return []r.Entry{r.MainEntry(s, loc[0], "")}
	}
	return nil
}

var gormTable = regexp.MustCompile(`func\s*\(\s*\w*\s*\*?([A-Z]\w*)\s*\)\s*TableName\s*\(\s*\)\s*string\s*\{\s*return\s+"([\w.]+)"`)

func entities(s *r.Source) []r.Entity {
	var out []r.Entity
	for _, m := range gormTable.FindAllStringSubmatchIndex(s.Code, -1) {
		out = append(out, r.Entity{Class: s.Code[m[2]:m[3]], Name: s.Code[m[2]:m[3]], Table: s.Code[m[4]:m[5]], Store: "sql", Framework: "gorm", File: s.Path, Line: s.Line(m[0])})
	}
	return out
}

var (
	// Go receivers and parameters: (h *Handler), (svc orders.Service).
	goParam = regexp.MustCompile(`\b([a-z_]\w*)\s+\*?((?:[a-z_]\w*\.)?[A-Z]\w*)\b`)
	goFunc  = regexp.MustCompile(`(?m)^func\s*(\([^)]*\))?\s*\w*\s*(?:\[[^\]]*\])?\s*\(([^)]*)\)`)
	goConst = regexp.MustCompile(`\b([A-Za-z_]\w*)\s*(?:string\s*)?=\s*("[^"\n]*")`)
)

func declared(s *r.Source, sy *r.Symbols) {
	code := s.Code
	for _, m := range goFunc.FindAllStringSubmatch(code, -1) {
		for _, part := range []string{m[1], m[2]} {
			for _, p := range goParam.FindAllStringSubmatch(part, -1) {
				sy.SetVar(p[1], p[2])
			}
		}
	}
	// Struct fields: `svc *orders.Service` on its own line inside a struct.
	for _, loc := range r.Rx(`\bstruct\s*\{`).FindAllStringIndex(code, -1) {
		body, _ := r.Balanced(code, loc[1]-1)
		for _, line := range strings.Split(body, "\n") {
			if p := goParam.FindStringSubmatch(strings.TrimSpace(line)); p != nil && strings.HasPrefix(strings.TrimSpace(line), p[1]) {
				sy.SetVar(p[1], p[2])
			}
		}
	}
}

// clientBases are resty clients built with a base: resty.New().SetBaseURL(...).
func clientBases(s *r.Source, sy *r.Symbols) []r.ClientBase {
	code := s.Code
	if !strings.Contains(code, "resty") {
		return nil
	}
	var out []r.ClientBase
	for _, m := range r.Rx(`\b([a-zA-Z_]\w*)\s*(?::=|=)\s*resty\s*\.\s*New\s*\(\s*\)`).FindAllStringSubmatchIndex(code, -1) {
		stmt := s.StatementFrom(m[0])
		if b := r.Rx(`SetBaseURL\s*\(`).FindStringIndex(stmt); b != nil {
			inner, _ := r.Balanced(stmt, b[1]-1)
			if name := code[m[2]:m[3]]; name != "" && strings.TrimSpace(inner) != "" {
				out = append(out, r.ClientBase{Name: name, Addr: s.RenderAddr(inner, sy), Resilience: r.ResilienceIn(s.Statement(m[0])), Line: s.Line(m[0])})
			}
		}
	}
	return out
}

var (
	restyChain   = regexp.MustCompile(`\.R\s*\(\s*\)((?:\s*\.\s*\w+\s*\([^()]*(?:\([^()]*\)[^()]*)*\))*?)\s*\.\s*(Get|Post|Put|Patch|Delete|Head|Options)\s*\(`)
	goHTTPCall   = regexp.MustCompile(`\bhttp\s*\.\s*(Get|Post|PostForm|Head)\s*\(`)
	goNewRequest = regexp.MustCompile(`\bhttp\s*\.\s*NewRequest(?:WithContext)?\s*\(`)
)

// calls are net/http's package functions and requests, and resty's chains.
func calls(s *r.Source, sy *r.Symbols, add func(r.OutCall, int)) {
	code := s.Code
	for _, m := range goHTTPCall.FindAllStringSubmatchIndex(code, -1) {
		inner, _ := r.Balanced(code, m[1]-1)
		if args := r.SplitArgs(inner); len(args) > 0 {
			method := strings.ToUpper(code[m[2]:m[3]])
			if method == "POSTFORM" {
				method = "POST"
			}
			add(r.OutCall{Kind: "http", Client: "net/http", Method: method, Addr: s.RenderAddr(args[0], sy)}, m[0])
		}
	}
	for _, loc := range goNewRequest.FindAllStringIndex(code, -1) {
		inner, _ := r.Balanced(code, loc[1]-1)
		args := r.SplitArgs(inner)
		if strings.HasSuffix(code[loc[0]:loc[1]], "WithContext(") || len(args) == 4 {
			args = args[1:]
		}
		if len(args) < 2 {
			continue
		}
		method := r.MethodLiteral(args[0])
		if method == "" {
			method = "ANY"
		}
		add(r.OutCall{Kind: "http", Client: "net/http", Method: method, Addr: s.RenderAddr(args[1], sy)}, loc[0])
	}
	for _, m := range restyChain.FindAllStringSubmatchIndex(code, -1) {
		inner, _ := r.Balanced(code, m[1]-1)
		if args := r.SplitArgs(inner); len(args) > 0 {
			add(r.OutCall{Kind: "http", Client: "resty", Method: strings.ToUpper(code[m[4]:m[5]]), Addr: s.RenderAddr(args[0], sy)}, m[0])
		}
	}
}

var callRules = []r.CallRule{
	{Members: regexp.MustCompile(`^(?:Publish|PublishWithContext)$`), Kind: "message", Client: "amqp", Arg: 1, Gate: `(?i)^(ch|channel|amqp\w*|rabbit\w*)$`},
	{Members: regexp.MustCompile(`^(?:Publish|Request)$`), Kind: "message", Client: "nats", Arg: 0, Gate: `(?i)^(nc|js|nats\w*|conn)$`},
}

var outRules = []r.OutRule{
	// Messaging.
	{Re: regexp.MustCompile(`\bsarama\s*\.\s*ProducerMessage\s*\{`), Kind: "message", Client: "kafka", Arg: -1, Key: "Topic"},
	{Re: regexp.MustCompile(`\bkafka\s*\.\s*(?:Writer|Message)\s*\{`), Kind: "message", Client: "kafka", Arg: -1, Key: "Topic"},

	// Databases, caches, storage.
	{Re: regexp.MustCompile(`\bsql\s*\.\s*Open\s*\(`), Kind: "sql", Client: "database/sql", Arg: 1},
	{Re: regexp.MustCompile(`\b(?:postgres|mysql|sqlserver)\s*\.\s*Open\s*\(`), Kind: "sql", Client: "gorm", Arg: 0},
	{Re: regexp.MustCompile(`\bpgxpool\s*\.\s*(?:New|Connect)\s*\(`), Kind: "sql", Client: "pgx", Arg: 1},
	{Re: regexp.MustCompile(`\.\s*ApplyURI\s*\(`), Kind: "nosql", Client: "mongodb", Arg: 0},
	{Re: regexp.MustCompile(`\bredis\s*\.\s*NewClient\s*\(\s*&\s*redis\s*\.\s*Options\s*\{`), Kind: "cache", Client: "go-redis", Arg: -1, Key: "Addr"},
	{Re: regexp.MustCompile(`\b(?:S3Client\s*\.\s*(?:builder|create)|new\s+S3Client|AmazonS3ClientBuilder|new\s+AmazonS3Client|s3\s*\.\s*New(?:FromConfig)?|new\s+BlobServiceClient|new\s+BlobContainerClient|BlobServiceClient\s*\.\s*fromConnectionString|StorageOptions\s*\.\s*getDefaultService|storage\s*\.\s*NewClient|new\s+Storage)\s*\(`), Kind: "storage", Client: "object-storage", Arg: 0},

	// gRPC.
	{Re: regexp.MustCompile(`\b\w+\s*\.\s*New(\w+)Client\s*\(\s*\w+\s*\)`), Kind: "grpc", Client: "grpc-go", Arg: -1},
	{Re: regexp.MustCompile(`\bgrpc\s*\.\s*(?:Dial|DialContext|NewClient)\s*\(`), Kind: "grpc", Client: "grpc-go", Arg: 0},
}
