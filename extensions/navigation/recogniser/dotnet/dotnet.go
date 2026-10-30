// Package dotnet reads C# for navigation: ASP.NET's attribute, conventional
// and minimal-API routes, Azure Functions, Entity Framework, the .NET
// container and Autofac, typed and named HttpClients, Refit, RestSharp, and
// .NET's messaging and data clients.
package dotnet

import (
	"path"
	"regexp"
	"strings"

	r "github.com/archstats/archstats/extensions/navigation/recogniser"
)

const csharp = "csharp"

// Plugin is .NET.
func Plugin() r.Plugin {
	return r.Plugin{
		Name: "dotnet",
		Languages: []*r.Language{{
			Name: csharp, Extensions: []string{".cs"}, Kind: r.Code,
			Syntax: r.Syntax{
				LineComments:       []string{"//"},
				BlockComments:      [][2]string{{"/*", "*/"}},
				Strings:            true,
				TripleQuotes:       true,
				CharLiterals:       true,
				Marks:              r.BracketMarks,
				MarkOpen:           "[",
				MarkSuffix:         "Attribute",
				TypeName:           r.CLikeTypeName,
				BraceInterpolation: true,
			},
		}},
		Recognisers: []*r.Recogniser{{
			Languages:       []string{csharp},
			ClassMark:       classMark,
			MemberEntries:   memberEntries,
			CallEntries:     minimalAPIRoutes,
			MainEntries:     mainEntries,
			Convention:      &r.Convention{Framework: "aspnet-conventional", Completes: "aspnet", Route: conventionalRoute},
			AuthMarks:       []string{"Authorize", "AllowAnonymous"},
			DocMarks:        []string{"SwaggerOperation", "ProducesResponseType", "OpenApiOperation"},
			Entities:        entities,
			Accesses:        accesses,
			Bindings:        bindings,
			Declared:        declared,
			Constants:       r.StaticConstants,
			ConfigKeys:      configKeys,
			ClientBases:     clientBases,
			DeclaredClients: declaredClients,
			Calls:           calls,
			ClientName:      clientName,
			OutRules:        outRules,
			CallRules:       callRules,
		}},
	}
}

// ---------------------------------------------------------------------------
// Entry points.
// ---------------------------------------------------------------------------

func classMark(m r.Mark, c *r.ClassContext) {
	switch m.Name {
	case "Route", "RoutePrefix", "Controller":
		c.Prefix = m.Arg("value", "path", "prefix", "Template", "template")
	}
}

var httpVerbs = map[string]string{
	"HttpGet": "GET", "HttpPost": "POST", "HttpPut": "PUT", "HttpDelete": "DELETE", "HttpPatch": "PATCH", "HttpHead": "HEAD", "HttpOptions": "OPTIONS",
}

