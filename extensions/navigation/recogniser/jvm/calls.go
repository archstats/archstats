package jvm

import (
	"regexp"
	"strings"

	r "github.com/archstats/archstats/extensions/navigation/recogniser"
)

// ---------------------------------------------------------------------------
// Symbols.
// ---------------------------------------------------------------------------

var (
	valueKey      = regexp.MustCompile(`@(?:Value|ConfigProperty)\s*\(\s*(?:name\s*=\s*|value\s*=\s*)?"\$?\{?([^}:"]+)(?::[^}"]*)?\}?"[^)]*\)\s*(?:(?:private|protected|public|final|lateinit|val|var|readonly)\s+)*(?:[A-Z][\w.<>]*\s+)?([a-z_]\w*)`)
	configProps   = regexp.MustCompile(`@ConfigurationProperties\s*\(\s*(?:prefix\s*=\s*|value\s*=\s*)?"([^"]+)"`)
	securityMatch = regexp.MustCompile(`\.(?:requestMatchers|antMatchers|mvcMatchers|regexMatchers|pathMatchers|securityMatcher)\s*\(`)
	securityRule  = regexp.MustCompile(`^\s*\.\s*(permitAll|authenticated|denyAll|anonymous|fullyAuthenticated|rememberMe|hasRole|hasAnyRole|hasAuthority|hasAnyAuthority|access|hasIpAddress)\s*\(([^()]*(?:\([^()]*\)[^()]*)*)\)`)
	anyRequest    = regexp.MustCompile(`\.anyRequest\s*\(\s*\)\s*\.\s*(permitAll|authenticated|denyAll|hasRole|hasAnyRole|hasAuthority|hasAnyAuthority|access)\s*\(([^()]*)\)`)
	kotlinAuthz   = regexp.MustCompile(`\bauthorize\s*\(\s*(?:HttpMethod\.(\w+)\s*,\s*)?("[^"]*"|anyRequest)\s*,\s*(permitAll|authenticated|denyAll|hasRole\s*\([^)]*\)|hasAnyRole\s*\([^)]*\)|hasAuthority\s*\([^)]*\)|hasAnyAuthority\s*\([^)]*\))`)
)

func declared(s *r.Source, sy *r.Symbols) {
	if s.Lang == java {
		r.TypedNames(s, sy)
		return
	}
	r.ColonTypes(s, sy)
}

func constants(s *r.Source, sy *r.Symbols) {
	r.StaticConstants(s, sy)
	if s.Lang == kotlin {
		r.AssignedConstants(s, sy)
	}
}

// configKeys are the configuration keys bound to fields (@Value,
// MicroProfile's @ConfigProperty) and the prefixes bound to types
// (@ConfigurationProperties).
func configKeys(s *r.Source, sy *r.Symbols) {
	for _, m := range valueKey.FindAllStringSubmatch(s.Code, -1) {
		sy.Keys[sy.In(m[2])] = sy.In(m[1])
	}
	for _, loc := range configProps.FindAllStringSubmatchIndex(s.Code, -1) {
		if t := s.TypeNear(s.Line(loc[0])); t != "" {
			sy.Prefixes[sy.In(t)] = sy.In(s.Code[loc[2]:loc[3]])
		}
	}
}

