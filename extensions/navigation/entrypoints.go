package navigation

import (
	"path"
	"regexp"
	"sort"
	"strings"
)

// Where the outside world gets into the code: an HTTP route, a page, a message
// or event consumed, a schedule, a command, a program's main. Architects think
// in use cases, and a use case starts at one of these. Every one is read from
// how a framework spells it; none is inferred from a name.
const (
	kindHTTP     = "http"
	kindPage     = "page"
	kindMessage  = "message"
	kindEvent    = "event"
	kindSchedule = "schedule"
	kindCLI      = "cli"
	kindMain     = "main"
	kindJob      = "job"
)

// An entry is one entry point as found in a file, before it is linked to the
// unit and function that handle it.
type entry struct {
	Kind      string
	Method    string
	Path      string
	Framework string
	// Handler is the handler as written where the route names it (a call's
	// argument, a urls.py view); empty for an annotated member, which is its
	// own handler.
	Handler string
	// Member and Class name the annotated declaration: the method and the
	// type it is in.
	Member string
	Class  string
	File   string
	Line   int
	// DeclLine is where the handler's declaration starts, for finding the
	// measured function that contains it.
	DeclLine int
}

var httpVerbs = map[string]string{
	"GetMapping": "GET", "PostMapping": "POST", "PutMapping": "PUT", "DeleteMapping": "DELETE", "PatchMapping": "PATCH",
	"HttpGet": "GET", "HttpPost": "POST", "HttpPut": "PUT", "HttpDelete": "DELETE", "HttpPatch": "PATCH", "HttpHead": "HEAD", "HttpOptions": "OPTIONS",
	"Get": "GET", "Post": "POST", "Put": "PUT", "Delete": "DELETE", "Patch": "PATCH", "Head": "HEAD", "Options": "OPTIONS", "All": "ANY",
}

var jaxrsVerbs = map[string]bool{"GET": true, "POST": true, "PUT": true, "DELETE": true, "PATCH": true, "HEAD": true, "OPTIONS": true}

// What a class-level mark contributes to its members: the path prefix, and
// whether its members are a client's (Feign, Retrofit) rather than routes.
type classContext struct {
	Name   string
	Line   int
	Prefix string
	Client bool
	// Framework is what the class-level mark says the members are.
	Framework string
	// Processor is a Nest Bull queue the class consumes.
	Processor string
}

func (s *source) entries() []entry {
	var out []entry
	decls := s.decls()
	classes := s.classContexts(decls)
	for _, d := range decls {
		if d.Class {
			out = append(out, s.classEntries(d)...)
			continue
		}
		out = append(out, s.memberEntries(d, enclosing(classes, d.Line))...)
	}
	out = append(out, s.callEntries()...)
	out = append(out, s.mainEntries()...)
	for i := range out {
		out[i].File = s.path
	}
	return out
}

// classContexts are the types in a file with what their marks say about the
// members below them, in order of declaration.
func (s *source) classContexts(decls []decl) []classContext {
	var out []classContext
	marked := map[int]decl{}
	for _, d := range decls {
		if d.Class {
			marked[d.Line] = d
		}
	}
	for _, t := range s.typeDecls() {
		c := classContext{Name: t.Name, Line: t.Line}
		if d, ok := marked[t.Line]; ok {
			for _, m := range d.Marks {
				switch m.Name {
				case "RequestMapping", "Path", "Controller", "Route", "RoutePrefix":
					c.Prefix = m.arg("value", "path", "prefix", "Template", "template")
					if m.Name == "Controller" && c.Framework == "" {
						c.Framework = "controller"
					}
				case "FeignClient", "RegisterRestClient", "HttpExchange", "RestClient":
					c.Client = true
				case "Processor":
					c.Processor = m.arg("name", "value")
				}
			}
		}
		out = append(out, c)
	}
	return out
}

func enclosing(classes []classContext, line int) classContext {
	var found classContext
	for _, c := range classes {
		if c.Line <= line {
			found = c
		}
	}
	return found
}