// memberEntries are an action's attribute routes and an Azure Function's
// trigger.
func memberEntries(s *r.Source, d r.Decl, class r.ClassContext, e r.Entry) []r.Entry {
	var out []r.Entry
	add := func(x r.Entry) { out = append(out, x) }
	for _, m := range d.Marks {
		// Refit's [Get("/x")] on an interface is a call the code makes.
		verb, ok := httpVerbs[m.Name]
		if !ok {
			continue
		}
		x := e
		x.Kind, x.Method, x.Line = r.KindHTTP, verb, m.Line
		x.Framework = "aspnet"
		p := m.Arg("value", "path", "uri", "template", "Template")
		if rt := d.Has("Route"); rt != nil && p == "" {
			p = rt.Arg("template", "Template")
		}
		x.Path = routeOf(class, p, d.Name)
		add(x)
	}
	if len(out) == 0 {
		if rt := d.Has("Route"); rt != nil {
			x := e
			x.Kind, x.Method, x.Framework, x.Line = r.KindHTTP, "ANY", "aspnet", rt.Line
			x.Path = routeOf(class, rt.Arg("template", "Template"), d.Name)
			add(x)
		}
	}
	// Azure Functions: the trigger is an attribute on a parameter.
	if m := d.Has("Function", "FunctionName"); m != nil {
		x := e
		x.Line, x.Framework, x.Path = m.Line, "azure-functions", m.Arg("name", "Name")
		x.Kind = r.KindJob
		sig := s.Code[s.Lines[d.Line-1]:min(len(s.Code), s.LineEnd(min(len(s.Lines), d.Line+6)))]
		switch {
		case strings.Contains(sig, "HttpTrigger"):
			x.Kind, x.Method = r.KindHTTP, "ANY"
			if rt := regexp.MustCompile(`Route\s*=\s*"([^"]*)"`).FindStringSubmatch(sig); rt != nil {
				x.Path = r.JoinRoute("/api", rt[1])
			}
		case strings.Contains(sig, "TimerTrigger"):
			x.Kind = r.KindSchedule
		case regexp.MustCompile(`(ServiceBus|Queue|EventHub|Kafka|RabbitMQ|EventGrid|CosmosDB|Blob)Trigger`).MatchString(sig):
			x.Kind = r.KindMessage
		}
		add(x)
	}
	return out
}

// routeOf joins a member's route to its class's, with ASP.NET's [controller]
// and [action] tokens filled and its ~/ or / meaning "from the root".
func routeOf(class r.ClassContext, member, action string) string {
	prefix := class.Prefix
	if strings.HasPrefix(member, "~/") || strings.HasPrefix(member, "/") {
		prefix, member = "", strings.TrimPrefix(member, "~")
	}
	rt := r.JoinRoute(prefix, member)
	ctrl := strings.TrimSuffix(class.Name, "Controller")
	return strings.NewReplacer("[controller]", ctrl, "[action]", action).Replace(rt)
}

var minimalAPI = regexp.MustCompile(`\.(MapGet|MapPost|MapPut|MapDelete|MapPatch|MapMethods|MapHub|Map)\s*(?:<[^>]*>)?\s*\(`)

func minimalAPIRoutes(s *r.Source) []r.Entry {
	var out []r.Entry
	for _, loc := range minimalAPI.FindAllStringSubmatchIndex(s.Code, -1) {
		verb := s.Code[loc[2]:loc[3]]
		inner, _ := r.Balanced(s.Code, loc[1]-1)
		args := r.SplitArgs(inner)
		if len(args) < 2 {
			continue
		}
		p := r.Unquote(args[0])
		if p == "" || !strings.HasPrefix(p, "/") && !strings.HasPrefix(p, "api") {
			continue
		}
		method := strings.ToUpper(strings.TrimPrefix(verb, "Map"))
		kind := r.KindHTTP
		switch verb {
		case "Map", "MapMethods":
			method = "ANY"
		case "MapHub":
			method, kind = "WS", r.KindMessage
		}
		e := r.Entry{Kind: kind, Method: method, Path: r.JoinRoute("", p), Framework: "aspnet-minimal", Line: s.Line(loc[0])}
		if h := args[len(args)-1]; !strings.Contains(h, "=>") {
			e.Handler = r.LastName(h)
		}
		_, end := r.Balanced(s.Code, loc[1]-1)
		if a := r.Rx(`^(?:\s*\.\s*\w+\s*(?:<[^<>]*>)?\s*\([^()]*\))*?\s*\.\s*(RequireAuthorization|AllowAnonymous)\s*\(([^()]*)\)`).FindStringSubmatch(s.Code[end:min(len(s.Code), end+300)]); a != nil {
			e.Auth = a[1]
			if strings.TrimSpace(a[2]) != "" {
				e.Auth += "(" + strings.TrimSpace(a[2]) + ")"
			}
		}
		out = append(out, e)
	}
	return out
}