// security reads the access Spring Security's matchers give paths, in Java
// and the Kotlin DSL, and the paths a JAX-RS application is mounted under.
func security(s *r.Source, sy *r.Symbols) {
	code := s.Code
	if s.Lang == scala || !containsAny(code, "Matcher", "anyRequest", "authorize", "ApplicationPath") {
		return
	}
	for _, m := range r.Rx(`@ApplicationPath\s*\(\s*"([^"]*)"`).FindAllStringSubmatch(code, -1) {
		sy.AppPaths = append(sy.AppPaths, m[1])
	}
	// A filter chain's securityMatcher scopes its rules, anyRequest()
	// included: anyRequest in a chain for /actuator/** is not every
	// route.
	type scope struct {
		at       int
		patterns []string
	}
	var scopes []scope
	chains := r.Rx(`\bSecurityFilterChain\b|\bSecurityWebFilterChain\b`).FindAllStringIndex(code, -1)
	chainOf := func(at int) int {
		c := -1
		for _, ch := range chains {
			if ch[0] <= at {
				c = ch[0]
			}
		}
		return c
	}
	for _, loc := range r.Rx(`\.\s*securityMatchers?\s*\(`).FindAllStringIndex(code, -1) {
		inner, _ := r.Balanced(code, loc[1]-1)
		_, patterns := matcherArgs(inner)
		scopes = append(scopes, scope{at: loc[0], patterns: patterns})
	}
	scopeOf := func(at int) ([]string, bool) {
		chain := chainOf(at)
		var found *scope
		for i := range scopes {
			if scopes[i].at < at && chainOf(scopes[i].at) == chain {
				found = &scopes[i]
			}
		}
		if found == nil {
			return []string{"/**"}, true
		}
		return found.patterns, len(found.patterns) > 0
	}
	for _, loc := range securityMatch.FindAllStringIndex(code, -1) {
		if strings.Contains(code[loc[0]:loc[1]], "securityMatcher") {
			continue
		}
		inner, end := r.Balanced(code, loc[1]-1)
		rule := securityRule.FindStringSubmatch(code[end:min(len(code), end+300)])
		if rule == nil {
			continue
		}
		rl := rule[1]
		if strings.TrimSpace(rule[2]) != "" {
			rl += "(" + strings.Join(strings.Fields(rule[2]), " ") + ")"
		}
		method, patterns := matcherArgs(inner)
		for _, p := range patterns {
			sy.AuthRules = append(sy.AuthRules, r.AuthRule{Pattern: p, Method: method, Rule: rl, Line: s.Line(loc[0])})
		}
	}
	for _, m := range anyRequest.FindAllStringSubmatchIndex(code, -1) {
		rl := code[m[2]:m[3]]
		if a := strings.TrimSpace(code[m[4]:m[5]]); a != "" {
			rl += "(" + a + ")"
		}
		patterns, known := scopeOf(m[0])
		if !known {
			continue
		}
		for _, p := range patterns {
			sy.AuthRules = append(sy.AuthRules, r.AuthRule{Pattern: p, Rule: rl, Line: s.Line(m[0])})
		}
	}
	for _, m := range kotlinAuthz.FindAllStringSubmatchIndex(code, -1) {
		p := "/**"
		if q := r.Unquote(code[m[4]:m[5]]); q != "" {
			p = q
		}
		method := ""
		if m[2] >= 0 {
			method = code[m[2]:m[3]]
		}
		sy.AuthRules = append(sy.AuthRules, r.AuthRule{Pattern: p, Method: method, Rule: strings.Join(strings.Fields(code[m[6]:m[7]]), ""), Line: s.Line(m[0])})
	}
}

// matcherArgs reads a matcher's method and path patterns, through a wrapper
// such as API_MATCHER.matcher(HttpMethod.GET, "/api/*/x").
func matcherArgs(inner string) (string, []string) {
	method := ""
	var patterns []string
	if m := r.Rx(`HttpMethod\s*\.\s*(\w+)`).FindStringSubmatch(inner); m != nil {
		method = m[1]
	}
	for _, m := range r.Rx(`"([^"]*)"`).FindAllStringSubmatch(inner, -1) {
		if strings.HasPrefix(m[1], "/") || m[1] == "*" || strings.HasPrefix(m[1], "**") {
			patterns = append(patterns, m[1])
		}
	}
	return method, patterns
}