// classEntries are the entry points a type is by its marks alone: a
// message handler class, a command, a Swift app's @main.
func (s *source) classEntries(d decl) []entry {
	e := entry{Class: d.Name, Line: d.Line, DeclLine: d.Line}
	switch {
	case d.has("main") != nil && s.lang == langSwift:
		e.Kind, e.Framework = kindMain, "swift"
	case d.has("AsMessageHandler") != nil:
		e.Kind, e.Framework = kindMessage, "symfony-messenger"
	case d.has("AsCommand") != nil:
		e.Kind, e.Framework, e.Path = kindCLI, "symfony-console", d.has("AsCommand").arg("name")
	case d.has("AsEventListener") != nil:
		e.Kind, e.Framework, e.Path = kindEvent, "symfony", d.has("AsEventListener").arg("event")
	case d.has("Command") != nil && (s.lang == langJava || s.lang == langKotlin) && d.has("Command").arg("name") != "":
		e.Kind, e.Framework, e.Path = kindCLI, "picocli", d.has("Command").arg("name")
	case d.has("ShellComponent") != nil:
		return nil
	case d.has("WebServlet") != nil:
		e.Kind, e.Framework, e.Path = kindHTTP, "servlet", d.has("WebServlet").arg("value", "urlPatterns")
	case d.has("ServerEndpoint") != nil:
		e.Kind, e.Framework, e.Path = kindMessage, "websocket", d.has("ServerEndpoint").arg("value")
	default:
		return nil
	}
	return []entry{e}
}