var (
	csMain     = regexp.MustCompile(`(?m)\bstatic\s+(?:async\s+)?(?:void|int|Task|Task<int>)\s+Main\s*\(`)
	csTopLevel = regexp.MustCompile(`(?:WebApplication|Host)\.Create\w*Builder\s*\(`)
)

// mainEntries are a Main method, or a top-level Program.cs.
func mainEntries(s *r.Source) []r.Entry {
	if strings.EqualFold(path.Base(s.Path), "Program.cs") && !csMain.MatchString(s.Code) && csTopLevel.MatchString(s.Code) {
		return []r.Entry{{Kind: r.KindMain, Framework: "dotnet", Path: path.Dir(s.Path), Line: 1, DeclLine: 1}}
	}
	if loc := csMain.FindStringIndex(s.Code); loc != nil {
		return []r.Entry{r.MainEntry(s, loc[0], "Main")}
	}
	return nil
}

var actionResult = regexp.MustCompile(`^public\s+(?:virtual\s+|override\s+|async\s+)*(?:Task\s*<\s*)?(?:I?ActionResult|JsonResult|ViewResult|PartialViewResult|ContentResult|FileResult|RedirectResult|RedirectToActionResult|IResult)\b`)

// conventionalRoute is an ASP.NET MVC action's route by convention: a
// public method returning an action result on a controller is reached at
// /{controller}/{action}.
func conventionalRoute(function, signature string) (string, bool) {
	parts := strings.Split(function, ".")
	if len(parts) < 2 || !strings.HasSuffix(parts[len(parts)-2], "Controller") || !actionResult.MatchString(signature) {
		return "", false
	}
	ctrl, action := strings.TrimSuffix(parts[len(parts)-2], "Controller"), strings.TrimSuffix(parts[len(parts)-1], "Async")
	return "/" + ctrl + "/" + action, true
}

// ---------------------------------------------------------------------------
// Data and wiring.
// ---------------------------------------------------------------------------

var (
	efDbSet      = regexp.MustCompile(`\bDbSet\s*<\s*([A-Z]\w*)\s*>\s+([A-Z]\w*)`)
	efSet        = regexp.MustCompile(`\.Set\s*<\s*([A-Z]\w*)\s*>\s*\(\s*\)`)
	dotnetAdd    = regexp.MustCompile(`\.(?:Add|TryAdd)(?:Scoped|Transient|Singleton)\s*<\s*([A-Z][\w.]*(?:<[^<>]*>)?)\s*,\s*([A-Z][\w.]*(?:<[^<>]*>)?)\s*>`)
	dotnetTypeof = regexp.MustCompile(`\.(?:Add|TryAdd)(?:Scoped|Transient|Singleton)\s*\(\s*typeof\s*\(\s*([A-Z][\w.<>,]*)\s*\)\s*,\s*typeof\s*\(\s*([A-Z][\w.<>,]*)\s*\)`)
	autofac      = regexp.MustCompile(`RegisterType\s*<\s*([A-Z][\w.]*)\s*>\s*\(\s*\)[^;]*?\.As\s*<\s*([A-Z][\w.]*)\s*>`)
)

func entities(s *r.Source) []r.Entity {
	var out []r.Entity
	for _, d := range s.Decls() {
		if !d.Class {
			continue
		}
		t := d.Has("Table")
		if t == nil {
			continue
		}
		out = append(out, r.Entity{Class: d.Name, Name: d.Name, Table: t.Arg("name", "value", "Name"), Store: "sql", Framework: "ef", File: s.Path, Line: d.Line})
	}
	for _, m := range efDbSet.FindAllStringSubmatchIndex(s.Code, -1) {
		out = append(out, r.Entity{Class: s.Code[m[2]:m[3]], Name: s.Code[m[2]:m[3]], Table: s.Code[m[4]:m[5]], Store: "sql", Framework: "ef", File: s.Path, Line: s.Line(m[0])})
	}
	return out
}

