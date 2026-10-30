// Package ecmascript reads JavaScript and TypeScript for navigation: Express
// and the servers like it, NestJS, file-routed frameworks (Next.js, Nuxt,
// SvelteKit), client-side routers, TypeORM, Mongoose and Sequelize,
// Angular's providers, and the browser's and Node's HTTP, messaging and data
// clients.
package ecmascript

import (
	"path"
	"regexp"
	"strings"

	r "github.com/archstats/archstats/extensions/navigation/recogniser"
)

const js = "js"

var tsAlias = regexp.MustCompile(`^\s*(?:export\s+)?(?:declare\s+)?type\s+([A-Za-z_$][\w$]*)\s*(?:<[^=]*>)?\s*=`)

// Plugin is JavaScript and TypeScript.
func Plugin() r.Plugin {
	return r.Plugin{
		Name: "ecmascript",
		Languages: []*r.Language{{
			Name: js, Extensions: []string{".js", ".jsx", ".mjs", ".cjs", ".ts", ".tsx", ".mts", ".cts", ".vue", ".svelte"}, Kind: r.Code,
			Syntax: r.Syntax{
				LineComments:  []string{"//"},
				BlockComments: [][2]string{{"/*", "*/"}},
				Strings:       true,
				Backticks:     r.TemplateBackticks,
				Marks:         r.AtMarks,
				TypeName: func(line string) string {
					if name := r.CLikeTypeName(line); name != "" {
						return name
					}
					if m := tsAlias.FindStringSubmatch(r.StripMarks(line)); m != nil {
						return m[1]
					}
					return ""
				},
			},
		}},
		Recognisers: []*r.Recogniser{{
			Languages:     []string{js},
			ClassMark:     classMark,
			MemberEntries: memberEntries,
			CallEntries: func(s *r.Source) []r.Entry {
				return append(serverRoutes(s), pageRoutes(s)...)
			},
			MainEntries: mainEntries,
			FileEntries: fileRoutes,
			RouteConfigs: map[string][]string{
				"next":      {"next.config.js", "next.config.mjs", "next.config.ts", "next.config.cjs"},
				"nuxt":      {"nuxt.config.ts", "nuxt.config.js", "nuxt.config.mjs"},
				"sveltekit": {"svelte.config.js", "svelte.config.ts"},
			},
			AuthMarks:   []string{"UseGuards", "Roles", "Public"},
			DocMarks:    []string{"ApiOperation", "ApiResponse", "ApiOkResponse", "ApiTags"},
			Entities:    entities,
			Accesses:    accesses,
			Bindings:    func(s *r.Source) []r.Binding { return r.Pairs(s, angularProv, "angular", false) },
			Scanned:     []string{"Injectable"},
			Declared:    func(s *r.Source, sy *r.Symbols) { r.TypedNames(s, sy); r.ColonTypes(s, sy) },
			Modules:     r.ModuleConstructed,
			Constants:   r.AssignedConstants,
			Security:    security,
			ClientBases: clientBases,
			Calls:       calls,
			ClientName: func(s *r.Source, lt, recv string) string {
				if n := r.ClientLibrary(lt, recv); n != "" {
					return n
				}
				if strings.Contains(lt, "httpclient") || r.LastSegment(recv) == "http" {
					return "angular-http"
				}
				return ""
			},
			OutRules:  outRules,
			CallRules: callRules,
		}},
	}
}

// ---------------------------------------------------------------------------
// Entry points.
// ---------------------------------------------------------------------------

func classMark(m r.Mark, c *r.ClassContext) {
	switch m.Name {
	case "Controller":
		c.Prefix = m.Arg("value", "path", "prefix", "Template", "template")
	case "Processor":
		c.Processor = m.Arg("name", "value")
	}
}

var httpVerbs = map[string]string{"Get": "GET", "Post": "POST", "Put": "PUT", "Delete": "DELETE", "Patch": "PATCH", "Head": "HEAD", "Options": "OPTIONS", "All": "ANY"}