// memberEntries are the entry points an annotated function or method is.
func (s *source) memberEntries(d decl, class classContext) []entry {
	if class.Client {
		return nil
	}
	e := entry{Member: d.Name, Class: class.Name, Line: d.Marks[0].Line, DeclLine: d.Line}
	for _, m := range d.Marks {
		if m.Line < e.Line {
			e.Line = m.Line
		}
	}
	var out []entry
	add := func(x entry) { out = append(out, x) }

	// HTTP: Spring, JAX-RS, Micronaut, NestJS, ASP.NET, Symfony.
	if m := d.has("RequestMapping"); m != nil {
		x := e
		x.Kind, x.Framework = kindHTTP, "spring"
		x.Method = requestMethod(m.Args)
		x.Path = joinRoute(class.Prefix, m.arg("value", "path"))
		x.Line = m.Line
		add(x)
	}
	for _, m := range d.Marks {
		verb, ok := httpVerbs[m.Name]
		if !ok || strings.Contains(m.Full, ".") && s.lang != langCSharp {
			continue
		}
		x := e
		x.Kind, x.Method, x.Line = kindHTTP, verb, m.Line
		switch {
		case strings.HasSuffix(m.Name, "Mapping"):
			x.Framework = "spring"
		case strings.HasPrefix(m.Name, "Http"):
			x.Framework = "aspnet"
		case s.lang == langJS:
			x.Framework = "nestjs"
		default:
			x.Framework = "micronaut"
		}
		p := m.arg("value", "path", "uri", "template", "Template")
		if r := d.has("Route"); r != nil && s.lang == langCSharp && p == "" {
			p = r.arg("template", "Template")
		}
		x.Path = s.routeOf(class, p, d.Name)
		add(x)
	}
	if s.lang == langCSharp && len(out) == 0 {
		if r := d.has("Route"); r != nil {
			x := e
			x.Kind, x.Method, x.Framework, x.Line = kindHTTP, "ANY", "aspnet", r.Line
			x.Path = s.routeOf(class, r.arg("template", "Template"), d.Name)
			add(x)
		}
	}
	for _, m := range d.Marks {
		if !jaxrsVerbs[m.Name] {
			continue
		}
		if len(m.Args) > 0 {
			// Retrofit's @GET("users/{id}") is a call the code makes, not a
			// route it serves.
			return nil
		}
		x := e
		x.Kind, x.Method, x.Framework, x.Line = kindHTTP, m.Name, "jax-rs", m.Line
		p := ""
		if pm := d.has("Path"); pm != nil {
			p = pm.arg("value")
		}
		x.Path = joinRoute(class.Prefix, p)
		add(x)
	}
	if m := d.has("Route"); m != nil && s.lang == langPHP {
		x := e
		x.Kind, x.Framework, x.Line = kindHTTP, "symfony", m.Line
		x.Method = strings.ToUpper(named(m.Args, "methods"))
		if x.Method == "" {
			x.Method = "ANY"
		}
		x.Path = joinRoute(class.Prefix, m.arg("path", "name"))
		add(x)
	}

	// Python web decorators: @app.route, @bp.get, @router.post.
	if s.lang == langPython {
		for _, m := range d.Marks {
			if !strings.Contains(m.Full, ".") {
				continue
			}
			verb := strings.ToUpper(m.Name)
			p := firstString(positional(m.Args))
			if (m.Name == "route" || m.Name == "api_route" || m.Name == "websocket" || jaxrsVerbs[verb]) && (p == "" && len(m.Args) == 0 || strings.HasPrefix(p, "/")) {
				x := e
				x.Kind, x.Line, x.Path, x.Framework = kindHTTP, m.Line, p, s.pythonWeb()
				switch {
				case jaxrsVerbs[verb]:
					x.Method = verb
				case m.Name == "websocket":
					x.Method = "WS"
				default:
					x.Method = strings.ToUpper(named(m.Args, "methods"))
					if x.Method == "" {
						x.Method = "GET"
					}
				}
				add(x)
			}
		}
	}

	// Messages, events and schedules.
	type consumer struct {
		kind, framework string
		names           []string
	}
	consumers := map[string]consumer{
		"KafkaListener":              {kindMessage, "spring-kafka", []string{"topics", "topicPattern", "value"}},
		"RabbitListener":             {kindMessage, "spring-amqp", []string{"queues", "value"}},
		"JmsListener":                {kindMessage, "jms", []string{"destination"}},
		"SqsListener":                {kindMessage, "spring-cloud-aws", []string{"value", "queueNames"}},
		"StreamListener":             {kindMessage, "spring-cloud-stream", []string{"value", "target"}},
		"ServiceActivator":           {kindMessage, "spring-integration", []string{"inputChannel"}},
		"MessageMapping":             {kindMessage, "spring-messaging", []string{"value"}},
		"Incoming":                   {kindMessage, "smallrye", []string{"value"}},
		"EventPattern":               {kindMessage, "nestjs", []string{"value"}},
		"MessagePattern":             {kindMessage, "nestjs", []string{"value"}},
		"SubscribeMessage":           {kindMessage, "nestjs-websockets", []string{"value"}},
		"AsMessageHandler":           {kindMessage, "symfony-messenger", nil},
		"EventListener":              {kindEvent, "spring", []string{"classes", "value", "condition"}},
		"TransactionalEventListener": {kindEvent, "spring", []string{"classes", "value"}},
		"OnEvent":                    {kindEvent, "nestjs", []string{"value"}},
		"AsEventListener":            {kindEvent, "symfony", []string{"event"}},
		"Scheduled":                  {kindSchedule, "spring", []string{"cron", "fixedRate", "fixedDelay", "every"}},
		"Cron":                       {kindSchedule, "nestjs", []string{"value"}},
		"Interval":                   {kindSchedule, "nestjs", []string{"value"}},
		"AsCronTask":                 {kindSchedule, "symfony-scheduler", []string{"expression"}},
		"AsPeriodicTask":             {kindSchedule, "symfony-scheduler", []string{"frequency"}},
		"ShellMethod":                {kindCLI, "spring-shell", []string{"key", "value"}},
		"Process":                    {kindJob, "nestjs-bull", []string{"name", "value"}},
	}
	for _, m := range d.Marks {
		c, ok := consumers[m.Name]
		if !ok || (s.lang == langPython && m.Name != "receiver") {
			continue
		}
		if m.Name == "Process" && class.Processor == "" {
			continue
		}
		x := e
		x.Kind, x.Framework, x.Line = c.kind, c.framework, m.Line
		if len(c.names) > 0 {
			x.Path = m.arg(c.names...)
			if x.Path == "" && len(m.Args) > 0 {
				x.Path = strings.TrimSpace(m.Args[0])
			}
		}
		if m.Name == "Process" {
			x.Path = class.Processor + joinIf(":", x.Path)
		}
		add(x)
	}
	if s.lang == langPython {
		for _, m := range d.Marks {
			x := e
			x.Line = m.Line
			switch {
			case m.Name == "receiver":
				x.Kind, x.Framework = kindEvent, "django-signals"
				if p := positional(m.Args); len(p) > 0 {
					x.Path = p[0]
				}
			case m.Name == "shared_task" || m.Name == "task" && strings.Contains(m.Full, "."), m.Name == "actor" && strings.Contains(m.Full, "dramatiq"):
				x.Kind, x.Framework, x.Path = kindJob, "celery", m.arg("name")
			case m.Name == "periodic_task":
				x.Kind, x.Framework = kindSchedule, "celery"
			case m.Name == "scheduled_job":
				x.Kind, x.Framework = kindSchedule, "apscheduler"
				x.Path = strings.Join(positional(m.Args), " ")
			case m.Name == "command" || m.Name == "group" && strings.Contains(m.Full, "click"):
				x.Kind, x.Framework = kindCLI, "click"
				x.Path = m.arg("name")
				if x.Path == "" {
					x.Path = strings.ReplaceAll(d.Name, "_", "-")
				}
			default:
				continue
			}
			add(x)
		}
	}
	// Azure Functions: the trigger is an attribute on a parameter.
	if m := d.has("Function", "FunctionName"); m != nil && s.lang == langCSharp {
		x := e
		x.Line, x.Framework, x.Path = m.Line, "azure-functions", m.arg("name", "Name")
		x.Kind = kindJob
		sig := s.code[s.lines[d.Line-1]:min(len(s.code), s.lineEnd(min(len(s.lines), d.Line+6)))]
		switch {
		case strings.Contains(sig, "HttpTrigger"):
			x.Kind, x.Method = kindHTTP, "ANY"
			if r := regexp.MustCompile(`Route\s*=\s*"([^"]*)"`).FindStringSubmatch(sig); r != nil {
				x.Path = joinRoute("/api", r[1])
			}
		case strings.Contains(sig, "TimerTrigger"):
			x.Kind = kindSchedule
		case regexp.MustCompile(`(ServiceBus|Queue|EventHub|Kafka|RabbitMQ|EventGrid|CosmosDB|Blob)Trigger`).MatchString(sig):
			x.Kind = kindMessage
		}
		add(x)
	}
	return out
}