func containsAny(text string, words ...string) bool {
	for _, w := range words {
		if strings.Contains(text, w) {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Outbound calls.
// ---------------------------------------------------------------------------

func clientName(s *r.Source, lt, recv string) string {
	if n := r.ClientLibrary(lt, recv); n != "" {
		return n
	}
	switch {
	case s.Lang == kotlin && strings.Contains(lt, "httpclient"):
		return "ktor-client"
	case s.Lang == java && strings.Contains(lt, "httpclient"):
		return "java.net.http"
	}
	return ""
}

// clientBases are the base addresses bound to WebClient and RestClient
// variables and to the clients @Bean methods build.
func clientBases(s *r.Source, sy *r.Symbols) []r.ClientBase {
	code := s.Code
	var out []r.ClientBase
	if !containsAny(code, "WebClient", "RestClient", "@Bean") {
		return nil
	}
	add := func(name, expr string, at int) {
		if name == "" || strings.TrimSpace(expr) == "" {
			return
		}
		out = append(out, r.ClientBase{Name: name, Addr: s.RenderAddr(expr, sy), Resilience: r.ResilienceIn(s.Statement(at)), Line: s.Line(at)})
	}
	for _, m := range r.Rx(`\b([A-Za-z_]\w*)\s*(?:=|:=)\s*(?:WebClient|RestClient)\s*\.\s*(?:builder|create)\s*\(`).FindAllStringSubmatchIndex(code, -1) {
		stmt := s.StatementFrom(m[0])
		if b := r.Rx(`(?:baseUrl|create)\s*\(`).FindStringIndex(stmt); b != nil {
			inner, _ := r.Balanced(stmt, b[1]-1)
			add(code[m[2]:m[3]], inner, m[0])
		}
	}
	// A @Bean method that returns a client built with a base: the bean is
	// injected by its method's name.
	for _, d := range s.Decls() {
		if d.Class || d.Has("Bean") == nil {
			continue
		}
		body := s.Body(d.Line)
		if b := r.Rx(`(?:\.baseUrl|WebClient\s*\.\s*create|\.rootUri|DefaultUriBuilderFactory)\s*\(`).FindStringIndex(body); b != nil {
			inner, _ := r.Balanced(body, b[1]-1)
			if inner != "" {
				out = append(out, r.ClientBase{Name: d.Name, Addr: s.RenderAddr(inner, sy), Resilience: r.ResilienceIn(body), Line: d.Line})
			}
		}
	}
	return out
}

// declaredClients are the methods of an HTTP client interface, each a call
// with its method and path: Feign, MicroProfile Rest Client, Spring's
// HttpExchange, Retrofit.
func declaredClients(s *r.Source) []r.OutCall {
	var out []r.OutCall
	decls := s.Decls()
	type client struct {
		name, base, target, path, kind string
		line                           int
	}
	var clients []client
	for _, d := range decls {
		if !d.Class {
			continue
		}
		for _, m := range d.Marks {
			c := client{name: d.Name, line: d.Line}
			switch m.Name {
			case "FeignClient":
				c.kind, c.target = "feign", m.Arg("name", "value", "contextId")
				c.base = r.Named(m.Args, "url")
				c.path = r.Named(m.Args, "path")
			case "RegisterRestClient":
				c.kind, c.target = "microprofile", r.Named(m.Args, "configKey")
				c.base = r.Named(m.Args, "baseUri")
				if p := d.Has("Path"); p != nil {
					c.path = p.Arg("value")
				}
			case "HttpExchange":
				c.kind = "http-exchange"
				c.base = m.Arg("url", "value")
			default:
				continue
			}
			clients = append(clients, c)
		}
	}
	enclosingClient := func(line int) *client {
		var found *client
		for i := range clients {
			if clients[i].line <= line {
				found = &clients[i]
			}
		}
		return found
	}
	for _, d := range decls {
		if d.Class {
			continue
		}
		c := enclosingClient(d.Line)
		for _, m := range d.Marks {
			method, p := "", ""
			kind := ""
			switch {
			case c != nil && (c.kind == "feign") && (strings.HasSuffix(m.Name, "Mapping")):
				if m.Name == "RequestMapping" {
					method = requestMethod(m.Args)
				} else {
					method = strings.ToUpper(strings.TrimSuffix(m.Name, "Mapping"))
				}
				p, kind = m.Arg("value", "path"), c.kind
			case c != nil && c.kind == "microprofile" && jaxrsVerbs[m.Name]:
				method, kind = m.Name, c.kind
				if pm := d.Has("Path"); pm != nil {
					p = pm.Arg("value")
				}
			case c != nil && c.kind == "http-exchange" && strings.HasSuffix(m.Name, "Exchange"):
				method = strings.ToUpper(strings.TrimSuffix(m.Name, "Exchange"))
				if method == "HTTP" {
					method = strings.ToUpper(r.Named(m.Args, "method"))
				}
				p, kind = m.Arg("url", "value"), c.kind
			case jaxrsVerbs[m.Name] && len(m.Args) > 0:
				// Retrofit: @GET("users/{id}") on an interface method.
				method, p, kind = m.Name, m.Arg("value"), "retrofit"
			default:
				continue
			}
			call := r.OutCall{Kind: "http", Client: kind, Method: method, Declared: true, Line: m.Line}
			call.Class = s.TypeAt(d.Line)
			if c != nil && kind != "retrofit" {
				call.Target, call.Base = c.target, c.base
				p = r.JoinRoute(c.path, p)
			}
			call.Addr = p
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
	webClientChain = regexp.MustCompile(`([A-Za-z_$][\w$.]*)\s*\.\s*(get|post|put|patch|delete|head|options|method)\s*\(\s*(?:HttpMethod\.(\w+))?\s*\)\s*\.\s*uri\s*\(`)
	javaHTTPReq    = regexp.MustCompile(`HttpRequest\s*\.\s*newBuilder\s*\(`)
	okHTTPReq      = regexp.MustCompile(`\bRequest\s*\.\s*Builder\s*\(\s*\)`)
	apacheReq      = regexp.MustCompile(`\bnew\s+Http(Get|Post|Put|Delete|Patch|Head|Options)\s*\(`)
	jaxrsTarget    = regexp.MustCompile(`\.\s*target\s*\(`)
	unirestCall    = regexp.MustCompile(`\b[Uu]nirest\s*\.\s*(get|post|put|patch|delete|head|options)\s*\(`)
)

// calls are the JVM's HTTP calls that are not one member called on a
// client: WebClient chains, java.net.http and OkHttp requests, Apache
// HttpClient, JAX-RS client targets, Unirest.
func calls(s *r.Source, sy *r.Symbols, add func(r.OutCall, int)) {
	code := s.Code
	for _, m := range webClientChain.FindAllStringSubmatchIndex(code, -1) {
		if !strings.Contains(code, "uri(") {
			break
		}
		recv := r.NormalRecv(code[m[2]:m[3]])
		method := strings.ToUpper(code[m[4]:m[5]])
		if m[6] >= 0 {
			method = strings.ToUpper(code[m[6]:m[7]])
		}
		inner, _ := r.Balanced(code, m[1]-1)
		args := r.SplitArgs(inner)
		if len(args) == 0 {
			continue
		}
		add(r.OutCall{Kind: "http", Client: s.ClientName(recv, sy), Method: method, Addr: s.RenderAddr(args[0], sy), Recv: r.LastSegment(recv)}, m[0])
	}
	for _, loc := range javaHTTPReq.FindAllStringIndex(code, -1) {
		stmt := s.StatementFrom(loc[0])
		addr := ""
		if u := r.Rx(`(?:newBuilder|\.uri)\s*\(\s*(?:URI\s*\.\s*create|new\s+URI|URI)\s*\(`).FindStringIndex(stmt); u != nil {
			inner, _ := r.Balanced(stmt, u[1]-1)
			addr = inner
		}
		if addr == "" {
			continue
		}
		method := "GET"
		if mm := r.Rx(`\.\s*(GET|POST|PUT|DELETE|PATCH|HEAD)\s*\(|\.\s*method\s*\(\s*"(\w+)"`).FindStringSubmatch(stmt); mm != nil {
			method = mm[1] + mm[2]
		}
		add(r.OutCall{Kind: "http", Client: "java.net.http", Method: method, Addr: s.RenderAddr(addr, sy)}, loc[0])
	}
	for _, loc := range okHTTPReq.FindAllStringIndex(code, -1) {
		stmt := s.StatementFrom(loc[0])
		u := r.Rx(`\.\s*url\s*\(`).FindStringIndex(stmt)
		if u == nil {
			continue
		}
		inner, _ := r.Balanced(stmt, u[1]-1)
		method := "GET"
		if mm := r.Rx(`\.\s*(post|put|delete|patch|head)\s*\(`).FindStringSubmatch(stmt); mm != nil {
			method = strings.ToUpper(mm[1])
		}
		add(r.OutCall{Kind: "http", Client: "okhttp", Method: method, Addr: s.RenderAddr(inner, sy)}, loc[0])
	}
	for _, m := range apacheReq.FindAllStringSubmatchIndex(code, -1) {
		inner, _ := r.Balanced(code, m[1]-1)
		if args := r.SplitArgs(inner); len(args) > 0 {
			add(r.OutCall{Kind: "http", Client: "apache-httpclient", Method: strings.ToUpper(code[m[2]:m[3]]), Addr: s.RenderAddr(args[0], sy)}, m[0])
		}
	}
	for _, loc := range jaxrsTarget.FindAllStringIndex(code, -1) {
		inner, end := r.Balanced(code, loc[1]-1)
		args := r.SplitArgs(inner)
		if len(args) != 1 {
			continue
		}
		rest := code[end:min(len(code), end+400)]
		if !strings.Contains(rest, ".request(") {
			continue
		}
		addr := s.RenderAddr(args[0], sy)
		for _, p := range r.Rx(`\.\s*path\s*\(\s*("[^"]*")\s*\)`).FindAllStringSubmatch(rest[:strings.Index(rest, ".request(")], -1) {
			addr = strings.TrimRight(addr, "/") + "/" + strings.TrimLeft(r.Unquote(p[1]), "/")
		}
		method := "GET"
		if mm := r.Rx(`\.\s*(get|post|put|delete|patch|head|method)\s*\(\s*(?:"(\w+)")?`).FindStringSubmatch(rest[strings.Index(rest, ".request("):]); mm != nil {
			method = strings.ToUpper(mm[1])
			if mm[1] == "method" && mm[2] != "" {
				method = strings.ToUpper(mm[2])
			}
		}
		add(r.OutCall{Kind: "http", Client: "jax-rs-client", Method: method, Addr: addr}, loc[0])
	}
	for _, m := range unirestCall.FindAllStringSubmatchIndex(code, -1) {
		inner, _ := r.Balanced(code, m[1]-1)
		if args := r.SplitArgs(inner); len(args) > 0 {
			add(r.OutCall{Kind: "http", Client: "unirest", Method: strings.ToUpper(code[m[2]:m[3]]), Addr: s.RenderAddr(args[0], sy)}, m[0])
		}
	}
}

var javaKotlin = []string{java, kotlin}

var callRules = []r.CallRule{
	{Members: regexp.MustCompile(`^(?:send|sendDefault)$`), Kind: "message", Client: "kafka", Arg: 0, Gate: `(?i)kafka`},
	{Members: regexp.MustCompile(`^(?:convertAndSend|send)$`), Kind: "message", Client: "amqp", Arg: 0, Gate: `(?i)rabbit|amqp`},
	{Members: regexp.MustCompile(`^(?:convertAndSend|send)$`), Kind: "message", Client: "jms", Arg: 0, Gate: `(?i)jms`},
	{Members: regexp.MustCompile(`^(?:send)$`), Kind: "message", Client: "spring-cloud-stream", Arg: 0, Gate: `(?i)streambridge`},
	{Langs: javaKotlin, Members: regexp.MustCompile(`^(?:send)$`), Kind: "mail", Client: "javamail", Arg: -1, Gate: `(?i)mailsender|javamailsender|^mail`},
	{Langs: javaKotlin, Members: regexp.MustCompile(`^(?:getSession)$`), Kind: "file_transfer", Client: "jsch", Arg: 1, Gate: `(?i)jsch`},
	{Langs: javaKotlin, Members: regexp.MustCompile(`^(?:marshalSendAndReceive|sendSourceAndReceive)$`), Kind: "soap", Client: "spring-ws", Arg: 0, Gate: `(?i)webservicetemplate|^ws|soap`},
	{Langs: javaKotlin, Members: regexp.MustCompile(`^setAddress$`), Kind: "soap", Client: "jax-ws", Arg: 0, Gate: `(?i)factory|proxy|jaxws`},
}

var outRules = []r.OutRule{
	// Databases, caches, search, LDAP, storage, file transfer.
	{Langs: javaKotlin, Re: regexp.MustCompile(`\bDriverManager\s*\.\s*getConnection\s*\(`), Kind: "sql", Client: "jdbc", Arg: 0},
	{Langs: javaKotlin, Re: regexp.MustCompile(`\b(?:MongoClients\s*\.\s*create|new\s+MongoClient)\s*\(`), Kind: "nosql", Client: "mongodb", Arg: 0},
	{Langs: javaKotlin, Re: regexp.MustCompile(`\bnew\s+(?:Jedis|JedisPool|JedisPooled)\s*\(`), Kind: "cache", Client: "jedis", Arg: 0},
	{Langs: javaKotlin, Re: regexp.MustCompile(`\bRedisClient\s*\.\s*create\s*\(`), Kind: "cache", Client: "lettuce", Arg: 0},
	{Langs: javaKotlin, Re: regexp.MustCompile(`\bRestClient\s*\.\s*builder\s*\(\s*(?:new\s+)?HttpHost`), Kind: "search", Client: "elasticsearch", Arg: -1},
	{Langs: javaKotlin, Re: regexp.MustCompile(`\bnew\s+(?:InitialDirContext|InitialLdapContext)\s*\(`), Kind: "ldap", Client: "jndi", Arg: -1},
	{Langs: javaKotlin, Re: regexp.MustCompile(`\b(?:S3Client\s*\.\s*(?:builder|create)|new\s+S3Client|AmazonS3ClientBuilder|new\s+AmazonS3Client|s3\s*\.\s*New(?:FromConfig)?|new\s+BlobServiceClient|new\s+BlobContainerClient|BlobServiceClient\s*\.\s*fromConnectionString|StorageOptions\s*\.\s*getDefaultService|storage\s*\.\s*NewClient|new\s+Storage)\s*\(`), Kind: "storage", Client: "object-storage", Arg: 0},
	{Langs: javaKotlin, Re: regexp.MustCompile(`\bnew\s+(?:FTPClient|FTPSClient|SftpClient|FtpClient|AsyncFtpClient)\s*\(`), Kind: "file_transfer", Client: "ftp", Arg: 0},

	// gRPC, SOAP.
	{Langs: javaKotlin, Re: regexp.MustCompile(`\b(\w+)Grpc\s*\.\s*new(?:Blocking|Future)?(?:V2)?Stub\s*\(`), Kind: "grpc", Client: "grpc-java", Arg: -1},
	{Langs: javaKotlin, Re: regexp.MustCompile(`\bManagedChannelBuilder\s*\.\s*for(?:Target|Address)\s*\(`), Kind: "grpc", Client: "grpc-java", Arg: 0},
	{Langs: javaKotlin, Re: regexp.MustCompile(`\bENDPOINT_ADDRESS_PROPERTY\s*,`), Kind: "soap", Client: "jax-ws", Arg: 0},
}