func accesses(s *r.Source) []r.Access {
	var out []r.Access
	for _, m := range efSet.FindAllStringSubmatchIndex(s.Code, -1) {
		out = append(out, r.Access{Target: s.Code[m[2]:m[3]], Access: r.AccessReadWrite, Via: "dbset", File: s.Path, Line: s.Line(m[0])})
	}
	return out
}

func bindings(s *r.Source) []r.Binding {
	out := r.Pairs(s, dotnetAdd, "dotnet_di", false)
	out = append(out, r.Pairs(s, dotnetTypeof, "dotnet_di", false)...)
	return append(out, r.Pairs(s, autofac, "autofac", true)...)
}

// ---------------------------------------------------------------------------
// Symbols.
// ---------------------------------------------------------------------------

var (
	// C# property: `public IOrderService Orders { get; }`.
	csProperty     = regexp.MustCompile(`\b([A-Z]\w*(?:<[^;=(){}]*?>)?)\s+([A-Z]\w*)\s*\{\s*(?:get|init|private get)`)
	optionsSection = regexp.MustCompile(`Configure\s*<\s*(\w+)\s*>\s*\(\s*[\w.]*GetSection\s*\(\s*"([^"]+)"|AddOptions\s*<\s*(\w+)\s*>\s*\(\s*\)\s*\.\s*Bind(?:Configuration)?\s*\(\s*(?:[\w.]*GetSection\s*\(\s*)?"([^"]+)"`)
)

func declared(s *r.Source, sy *r.Symbols) {
	r.TypedNames(s, sy)
	for _, m := range csProperty.FindAllStringSubmatch(s.Code, -1) {
		sy.SetVar(m[2], m[1])
	}
}

// configKeys are .NET options bound to a section:
// Configure<PartnerOptions>(config.GetSection("Partner")).
func configKeys(s *r.Source, sy *r.Symbols) {
	for _, m := range optionsSection.FindAllStringSubmatch(s.Code, -1) {
		sy.Prefixes[sy.In(m[1]+m[3])] = sy.In(m[2] + m[4])
	}
}

// ---------------------------------------------------------------------------
// Outbound calls.
// ---------------------------------------------------------------------------

func clientName(s *r.Source, lt, recv string) string {
	if strings.Contains(lt, "restclient") {
		return "restsharp"
	}
	if n := r.ClientLibrary(lt, recv); n != "" {
		return n
	}
	return "httpclient"
}