// routeOf joins a member's route to its class's, with ASP.NET's [controller]
// and [action] tokens filled and its ~/ meaning "from the root".
func (s *source) routeOf(class classContext, member, action string) string {
	prefix := class.Prefix
	if strings.HasPrefix(member, "~/") || strings.HasPrefix(member, "/") && s.lang == langCSharp {
		prefix, member = "", strings.TrimPrefix(member, "~")
	}
	r := joinRoute(prefix, member)
	if s.lang == langCSharp {
		ctrl := strings.TrimSuffix(class.Name, "Controller")
		r = strings.NewReplacer("[controller]", ctrl, "[action]", action).Replace(r)
	}
	return r
}

func joinIf(sep, s string) string {
	if s == "" {
		return ""
	}
	return sep + s
}

// joinRoute joins a prefix and a path into one route, each with one leading
// slash and none doubled; an empty path is the prefix itself.
func joinRoute(prefix, p string) string {
	prefix, p = strings.TrimSpace(prefix), strings.TrimSpace(p)
	if prefix == "" && p == "" {
		return "/"
	}
	joined := strings.TrimRight(prefix, "/") + "/" + strings.TrimLeft(p, "/")
	if !strings.HasPrefix(joined, "/") {
		joined = "/" + joined
	}
	for strings.Contains(joined, "//") {
		joined = strings.ReplaceAll(joined, "//", "/")
	}
	if len(joined) > 1 {
		joined = strings.TrimRight(joined, "/")
	}
	return joined
}

// requestMethod is the method a @RequestMapping restricts itself to, or ANY.
func requestMethod(args []string) string {
	m := named(args, "method")
	if m == "" {
		return "ANY"
	}
	var verbs []string
	for _, v := range regexp.MustCompile(`(GET|POST|PUT|DELETE|PATCH|HEAD|OPTIONS)`).FindAllString(strings.Join(args, ","), -1) {
		if !contains(verbs, v) {
			verbs = append(verbs, v)
		}
	}
	if len(verbs) == 0 {
		return "ANY"
	}
	return strings.Join(verbs, ",")
}

func contains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

func (s *source) pythonWeb() string {
	switch {
	case strings.Contains(s.raw, "fastapi"):
		return "fastapi"
	case strings.Contains(s.raw, "flask"):
		return "flask"
	case strings.Contains(s.raw, "sanic"):
		return "sanic"
	}
	return "python-web"
}