var consumers = map[string]r.Consumer{
	"EventPattern":     {Kind: r.KindMessage, Framework: "nestjs", Names: []string{"value"}},
	"MessagePattern":   {Kind: r.KindMessage, Framework: "nestjs", Names: []string{"value"}},
	"SubscribeMessage": {Kind: r.KindMessage, Framework: "nestjs-websockets", Names: []string{"value"}},
	"OnEvent":          {Kind: r.KindEvent, Framework: "nestjs", Names: []string{"value"}},
	"Cron":             {Kind: r.KindSchedule, Framework: "nestjs", Names: []string{"value"}},
	"Interval":         {Kind: r.KindSchedule, Framework: "nestjs", Names: []string{"value"}},
	"Process":          {Kind: r.KindJob, Framework: "nestjs-bull", Names: []string{"name", "value"}},
}

// memberEntries are NestJS's routes and the messages, events, schedules and
// queue jobs a member consumes.
func memberEntries(s *r.Source, d r.Decl, class r.ClassContext, e r.Entry) []r.Entry {
	var out []r.Entry
	for _, m := range d.Marks {
		verb, ok := httpVerbs[m.Name]
		if !ok || strings.Contains(m.Full, ".") {
			continue
		}
		x := e
		x.Kind, x.Method, x.Line, x.Framework = r.KindHTTP, verb, m.Line, "nestjs"
		p := m.Arg("value", "path", "uri", "template", "Template")
		prefix := class.Prefix
		if strings.HasPrefix(p, "~/") {
			prefix, p = "", strings.TrimPrefix(p, "~")
		}
		x.Path = r.JoinRoute(prefix, p)
		out = append(out, x)
	}
	for _, m := range d.Marks {
		c, ok := consumers[m.Name]
		if !ok || m.Name == "Process" && class.Processor == "" {
			continue
		}
		x := c.Consume(m, e)
		if m.Name == "Process" {
			x.Path = class.Processor + r.JoinIf(":", x.Path)
		}
		out = append(out, x)
	}
	return out
}

var (
	jsRouteCall = regexp.MustCompile(`\b([A-Za-z_$][\w$]*(?:\.[A-Za-z_$][\w$]*)*)\.(get|post|put|patch|delete|del|all|options|head)\s*\(`)
	jsxRoute    = regexp.MustCompile(`<Route\b[^>]*?\bpath\s*=\s*["'{]\s*["'` + "`" + `]?([^"'` + "`" + `}]*)`)
	objectRoute = regexp.MustCompile(`\bpath\s*:\s*(["'` + "`" + `])([^"'` + "`" + `]*)["'` + "`" + `]`)
	routeTarget = regexp.MustCompile(`\b(component|element|Component|loadComponent|lazy|redirectTo)\s*:\s*`)
	jsClients   = map[string]bool{"axios": true, "http": true, "https": true, "client": true, "api": true, "request": true, "$http": true, "httpClient": true, "HttpClient": true, "superagent": true, "got": true, "ky": true, "instance": true, "fetch": true, "agent": true, "supertest": true, "cy": true, "page": true, "map": true, "params": true, "headers": true, "cache": true, "store": true, "searchParams": true, "query": true, "localStorage": true, "sessionStorage": true, "Reflect": true, "storage": true, "redis": true, "config": true, "settings": true, "process": true, "env": true, "cookies": true, "_": true, "lodash": true}
)