// clientBases are a HttpClient's BaseAddress, typed and named clients
// registered with one, Refit's RestService.For and RestSharp's RestClient.
func clientBases(s *r.Source, sy *r.Symbols) []r.ClientBase {
	code := s.Code
	var out []r.ClientBase
	if !r.Rx(`BaseAddress|AddHttpClient|AddRefitClient|RestService|RestClient`).MatchString(code) {
		return nil
	}
	add := func(name, expr string, at int) {
		if name == "" || strings.TrimSpace(expr) == "" {
			return
		}
		out = append(out, r.ClientBase{Name: name, Addr: s.RenderAddr(expr, sy), Resilience: r.ResilienceIn(s.Statement(at)), Line: s.Line(at)})
	}
	for _, m := range r.Rx(`\b([A-Za-z_]\w*)\s*\.\s*BaseAddress\s*=\s*new\s+Uri\s*\(`).FindAllStringSubmatchIndex(code, -1) {
		inner, _ := r.Balanced(code, m[1]-1)
		add(code[m[2]:m[3]], inner, m[0])
	}
	// AddHttpClient<IOrders, Orders>(c => c.BaseAddress = new Uri(...)),
	// AddHttpClient("orders", ...), AddRefitClient<IOrders>().ConfigureHttpClient(...).
	for _, m := range r.Rx(`\bAdd(?:HttpClient|RefitClient)\s*(?:<\s*([\w.]+)\s*(?:,\s*([\w.]+)\s*)?>)?\s*\(\s*(?:"([^"]+)")?`).FindAllStringSubmatchIndex(code, -1) {
		stmt := s.StatementFrom(m[0])
		b := r.Rx(`BaseAddress\s*=\s*new\s+Uri\s*\(`).FindStringIndex(stmt)
		expr := ""
		if b != nil {
			expr, _ = r.Balanced(stmt, b[1]-1)
		}
		res := r.ResilienceIn(stmt)
		for _, g := range []int{2, 4, 6} {
			if m[g] >= 0 {
				name := code[m[g]:m[g+1]]
				out = append(out, r.ClientBase{Name: name[strings.LastIndexByte(name, '.')+1:], Addr: s.RenderAddr(expr, sy), Resilience: res, Line: s.Line(m[0])})
			}
		}
	}
	for _, m := range r.Rx(`\bRestService\s*\.\s*For\s*<\s*(\w+)\s*>\s*\(`).FindAllStringSubmatchIndex(code, -1) {
		inner, _ := r.Balanced(code, m[1]-1)
		if args := r.SplitArgs(inner); len(args) > 0 {
			add(code[m[2]:m[3]], args[0], m[0])
		}
	}
	for _, m := range r.Rx(`\b([A-Za-z_]\w*)\s*=\s*(?:new\s+)?(?:RestClient)\s*\(`).FindAllStringSubmatchIndex(code, -1) {
		inner, _ := r.Balanced(code, m[1]-1)
		if args := r.SplitArgs(inner); len(args) > 0 {
			add(code[m[2]:m[3]], args[0], m[0])
		}
	}
	return out
}

var refitVerbs = map[string]bool{"Get": true, "Post": true, "Put": true, "Delete": true, "Patch": true, "Head": true, "Options": true}

// declaredClients are the methods of a Refit interface, each a call with its
// method and path.
func declaredClients(s *r.Source) []r.OutCall {
	var out []r.OutCall
	isInterface := func(line int) bool {
		for _, t := range s.TypeDecls() {
			if t.Line <= line {
				if strings.Contains(" "+t.Signature+" ", " interface ") {
					return true
				}
			}
		}
		return false
	}
	for _, d := range s.Decls() {
		if d.Class {
			continue
		}
		for _, m := range d.Marks {
			if !refitVerbs[m.Name] || !isInterface(d.Line) {
				continue
			}
			call := r.OutCall{Kind: "http", Client: "refit", Method: strings.ToUpper(m.Name), Declared: true, Line: m.Line}
			call.Class = s.TypeAt(d.Line)
			call.Addr = m.Arg("value")
			if call.Addr == "" {
				call.Addr = "/"
			}
			if res := s.ResilienceOf(d.Marks); res != "" {
				call.Resilience = res
			}
			out = append(out, call)
		}
	}
	return out
}

var (
	csRequestMsg = regexp.MustCompile(`\bnew\s+HttpRequestMessage\s*\(\s*(?:HttpMethod\.(\w+)|new\s+HttpMethod\s*\(\s*"(\w+)"\s*\))\s*,`)
	restSharpReq = regexp.MustCompile(`\bnew\s+RestRequest\s*\(`)
)

// calls are HttpRequestMessages and RestSharp requests.
func calls(s *r.Source, sy *r.Symbols, add func(r.OutCall, int)) {
	code := s.Code
	for _, m := range csRequestMsg.FindAllStringSubmatchIndex(code, -1) {
		open := strings.Index(code[m[0]:], "(") + m[0]
		inner, _ := r.Balanced(code, open)
		args := r.SplitArgs(inner)
		if len(args) < 2 {
			continue
		}
		method := ""
		if m[2] >= 0 {
			method = code[m[2]:m[3]]
		} else {
			method = code[m[4]:m[5]]
		}
		add(r.OutCall{Kind: "http", Client: "httpclient", Method: strings.ToUpper(method), Addr: s.RenderAddr(args[1], sy)}, m[0])
	}
	for _, loc := range restSharpReq.FindAllStringIndex(code, -1) {
		inner, _ := r.Balanced(code, loc[1]-1)
		args := r.SplitArgs(inner)
		if len(args) == 0 {
			continue
		}
		method := "GET"
		if len(args) > 1 {
			if mm := r.MethodLiteral(args[1]); mm != "" {
				method = mm
			}
		}
		add(r.OutCall{Kind: "http", Client: "restsharp", Method: method, Addr: s.RenderAddr(args[0], sy)}, loc[0])
	}
}