var (
	jsRouteCall  = regexp.MustCompile(`\b([A-Za-z_$][\w$]*(?:\.[A-Za-z_$][\w$]*)*)\.(get|post|put|patch|delete|del|all|options|head)\s*\(`)
	goRouteCall  = regexp.MustCompile(`\b([A-Za-z_]\w*)\.(GET|POST|PUT|PATCH|DELETE|HEAD|OPTIONS|Any|Handle|HandleFunc|Get|Post|Put|Patch|Delete|Head|Options|Match|Method|MethodFunc|Static)\s*\(`)
	goGroup      = regexp.MustCompile(`\b([A-Za-z_]\w*)\s*:?=\s*([A-Za-z_]\w*)\.(Group|PathPrefix|Prefix|Party)\s*\(\s*"([^"]*)"`)
	pyURL        = regexp.MustCompile(`\b(path|re_path|url)\s*\(`)
	laravelRoute = regexp.MustCompile(`Route::(get|post|put|patch|delete|options|any|match|resource|apiResource)\s*\(`)
	ktorRoute    = regexp.MustCompile(`\b(get|post|put|patch|delete|head|options)\s*\(\s*"([^"]*)"\s*\)\s*\{`)
	minimalAPI   = regexp.MustCompile(`\.(MapGet|MapPost|MapPut|MapDelete|MapPatch|MapMethods|MapHub|Map)\s*(?:<[^>]*>)?\s*\(`)
	cobraCommand = regexp.MustCompile(`cobra\.Command\s*\{`)
	jsxRoute     = regexp.MustCompile(`<Route\b[^>]*?\bpath\s*=\s*["'{]\s*["'` + "`" + `]?([^"'` + "`" + `}]*)`)
	objectRoute  = regexp.MustCompile(`\bpath\s*:\s*(["'` + "`" + `])([^"'` + "`" + `]*)["'` + "`" + `]`)
	routeTarget  = regexp.MustCompile(`\b(component|element|Component|loadComponent|lazy|redirectTo)\s*:\s*`)
	jsClients    = map[string]bool{"axios": true, "http": true, "https": true, "client": true, "api": true, "request": true, "$http": true, "httpClient": true, "HttpClient": true, "superagent": true, "got": true, "ky": true, "instance": true, "fetch": true, "agent": true, "supertest": true, "cy": true, "page": true, "map": true, "params": true, "headers": true, "cache": true, "store": true, "searchParams": true, "query": true, "localStorage": true, "sessionStorage": true, "Reflect": true, "storage": true, "redis": true, "config": true, "settings": true, "process": true, "env": true, "cookies": true, "_": true, "lodash": true}
)

// callEntries are routes declared by calling something: Express, Go's
// routers, Django's urls, Laravel, Ktor, ASP.NET minimal APIs, cobra,
// React, Vue and Angular routers.
func (s *source) callEntries() []entry {
	switch s.lang {
	case langJS:
		return append(s.jsRoutes(), s.pageRoutes()...)
	case langGo:
		return s.goRoutes()
	case langPython:
		if strings.HasSuffix(s.path, "urls.py") || strings.Contains(s.code, "urlpatterns") {
			return s.djangoURLs()
		}
	case langPHP:
		return s.laravelRoutes()
	case langKotlin:
		if strings.Contains(s.code, "routing") {
			return s.ktorRoutes()
		}
	case langCSharp:
		return s.minimalAPIRoutes()
	}
	return nil
}

func (s *source) jsRoutes() []entry {
	framework := "node-http"
	for _, f := range []string{"express", "fastify", "koa", "hono", "restify", "hapi"} {
		if strings.Contains(s.raw, `"`+f) || strings.Contains(s.raw, `'`+f) {
			framework = f
			break
		}
	}
	var out []entry
	for _, loc := range jsRouteCall.FindAllStringSubmatchIndex(s.code, -1) {
		receiver := s.code[loc[2]:loc[3]]
		last := receiver[strings.LastIndexByte(receiver, '.')+1:]
		if jsClients[last] || jsClients[receiver] || strings.HasPrefix(receiver, "this.http") {
			continue
		}
		inner, _ := balanced(s.code, loc[1]-1)
		args := splitArgs(inner)
		if len(args) < 2 {
			continue
		}
		p := unquote(args[0])
		if !strings.HasPrefix(p, "/") && p != "*" {
			continue
		}
		handler := args[len(args)-1]
		if strings.HasPrefix(handler, "{") || unquote(handler) != "" || isEmptyString(handler) {
			continue
		}
		e := entry{Kind: kindHTTP, Method: strings.ToUpper(strings.TrimSuffix(s.code[loc[4]:loc[5]], "del")), Path: p, Framework: framework, Line: s.line(loc[0])}
		if e.Method == "" {
			e.Method = "DELETE"
		}
		if e.Method == "ALL" {
			e.Method = "ANY"
		}
		if !strings.Contains(handler, "=>") && !strings.HasPrefix(handler, "function") && !strings.HasPrefix(handler, "async") {
			e.Handler = lastName(handler)
		}
		out = append(out, e)
	}
	return out
}