// serverRoutes are the routes Express, Fastify, Koa, Hono, Restify and hapi
// declare by calling the app or a router.
func serverRoutes(s *r.Source) []r.Entry {
	framework := "node-http"
	for _, f := range []string{"express", "fastify", "koa", "hono", "restify", "hapi"} {
		if strings.Contains(s.Raw, `"`+f) || strings.Contains(s.Raw, `'`+f) {
			framework = f
			break
		}
	}
	var out []r.Entry
	for _, loc := range jsRouteCall.FindAllStringSubmatchIndex(s.Code, -1) {
		receiver := s.Code[loc[2]:loc[3]]
		last := receiver[strings.LastIndexByte(receiver, '.')+1:]
		if jsClients[last] || jsClients[receiver] || strings.HasPrefix(receiver, "this.http") {
			continue
		}
		inner, _ := r.Balanced(s.Code, loc[1]-1)
		args := r.SplitArgs(inner)
		if len(args) < 2 {
			continue
		}
		p := r.Unquote(args[0])
		if !strings.HasPrefix(p, "/") && p != "*" {
			continue
		}
		handler := args[len(args)-1]
		if strings.HasPrefix(handler, "{") || r.Unquote(handler) != "" || r.IsEmptyString(handler) {
			continue
		}
		e := r.Entry{Kind: r.KindHTTP, Method: strings.ToUpper(strings.TrimSuffix(s.Code[loc[4]:loc[5]], "del")), Path: p, Framework: framework, Line: s.Line(loc[0])}
		if e.Method == "" {
			e.Method = "DELETE"
		}
		if e.Method == "ALL" {
			e.Method = "ANY"
		}
		if !strings.Contains(handler, "=>") && !strings.HasPrefix(handler, "function") && !strings.HasPrefix(handler, "async") {
			e.Handler = r.LastName(handler)
		}
		for _, mw := range args[1 : len(args)-1] {
			if name := r.LastName(mw); name != "" && r.Authish.MatchString(name) {
				e.Auth = strings.TrimPrefix(e.Auth+", "+name, ", ")
			}
		}
		out = append(out, e)
	}
	return out
}

// pageRoutes are the routes a client-side router declares: React Router's
// <Route path> and route objects, Vue Router's and Angular's route arrays.
func pageRoutes(s *r.Source) []r.Entry {
	framework := ""
	switch {
	case strings.Contains(s.Raw, "react-router"):
		framework = "react-router"
	case strings.Contains(s.Raw, "vue-router"):
		framework = "vue-router"
	case strings.Contains(s.Raw, "@angular/router"):
		framework = "angular-router"
	case strings.Contains(s.Raw, "@tanstack/react-router") || strings.Contains(s.Raw, "@tanstack/router"):
		framework = "tanstack-router"
	default:
		return nil
	}
	var out []r.Entry
	for _, loc := range jsxRoute.FindAllStringSubmatchIndex(s.Code, -1) {
		end := strings.Index(s.Code[loc[1]:], ">")
		if end < 0 {
			end = len(s.Code) - loc[1]
		}
		tag := s.Code[loc[0] : loc[1]+end]
		e := r.Entry{Kind: r.KindPage, Path: s.Code[loc[2]:loc[3]], Framework: framework, Line: s.Line(loc[0])}
		if m := regexp.MustCompile(`(?:element\s*=\s*\{\s*<\s*([A-Za-z_$][\w$.]*)|component\s*=\s*\{\s*([A-Za-z_$][\w$.]*))`).FindStringSubmatch(tag); m != nil {
			e.Handler = r.LastName(m[1] + m[2])
		}
		out = append(out, e)
	}
	for _, loc := range objectRoute.FindAllStringSubmatchIndex(s.Code, -1) {
		// The route's component is in the same object: up to the next path.
		window := s.Code[loc[1]:min(len(s.Code), loc[1]+400)]
		if next := objectRoute.FindStringIndex(window); next != nil {
			window = window[:next[0]]
		}
		e := r.Entry{Kind: r.KindPage, Path: s.Code[loc[4]:loc[5]], Framework: framework, Line: s.Line(loc[0])}
		if t := routeTarget.FindStringIndex(window); t != nil {
			e.Handler = routeComponent(window[t[1]:])
		} else if !strings.Contains(window, "children") {
			continue
		}
		out = append(out, e)
	}
	return out
}