var callRules = []r.CallRule{
	{Members: regexp.MustCompile(`^(?:ProduceAsync|Produce)$`), Kind: "message", Client: "kafka", Arg: 0, Gate: `(?i)produc`},
	{Members: regexp.MustCompile(`^(?:Publish|Send)$`), Tail: regexp.MustCompile(`^\(\s*new\s+([A-Z]\w*)`), Kind: "message", Client: "masstransit", Arg: -1, Gate: `(?i)bus|publish|endpoint|sendendpoint`},
}

var outRules = []r.OutRule{
	// Messaging.
	{Re: regexp.MustCompile(`\bCreateSender\s*\(`), Kind: "message", Client: "azure-service-bus", Arg: 0},
	{Re: regexp.MustCompile(`\bBasicPublish(?:Async)?\s*\(`), Kind: "message", Client: "rabbitmq", Arg: 0},

	// Databases, caches, mail, LDAP, storage, file transfer.
	{Re: regexp.MustCompile(`\bnew\s+(?:SqlConnection|NpgsqlConnection|MySqlConnection|OracleConnection|SqliteConnection)\s*\(`), Kind: "sql", Client: "ado.net", Arg: 0},
	{Re: regexp.MustCompile(`\.\s*Use(?:SqlServer|Npgsql|MySql|MySQL|Oracle|Sqlite|Cosmos)\s*\(`), Kind: "sql", Client: "ef-core", Arg: 0},
	{Re: regexp.MustCompile(`\bnew\s+MongoClient\s*\(`), Kind: "nosql", Client: "mongodb", Arg: 0},
	{Re: regexp.MustCompile(`\bConnectionMultiplexer\s*\.\s*Connect(?:Async)?\s*\(`), Kind: "cache", Client: "stackexchange.redis", Arg: 0},
	{Re: regexp.MustCompile(`\bnew\s+SmtpClient\s*\(`), Kind: "mail", Client: "smtp", Arg: 0},
	{Re: regexp.MustCompile(`\bnew\s+(?:LdapConnection|DirectoryEntry)\s*\(`), Kind: "ldap", Client: "directory-services", Arg: 0},
	{Re: regexp.MustCompile(`\b(?:S3Client\s*\.\s*(?:builder|create)|new\s+S3Client|AmazonS3ClientBuilder|new\s+AmazonS3Client|s3\s*\.\s*New(?:FromConfig)?|new\s+BlobServiceClient|new\s+BlobContainerClient|BlobServiceClient\s*\.\s*fromConnectionString|StorageOptions\s*\.\s*getDefaultService|storage\s*\.\s*NewClient|new\s+Storage)\s*\(`), Kind: "storage", Client: "object-storage", Arg: 0},
	{Re: regexp.MustCompile(`\bnew\s+(?:FTPClient|FTPSClient|SftpClient|FtpClient|AsyncFtpClient)\s*\(`), Kind: "file_transfer", Client: "ftp", Arg: 0},

	// gRPC, GraphQL.
	{Re: regexp.MustCompile(`\bGrpcChannel\s*\.\s*ForAddress\s*\(`), Kind: "grpc", Client: "grpc-dotnet", Arg: 0},
	{Re: regexp.MustCompile(`\bnew\s+GraphQLHttpClient\s*\(`), Kind: "graphql", Client: "graphql-client", Arg: 0},
}