// pageRoutes are the routes a client-side router declares: React Router's
// <Route path> and route objects, Vue Router's and Angular's route arrays.
func (s *source) pageRoutes() []entry {
	framework := ""
	switch {
	case strings.Contains(s.raw, "react-router"):
		framework = "react-router"
	case strings.Contains(s.raw, "vue-router"):
		framework = "vue-router"
	case strings.Contains(s.raw, "@angular/router"):
		framework = "angular-router"
	case strings.Contains(s.raw, "@tanstack/react-router") || strings.Contains(s.raw, "@tanstack/router"):
		framework = "tanstack-router"
	default:
		return nil
	}
	var out []entry
	for _, loc := range jsxRoute.FindAllStringSubmatchIndex(s.code, -1) {
		end := strings.Index(s.code[loc[1]:], ">")
		if end < 0 {
			end = len(s.code) - loc[1]
		}
		tag := s.code[loc[0] : loc[1]+end]
		e := entry{Kind: kindPage, Path: s.code[loc[2]:loc[3]], Framework: framework, Line: s.line(loc[0])}
		if m := regexp.MustCompile(`(?:element\s*=\s*\{\s*<\s*([A-Za-z_$][\w$.]*)|component\s*=\s*\{\s*([A-Za-z_$][\w$.]*))`).FindStringSubmatch(tag); m != nil {
			e.Handler = lastName(m[1] + m[2])
		}
		out = append(out, e)
	}
	for _, loc := range objectRoute.FindAllStringSubmatchIndex(s.code, -1) {
		// The route's component is in the same object: up to the next path.
		window := s.code[loc[1]:min(len(s.code), loc[1]+400)]
		if next := objectRoute.FindStringIndex(window); next != nil {
			window = window[:next[0]]
		}
		e := entry{Kind: kindPage, Path: s.code[loc[4]:loc[5]], Framework: framework, Line: s.line(loc[0])}
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
	end := strings.IndexFunc(v, func(r rune) bool {
		return !(r == '_' || r == '$' || r == '.' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9')
	})
	if end >= 0 {
		v = v[:end]
	}
	return lastName(v)
}

func (s *source) goRoutes() []entry {
	framework := "net/http"
	for _, f := range []struct{ imp, name string }{{"gin-gonic/gin", "gin"}, {"labstack/echo", "echo"}, {"go-chi/chi", "chi"}, {"gofiber/fiber", "fiber"}, {"gorilla/mux", "gorilla"}, {"kataras/iris", "iris"}, {"julienschmidt/httprouter", "httprouter"}} {
		if strings.Contains(s.raw, f.imp) {
			framework = f.name
			break
		}
	}
	prefixes := map[string]string{}
	for _, m := range goGroup.FindAllStringSubmatch(s.code, -1) {
		prefixes[m[1]] = joinRoute(prefixes[m[2]], m[4])
	}
	var out []entry
	for _, loc := range goRouteCall.FindAllStringSubmatchIndex(s.code, -1) {
		receiver, verb := s.code[loc[2]:loc[3]], s.code[loc[4]:loc[5]]
		inner, end := balanced(s.code, loc[1]-1)
		args := splitArgs(inner)
		if verb == "Match" || verb == "Method" || verb == "MethodFunc" {
			if len(args) < 3 {
				continue
			}
			args = args[1:]
		}
		if len(args) < 2 {
			continue
		}
		p := unquote(args[0])
		method := strings.ToUpper(verb)
		if verb == "Match" || verb == "Method" || verb == "MethodFunc" {
			method = strings.ToUpper(unquote(strings.Split(inner, ",")[0]))
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
			if m := regexp.MustCompile(`^\s*\.Methods\(\s*"([A-Z]+)"`).FindStringSubmatch(s.code[end:min(len(s.code), end+60)]); m != nil {
				method = m[1]
			}
		case "STATIC":
			continue
		}
		e := entry{Kind: kindHTTP, Method: method, Path: joinRoute(prefixes[receiver], p), Framework: framework, Line: s.line(loc[0])}
		if h := args[len(args)-1]; !strings.HasPrefix(h, "func") {
			e.Handler = lastName(h)
		}
		out = append(out, e)
	}
	for _, loc := range cobraCommand.FindAllStringIndex(s.code, -1) {
		body, _ := balanced(s.code, loc[1]-1)
		if m := regexp.MustCompile(`\bUse:\s*"([^"]+)"`).FindStringSubmatch(body); m != nil {
			e := entry{Kind: kindCLI, Path: strings.Fields(m[1])[0], Framework: "cobra", Line: s.line(loc[0])}
			if r := regexp.MustCompile(`\bRunE?:\s*([A-Za-z_][\w.]*)`).FindStringSubmatch(body); r != nil {
				e.Handler = lastName(r[1])
			}
			out = append(out, e)
		}
	}
	return out
}

var getClassAlias = regexp.MustCompile(`\b([a-z_]\w*)\s*=\s*get_class\(\s*["'][\w.]+["']\s*,\s*["'](\w+)["']`)

func (s *source) djangoURLs() []entry {
	// django-oscar's apps hold their views as attributes loaded by name
	// (`list_view = get_class("dashboard.views", "ListView")`) so a project
	// can override them; the class named is the handler.
	aliases := map[string]string{}
	for _, m := range getClassAlias.FindAllStringSubmatch(s.code, -1) {
		aliases[m[1]] = m[2]
	}
	var out []entry
	for _, loc := range pyURL.FindAllStringSubmatchIndex(s.code, -1) {
		if loc[0] > 0 && (s.code[loc[0]-1] == '.' || isWordByte(s.code[loc[0]-1])) {
			continue
		}
		inner, _ := balanced(s.code, loc[1]-1)
		args := splitArgs(inner)
		if len(args) < 2 {
			continue
		}
		p := unquote(args[0])
		if p == "" && !isEmptyString(args[0]) {
			continue
		}
		view := args[1]
		if strings.HasPrefix(view, "include(") || strings.HasSuffix(view, ".urls") || strings.Contains(view, ".urls[") {
			continue
		}
		p = strings.TrimSuffix(strings.TrimPrefix(p, "^"), "$")
		handler := lastName(view)
		if cls, ok := aliases[handler]; ok {
			handler = cls
		}
		out = append(out, entry{Kind: kindHTTP, Method: "ANY", Path: joinRoute("", p), Framework: "django", Handler: handler, Line: s.line(loc[0])})
	}
	return out
}

func (s *source) laravelRoutes() []entry {
	var out []entry
	for _, loc := range laravelRoute.FindAllStringSubmatchIndex(s.code, -1) {
		verb := s.code[loc[2]:loc[3]]
		inner, _ := balanced(s.code, loc[1]-1)
		args := splitArgs(inner)
		method := strings.ToUpper(verb)
		if verb == "match" && len(args) > 1 {
			method = strings.ToUpper(strings.Join(regexp.MustCompile(`[A-Za-z]+`).FindAllString(args[0], -1), ","))
			args = args[1:]
		}
		if verb == "resource" || verb == "apiResource" || verb == "any" {
			method = "ANY"
		}
		if len(args) < 1 {
			continue
		}
		e := entry{Kind: kindHTTP, Method: method, Path: joinRoute("", unquote(args[0])), Framework: "laravel", Line: s.line(loc[0])}
		if len(args) > 1 {
			e.Handler = lastName(args[1])
		}
		out = append(out, e)
	}
	return out
}

func (s *source) ktorRoutes() []entry {
	var out []entry
	for _, m := range ktorRoute.FindAllStringSubmatchIndex(s.code, -1) {
		out = append(out, entry{Kind: kindHTTP, Method: strings.ToUpper(s.code[m[2]:m[3]]), Path: joinRoute("", s.code[m[4]:m[5]]), Framework: "ktor", Line: s.line(m[0])})
	}
	return out
}

func (s *source) minimalAPIRoutes() []entry {
	var out []entry
	for _, loc := range minimalAPI.FindAllStringSubmatchIndex(s.code, -1) {
		verb := s.code[loc[2]:loc[3]]
		inner, _ := balanced(s.code, loc[1]-1)
		args := splitArgs(inner)
		if len(args) < 2 {
			continue
		}
		p := unquote(args[0])
		if p == "" || !strings.HasPrefix(p, "/") && !strings.HasPrefix(p, "api") {
			continue
		}
		method := strings.ToUpper(strings.TrimPrefix(verb, "Map"))
		kind := kindHTTP
		switch verb {
		case "Map", "MapMethods":
			method = "ANY"
		case "MapHub":
			method, kind = "WS", kindMessage
		}
		e := entry{Kind: kind, Method: method, Path: joinRoute("", p), Framework: "aspnet-minimal", Line: s.line(loc[0])}
		if h := args[len(args)-1]; !strings.Contains(h, "=>") {
			e.Handler = lastName(h)
		}
		out = append(out, e)
	}
	return out
}

var (
	javaMain   = regexp.MustCompile(`(?m)\bpublic\s+static\s+void\s+main\s*\(\s*(?:final\s+)?String`)
	kotlinMain = regexp.MustCompile(`(?m)^\s*fun\s+main\s*\(`)
	csMain     = regexp.MustCompile(`(?m)\bstatic\s+(?:async\s+)?(?:void|int|Task|Task<int>)\s+Main\s*\(`)
	goMain     = regexp.MustCompile(`(?m)^func\s+main\s*\(\s*\)`)
	goPackage  = regexp.MustCompile(`(?m)^package\s+main\b`)
	pyMain     = regexp.MustCompile(`(?m)^if\s+__name__\s*==\s*['"]__main__['"]\s*:`)
	dartMain   = regexp.MustCompile(`(?m)^(?:void|Future<void>|Future)\s+main\s*\(`)
	jsMain     = regexp.MustCompile(`require\.main\s*===?\s*module`)
	csTopLevel = regexp.MustCompile(`(?:WebApplication|Host)\.Create\w*Builder\s*\(`)
)

// mainEntries are the programs: a main function, Python's __main__ guard,
// a C# top-level Program.cs.
func (s *source) mainEntries() []entry {
	var re *regexp.Regexp
	switch s.lang {
	case langJava:
		re = javaMain
	case langKotlin:
		re = kotlinMain
	case langCSharp:
		re = csMain
		if strings.EqualFold(path.Base(s.path), "Program.cs") && !csMain.MatchString(s.code) && csTopLevel.MatchString(s.code) {
			return []entry{{Kind: kindMain, Framework: "dotnet", Path: path.Dir(s.path), Line: 1, DeclLine: 1}}
		}
	case langGo:
		if !goPackage.MatchString(s.code) {
			return nil
		}
		re = goMain
	case langPython:
		re = pyMain
		if m := regexp.MustCompile(`(^|/)management/commands/([a-z_][a-z0-9_]*)\.py$`).FindStringSubmatch(s.path); m != nil && m[2] != "__init__" && strings.Contains(s.code, "class Command") {
			line := strings.Count(s.code[:strings.Index(s.code, "class Command")], "\n") + 1
			return []entry{{Kind: kindCLI, Framework: "django-admin", Path: m[2], Class: "Command", Line: line, DeclLine: line}}
		}
	case langDart:
		re = dartMain
	case langJS:
		re = jsMain
	}
	if re == nil {
		return nil
	}
	loc := re.FindStringIndex(s.code)
	if loc == nil {
		return nil
	}
	line := s.line(loc[0])
	e := entry{Kind: kindMain, Framework: s.lang, Path: path.Dir(s.path), Line: line, DeclLine: line}
	if s.lang == langJava || s.lang == langCSharp {
		e.Member = "main"
		if s.lang == langCSharp {
			e.Member = "Main"
		}
	}
	return []entry{e}
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
func fileRoutes(p string) []entry {
	clean := func(segments string) string {
		var keep []string
		for _, seg := range strings.Split(strings.Trim(segments, "/"), "/") {
			if seg == "" || strings.HasPrefix(seg, "(") && strings.HasSuffix(seg, ")") || strings.HasPrefix(seg, "@") {
				continue
			}
			keep = append(keep, seg)
		}
		return joinRoute("", strings.Join(keep, "/"))
	}
	if m := nextApp.FindStringSubmatch(p); m != nil {
		if m[2] == "route" {
			return []entry{{Kind: kindHTTP, Method: "ANY", Path: clean(m[1]), Framework: "next", File: p, Line: 1, DeclLine: 1}}
		}
		return []entry{{Kind: kindPage, Path: clean(m[1]), Framework: "next", File: p, Line: 1, DeclLine: 1}}
	}
	if m := svelteKit.FindStringSubmatch(p); m != nil {
		kind := kindPage
		if m[2] == "server" {
			kind = kindHTTP
		}
		return []entry{{Kind: kind, Method: map[string]string{kindHTTP: "ANY"}[kind], Path: clean(m[1]), Framework: "sveltekit", File: p, Line: 1, DeclLine: 1}}
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
		return []entry{{Kind: kindHTTP, Method: method, Path: clean(route), Framework: "nuxt", File: p, Line: 1, DeclLine: 1}}
	}
	if m := nextPages.FindStringSubmatch(p); m != nil {
		base := path.Base(m[1])
		if strings.HasPrefix(base, "_") || strings.Contains(m[1], "/components/") || strings.HasSuffix(base, ".test") || strings.HasSuffix(base, ".spec") {
			return nil
		}
		route := strings.TrimSuffix(strings.TrimSuffix(m[1], "/index"), "index")
		e := entry{Kind: kindPage, Path: clean(route), Framework: "pages", File: p, Line: 1, DeclLine: 1}
		if strings.HasPrefix(m[1], "api/") {
			e.Kind, e.Method = kindHTTP, "ANY"
		}
		return []entry{e}
	}
	return nil
}

func sortEntries(list []entryRow) {
	sort.SliceStable(list, func(i, j int) bool {
		a, b := list[i], list[j]
		if a.Kind != b.Kind {
			return a.Kind < b.Kind
		}
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		if a.Method != b.Method {
			return a.Method < b.Method
		}
		if a.File != b.File {
			return a.File < b.File
		}
		return a.Line < b.Line
	})
}