// routeComponent is the component a route object names: `OrdersPage`,
// `<OrdersPage />`, or the file a lazy import loads.
func routeComponent(v string) string {
	v = strings.TrimSpace(v)
	if m := regexp.MustCompile(`import\(\s*["'` + "`" + `]([^"'` + "`" + `]+)`).FindStringSubmatch(v); m != nil {
		base := path.Base(m[1])
		return strings.TrimSuffix(base, path.Ext(base))
	}
	if strings.HasPrefix(v, "<") {
		v = strings.TrimLeft(v, "< ")
	}
	end := strings.IndexFunc(v, func(c rune) bool {
		return !(c == '_' || c == '$' || c == '.' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9')
	})
	if end >= 0 {
		v = v[:end]
	}
	return r.LastName(v)
}

var jsMain = regexp.MustCompile(`require\.main\s*===?\s*module`)

func mainEntries(s *r.Source) []r.Entry {
	if loc := jsMain.FindStringIndex(s.Code); loc != nil {
		return []r.Entry{r.MainEntry(s, loc[0], "")}
	}
	return nil
}

var (
	nextApp    = regexp.MustCompile(`(?:^|/)app/((?:[^/]+/)*)(page|route)\.(?:tsx?|jsx?|mdx)$`)
	nextPages  = regexp.MustCompile(`(?:^|/)pages/(.+)\.(?:tsx?|jsx?|vue)$`)
	nuxtServer = regexp.MustCompile(`(?:^|/)server/(api|routes)/(.+?)(?:\.(get|post|put|patch|delete))?\.(?:ts|js)$`)
	svelteKit  = regexp.MustCompile(`(?:^|/)routes/((?:[^/]+/)*)\+(page|server)\.(?:svelte|ts|js)$`)
)

// fileRoutes are routes a framework reads from where a file is: Next.js's
// app and pages directories, Nuxt's pages and server routes, SvelteKit's
// routes. They are candidates until the framework's config is found.
func fileRoutes(p string) []r.Entry {
	clean := func(segments string) string {
		var keep []string
		for _, seg := range strings.Split(strings.Trim(segments, "/"), "/") {
			if seg == "" || strings.HasPrefix(seg, "(") && strings.HasSuffix(seg, ")") || strings.HasPrefix(seg, "@") {
				continue
			}
			keep = append(keep, seg)
		}
		return r.JoinRoute("", strings.Join(keep, "/"))
	}
	if m := nextApp.FindStringSubmatch(p); m != nil {
		if m[2] == "route" {
			return []r.Entry{{Kind: r.KindHTTP, Method: "ANY", Path: clean(m[1]), Framework: "next", Candidates: []string{"next"}, File: p, Line: 1, DeclLine: 1}}
		}
		return []r.Entry{{Kind: r.KindPage, Path: clean(m[1]), Framework: "next", Candidates: []string{"next"}, File: p, Line: 1, DeclLine: 1}}
	}
	if m := svelteKit.FindStringSubmatch(p); m != nil {
		kind := r.KindPage
		if m[2] == "server" {
			kind = r.KindHTTP
		}
		return []r.Entry{{Kind: kind, Method: map[string]string{r.KindHTTP: "ANY"}[kind], Path: clean(m[1]), Framework: "sveltekit", Candidates: []string{"sveltekit"}, File: p, Line: 1, DeclLine: 1}}
	}
	if m := nuxtServer.FindStringSubmatch(p); m != nil {
		method := strings.ToUpper(m[3])
		if method == "" {
			method = "ANY"
		}
		route := strings.TrimSuffix(m[2], "/index")
		if m[1] == "api" {
			route = "api/" + route
		}
		return []r.Entry{{Kind: r.KindHTTP, Method: method, Path: clean(route), Framework: "nuxt", Candidates: []string{"nuxt"}, File: p, Line: 1, DeclLine: 1}}
	}
	if m := nextPages.FindStringSubmatch(p); m != nil {
		base := path.Base(m[1])
		if strings.HasPrefix(base, "_") || strings.Contains(m[1], "/components/") || strings.HasSuffix(base, ".test") || strings.HasSuffix(base, ".spec") {
			return nil
		}
		route := strings.TrimSuffix(strings.TrimSuffix(m[1], "/index"), "index")
		// A pages/ folder is Next.js's or Nuxt's, whichever is configured.
		e := r.Entry{Kind: r.KindPage, Path: clean(route), Candidates: []string{"next", "nuxt"}, File: p, Line: 1, DeclLine: 1}
		if strings.HasPrefix(m[1], "api/") {
			e.Kind, e.Method = r.KindHTTP, "ANY"
		}
		return []r.Entry{e}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Data and wiring.
// ---------------------------------------------------------------------------

var (
	mongooseModel = regexp.MustCompile(`\b(?:mongoose\.)?model\s*(?:<[^>]*>)?\s*\(\s*["']([A-Za-z_]\w*)["']`)
	sequelizeDef  = regexp.MustCompile(`\.define\s*\(\s*["']([A-Za-z_]\w*)["']`)
	jsModelCall   = regexp.MustCompile(`\b([A-Z]\w*)\.(findOne|findAll|find|findById|findByPk|findOneAndUpdate|findByIdAndUpdate|findOneAndDelete|findByIdAndDelete|create|insertMany|updateOne|updateMany|deleteOne|deleteMany|destroy|update|upsert|aggregate|countDocuments|count|bulkCreate|exists)\s*\(`)
	jsWrites      = map[string]bool{"create": true, "insertMany": true, "updateOne": true, "updateMany": true, "deleteOne": true, "deleteMany": true, "destroy": true, "update": true, "upsert": true, "bulkCreate": true, "findOneAndUpdate": true, "findByIdAndUpdate": true, "findOneAndDelete": true, "findByIdAndDelete": true}
	angularProv   = regexp.MustCompile(`\bprovide\s*:\s*([A-Z]\w*)\s*,\s*use(?:Class|Existing)\s*:\s*([A-Z]\w*)`)
)

// entities are TypeORM's and Mongoose's decorated classes and the models
// Mongoose and Sequelize define by calling.
func entities(s *r.Source) []r.Entity {
	var out []r.Entity
	for _, d := range s.Decls() {
		if !d.Class {
			continue
		}
		e := r.Entity{Class: d.Name, Name: d.Name, File: s.Path, Line: d.Line}
		switch {
		case d.Has("Entity") != nil:
			e.Store, e.Framework = "sql", "typeorm"
			e.Table = d.Has("Entity").Arg("name")
		case d.Has("Schema") != nil && strings.Contains(s.Raw, "@nestjs/mongoose"):
			e.Store, e.Framework = "document", "mongoose"
		default:
			continue
		}
		if t := d.Has("Table"); t != nil && e.Table == "" {
			e.Table = t.Arg("name", "value", "Name")
		}
		out = append(out, e)
	}
	for _, m := range mongooseModel.FindAllStringSubmatchIndex(s.Code, -1) {
		if strings.Contains(s.Raw, "mongoose") {
			out = append(out, r.Entity{Name: s.Code[m[2]:m[3]], Store: "document", Framework: "mongoose", File: s.Path, Line: s.Line(m[0])})
		}
	}
	for _, m := range sequelizeDef.FindAllStringSubmatchIndex(s.Code, -1) {
		if strings.Contains(s.Raw, "sequelize") {
			out = append(out, r.Entity{Name: s.Code[m[2]:m[3]], Table: s.Code[m[2]:m[3]], Store: "sql", Framework: "sequelize", File: s.Path, Line: s.Line(m[0])})
		}
	}
	return out
}

func accesses(s *r.Source) []r.Access {
	var out []r.Access
	for _, m := range jsModelCall.FindAllStringSubmatchIndex(s.Code, -1) {
		kind := r.AccessRead
		if jsWrites[s.Code[m[4]:m[5]]] {
			kind = r.AccessWrite
		}
		// Only a name that turns out to be an entity is kept; Promise.all
		// and Object.create are not tables.
		out = append(out, r.Access{Target: s.Code[m[2]:m[3]], Access: kind, Via: "orm?", File: s.Path, Line: s.Line(m[0])})
	}
	return out
}

// ---------------------------------------------------------------------------
// Symbols and outbound calls.
// ---------------------------------------------------------------------------

var jsUseAuth = regexp.MustCompile(`\b(?:app|router|server)\s*\.\s*use\s*\(\s*(["'][^"']+["'])\s*,\s*([A-Za-z_$][\w$.]*)`)

// security reads app.use('/admin', requireAuth).
func security(s *r.Source, sy *r.Symbols) {
	code := s.Code
	if !strings.Contains(code, ".use(") {
		return
	}
	for _, m := range jsUseAuth.FindAllStringSubmatchIndex(code, -1) {
		name := code[m[4]:m[5]]
		if !r.Authish.MatchString(name) {
			continue
		}
		sy.AuthRules = append(sy.AuthRules, r.AuthRule{Pattern: strings.TrimRight(r.Unquote(code[m[2]:m[3]]), "/") + "/**", Rule: r.LastName(name), Line: s.Line(m[0])})
	}
}

// clientBases are axios instances created with a base, and axios's default
// base.
func clientBases(s *r.Source, sy *r.Symbols) []r.ClientBase {
	code := s.Code
	if !strings.Contains(code, "axios") {
		return nil
	}
	var out []r.ClientBase
	add := func(name, expr string, at int) {
		if name == "" || strings.TrimSpace(expr) == "" {
			return
		}
		out = append(out, r.ClientBase{Name: name, Addr: s.RenderAddr(expr, sy), Resilience: r.ResilienceIn(s.Statement(at)), Line: s.Line(at)})
	}
	for _, m := range r.Rx(`\b([A-Za-z_$][\w$]*)\s*(?::\s*[\w<>.]+\s*)?=\s*axios\s*\.\s*create\s*\(\s*\{`).FindAllStringSubmatchIndex(code, -1) {
		body, _ := r.Balanced(code, m[1]-1)
		add(code[m[2]:m[3]], r.ObjectKey(body, "baseURL"), m[0])
	}
	if m := r.Rx(`axios\s*\.\s*defaults\s*\.\s*baseURL\s*=\s*([^;\n]+)`).FindStringSubmatchIndex(code); m != nil {
		add("axios", code[m[2]:m[3]], m[0])
	}
	return out
}

var (
	jsFetch       = regexp.MustCompile(`(?:^|[^\w$.])(fetch|\$fetch|useFetch|useLazyFetch|ofetch)\s*(?:<[^<>()]*>)?\s*\(`)
	jsAxiosConfig = regexp.MustCompile(`\baxios\s*\(\s*\{`)
)

// calls are fetch and its wrappers, and axios called with a config.
func calls(s *r.Source, sy *r.Symbols, add func(r.OutCall, int)) {
	code := s.Code
	for _, m := range jsFetch.FindAllStringSubmatchIndex(code, -1) {
		open := m[1] - 1
		inner, _ := r.Balanced(code, open)
		args := r.SplitArgs(inner)
		if len(args) == 0 {
			continue
		}
		method := "GET"
		if len(args) > 1 {
			if mm := r.Rx(`\bmethod\s*:\s*['"` + "`" + `](\w+)`).FindStringSubmatch(args[1]); mm != nil {
				method = strings.ToUpper(mm[1])
			}
		}
		rendered := s.RenderAddr(args[0], sy)
		if !r.LooksLikeAddr(rendered, args[0]) {
			continue
		}
		add(r.OutCall{Kind: "http", Client: strings.TrimPrefix(code[m[2]:m[3]], "$"), Method: method, Addr: rendered}, m[2])
	}
	for _, loc := range jsAxiosConfig.FindAllStringIndex(code, -1) {
		body, _ := r.Balanced(code, loc[1]-1)
		addr := r.ObjectKey(body, "url")
		if addr == "" {
			continue
		}
		method := "GET"
		if mm := r.MethodLiteral(r.ObjectKey(body, "method")); mm != "" {
			method = mm
		}
		add(r.OutCall{Kind: "http", Client: "axios", Method: method, Addr: s.RenderAddr(addr, sy)}, loc[0])
	}
}

var callRules = []r.CallRule{
	{Members: regexp.MustCompile(`^(?:send|sendBatch)$`), Tail: regexp.MustCompile(`^\(\s*\{`), Kind: "message", Client: "kafka", Arg: -1, Key: "topic", Gate: `(?i)produc|kafka`},
	{Members: regexp.MustCompile(`^(?:publish)$`), Kind: "message", Client: "amqp", Arg: 0, Gate: `(?i)^(ch|channel|amqp\w*|rabbit\w*|confirmChannel)$`},
	{Members: regexp.MustCompile(`^(?:emit|send)$`), Kind: "message", Client: "nestjs-microservices", Arg: 0, Gate: `(?i)ClientProxy|^(client|\w+Client|\w+Service)$`},
}

var outRules = []r.OutRule{
	// Messaging.
	{Re: regexp.MustCompile(`\b([\w$.]+)\s*\.\s*(sendToQueue)\s*\(`), Kind: "message", Client: "amqp", Arg: 0},
	{Re: regexp.MustCompile(`\bnew\s+SendMessage(?:Batch)?Command\s*\(\s*\{`), Kind: "message", Client: "sqs", Arg: -1, Key: "QueueUrl"},
	{Re: regexp.MustCompile(`\bnew\s+PublishCommand\s*\(\s*\{`), Kind: "message", Client: "sns", Arg: -1, Key: "TopicArn"},
	{Re: regexp.MustCompile(`\b([\w$.]*[Qq]ueue)\s*\.\s*(add)\s*\(`), Kind: "message", Client: "bull", Arg: 0},

	// Databases, caches, search, mail, storage, file transfer.
	{Re: regexp.MustCompile(`\bnew\s+(?:Pool|pg\.Pool|pg\.Client)\s*\(\s*\{`), Kind: "sql", Client: "pg", Arg: -1, Key: "connectionString"},
	{Re: regexp.MustCompile(`\bmysql2?\s*\.\s*create(?:Connection|Pool)\s*\(\s*\{`), Kind: "sql", Client: "mysql", Arg: -1, Key: "host"},
	{Re: regexp.MustCompile(`\b(?:mongoose\s*\.\s*connect|mongoose\s*\.\s*createConnection|new\s+MongoClient|MongoClient\s*\.\s*connect)\s*\(`), Kind: "nosql", Client: "mongodb", Arg: 0},
	{Re: regexp.MustCompile(`\b(?:createClient|redis\s*\.\s*createClient)\s*\(\s*\{`), Kind: "cache", Client: "redis", Arg: -1, Key: "url"},
	{Re: regexp.MustCompile(`\bnew\s+(?:Redis|IORedis)\s*\(`), Kind: "cache", Client: "ioredis", Arg: 0},
	{Re: regexp.MustCompile(`\bnew\s+(?:elasticsearch\s*\.\s*)?Client\s*\(\s*\{\s*node`), Kind: "search", Client: "elasticsearch", Arg: -1, Key: "node"},
	{Re: regexp.MustCompile(`\bnodemailer\s*\.\s*createTransport\s*\(\s*\{`), Kind: "mail", Client: "nodemailer", Arg: -1, Key: "host"},
	{Re: regexp.MustCompile(`\b(?:S3Client\s*\.\s*(?:builder|create)|new\s+S3Client|AmazonS3ClientBuilder|new\s+AmazonS3Client|s3\s*\.\s*New(?:FromConfig)?|new\s+BlobServiceClient|new\s+BlobContainerClient|BlobServiceClient\s*\.\s*fromConnectionString|StorageOptions\s*\.\s*getDefaultService|storage\s*\.\s*NewClient|new\s+Storage)\s*\(`), Kind: "storage", Client: "object-storage", Arg: 0},
	{Re: regexp.MustCompile(`\b([\w$.]*(?:sftp|ftp)\w*)\s*\.\s*(connect|access)\s*\(\s*\{`), Kind: "file_transfer", Client: "sftp", Arg: -1, Key: "host"},

	// GraphQL.
	{Re: regexp.MustCompile(`\bnew\s+ApolloClient\s*\(\s*\{|\bnew\s+HttpLink\s*\(\s*\{|\bcreateHttpLink\s*\(\s*\{`), Kind: "graphql", Client: "apollo", Arg: -1, Key: "uri"},
	{Re: regexp.MustCompile(`\bnew\s+GraphQLClient\s*\(`), Kind: "graphql", Client: "graphql-request", Arg: 0},
}
