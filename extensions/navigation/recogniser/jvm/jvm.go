// Package jvm reads Java, Kotlin and Scala for navigation: Spring, JAX-RS,
// Micronaut, picocli and the JVM's messaging, JPA and Hibernate mappings,
// Spring and Guice wiring, Spring Security, and the JVM's HTTP and data
// clients. It also reads the XML those frameworks are configured in.
package jvm

import (
	"regexp"
	"strings"

	r "github.com/archstats/archstats/extensions/navigation/recogniser"
)

const (
	java   = "java"
	kotlin = "kotlin"
	scala  = "scala"
)

var jvm = []string{java, kotlin, scala}

func syntax() r.Syntax {
	return r.Syntax{
		LineComments:  []string{"//"},
		BlockComments: [][2]string{{"/*", "*/"}},
		Strings:       true,
		TripleQuotes:  true,
		CharLiterals:  true,
		Marks:         r.AtMarks,
		TypeName:      r.CLikeTypeName,
	}
}

// Plugin is the JVM family.
func Plugin() r.Plugin {
	kt := syntax()
	kt.DollarInterpolation = true
	kt.ExpressionBodies = true
	return r.Plugin{
		Name: "jvm",
		Languages: []*r.Language{
			{Name: java, Extensions: []string{".java"}, Kind: r.Code, Syntax: syntax()},
			{Name: kotlin, Extensions: []string{".kt", ".kts"}, Kind: r.Code, Syntax: kt},
			{Name: scala, Extensions: []string{".scala"}, Kind: r.Code, Syntax: syntax()},
		},
		Recognisers: []*r.Recogniser{
			{
				Languages:       jvm,
				ClassMark:       classMark,
				ClassEntries:    classEntries,
				MemberEntries:   memberEntries,
				CallEntries:     callEntries,
				MainEntries:     mainEntries,
				AuthMarks:       []string{"PreAuthorize", "PostAuthorize", "Secured", "RolesAllowed", "PermitAll", "DenyAll", "RequiresPermissions", "RequiresRoles", "RequiresAuthentication", "Authenticated", "Anonymous"},
				DocMarks:        []string{"Operation", "ApiOperation", "ApiResponse", "ApiResponses", "Tag", "Api"},
				Entities:        entities,
				Accesses:        accesses,
				QueryVia:        queryVia,
				Bindings:        bindings,
				Scanned:         []string{"Component", "Service", "Repository", "Controller", "RestController", "Named", "Singleton", "ApplicationScoped", "RequestScoped", "Stateless", "AutoConfiguration"},
				Declared:        declared,
				Constants:       constants,
				ConfigKeys:      configKeys,
				Security:        security,
				ClientBases:     clientBases,
				DeclaredClients: declaredClients,
				Calls:           calls,
				ClientName:      clientName,
				Resilience:      map[string]string{"Retry": "retry", "Retryable": "retry", "CircuitBreaker": "circuit_breaker", "TimeLimiter": "timeout", "Timeout": "timeout", "Bulkhead": "retry", "Fallback": "circuit_breaker", "RateLimiter": "retry"},
				OutRules:        outRules,
				CallRules:       callRules,
			},
			{
				Languages: []string{"xml"},
				Entities:  xmlEntities,
				Bindings:  xmlBindings,
			},
		},
	}
}

// ---------------------------------------------------------------------------
// Entry points.
// ---------------------------------------------------------------------------

func classMark(m r.Mark, c *r.ClassContext) {
	switch m.Name {
	case "RequestMapping", "Path", "Controller":
		c.Prefix = m.Arg("value", "path", "prefix", "Template", "template")
	case "FeignClient", "RegisterRestClient", "HttpExchange", "RestClient":
		c.Client = true
	}
}

// classEntries are the entry points a type is by its marks alone: a
// command, a servlet, a websocket endpoint.
func classEntries(s *r.Source, d r.Decl) []r.Entry {
	e := r.Entry{Class: d.Name, Line: d.Line, DeclLine: d.Line}
	switch {
	case d.Has("Command") != nil && (s.Lang == java || s.Lang == kotlin) && d.Has("Command").Arg("name") != "":
		e.Kind, e.Framework, e.Path = r.KindCLI, "picocli", d.Has("Command").Arg("name")
	case d.Has("ShellComponent") != nil:
		return nil
	case d.Has("WebServlet") != nil:
		e.Kind, e.Framework, e.Path = r.KindHTTP, "servlet", d.Has("WebServlet").Arg("value", "urlPatterns")
	case d.Has("ServerEndpoint") != nil:
		e.Kind, e.Framework, e.Path = r.KindMessage, "websocket", d.Has("ServerEndpoint").Arg("value")
	default:
		return nil
	}
	return []r.Entry{e}
}

var httpVerbs = map[string]string{
	"GetMapping": "GET", "PostMapping": "POST", "PutMapping": "PUT", "DeleteMapping": "DELETE", "PatchMapping": "PATCH",
	"Get": "GET", "Post": "POST", "Put": "PUT", "Delete": "DELETE", "Patch": "PATCH", "Head": "HEAD", "Options": "OPTIONS", "All": "ANY",
}

var jaxrsVerbs = map[string]bool{"GET": true, "POST": true, "PUT": true, "DELETE": true, "PATCH": true, "HEAD": true, "OPTIONS": true}

var consumers = map[string]r.Consumer{
	"KafkaListener":              {Kind: r.KindMessage, Framework: "spring-kafka", Names: []string{"topics", "topicPattern", "value"}},
	"RabbitListener":             {Kind: r.KindMessage, Framework: "spring-amqp", Names: []string{"queues", "value"}},
	"JmsListener":                {Kind: r.KindMessage, Framework: "jms", Names: []string{"destination"}},
	"SqsListener":                {Kind: r.KindMessage, Framework: "spring-cloud-aws", Names: []string{"value", "queueNames"}},
	"StreamListener":             {Kind: r.KindMessage, Framework: "spring-cloud-stream", Names: []string{"value", "target"}},
	"ServiceActivator":           {Kind: r.KindMessage, Framework: "spring-integration", Names: []string{"inputChannel"}},
	"MessageMapping":             {Kind: r.KindMessage, Framework: "spring-messaging", Names: []string{"value"}},
	"Incoming":                   {Kind: r.KindMessage, Framework: "smallrye", Names: []string{"value"}},
	"EventListener":              {Kind: r.KindEvent, Framework: "spring", Names: []string{"classes", "value", "condition"}},
	"TransactionalEventListener": {Kind: r.KindEvent, Framework: "spring", Names: []string{"classes", "value"}},
	"Scheduled":                  {Kind: r.KindSchedule, Framework: "spring", Names: []string{"cron", "fixedRate", "fixedDelay", "every"}},
	"ShellMethod":                {Kind: r.KindCLI, Framework: "spring-shell", Names: []string{"key", "value"}},
}

// memberEntries are the entry points an annotated method is: Spring,
// JAX-RS and Micronaut routes, and the messages, events and schedules it
// consumes.
func memberEntries(s *r.Source, d r.Decl, class r.ClassContext, e r.Entry) []r.Entry {
	var out []r.Entry
	add := func(x r.Entry) { out = append(out, x) }

	if m := d.Has("RequestMapping"); m != nil {
		x := e
		x.Kind, x.Framework = r.KindHTTP, "spring"
		x.Method = requestMethod(m.Args)
		x.Path = r.JoinRoute(class.Prefix, m.Arg("value", "path"))
		x.Line = m.Line
		add(x)
	}
	for _, m := range d.Marks {
		verb, ok := httpVerbs[m.Name]
		if !ok || strings.Contains(m.Full, ".") {
			continue
		}
		x := e
		x.Kind, x.Method, x.Line = r.KindHTTP, verb, m.Line
		if strings.HasSuffix(m.Name, "Mapping") {
			x.Framework = "spring"
		} else {
			x.Framework = "micronaut"
		}
		x.Path = routeOf(class, m.Arg("value", "path", "uri", "template", "Template"))
		add(x)
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
		x.Kind, x.Method, x.Framework, x.Line = r.KindHTTP, m.Name, "jax-rs", m.Line
		p := ""
		if pm := d.Has("Path"); pm != nil {
			p = pm.Arg("value")
		}
		x.Path = r.JoinRoute(class.Prefix, p)
		add(x)
	}
	for _, m := range d.Marks {
		if c, ok := consumers[m.Name]; ok {
			add(c.Consume(m, e))
		}
	}
	return out
}

// routeOf joins a member's route to its class's; ~/ means "from the root".
func routeOf(class r.ClassContext, member string) string {
	prefix := class.Prefix
	if strings.HasPrefix(member, "~/") {
		prefix, member = "", strings.TrimPrefix(member, "~")
	}
	return r.JoinRoute(prefix, member)
}

// requestMethod is the method a @RequestMapping restricts itself to, or ANY.
func requestMethod(args []string) string {
	m := r.Named(args, "method")
	if m == "" {
		return "ANY"
	}
	var verbs []string
	for _, v := range regexp.MustCompile(`(GET|POST|PUT|DELETE|PATCH|HEAD|OPTIONS)`).FindAllString(strings.Join(args, ","), -1) {
		if !r.Contains(verbs, v) {
			verbs = append(verbs, v)
		}
	}
	if len(verbs) == 0 {
		return "ANY"
	}
	return strings.Join(verbs, ",")
}

var ktorRoute = regexp.MustCompile(`\b(get|post|put|patch|delete|head|options)\s*\(\s*"([^"]*)"\s*\)\s*\{`)

// callEntries are Ktor's routing DSL.
func callEntries(s *r.Source) []r.Entry {
	if s.Lang != kotlin || !strings.Contains(s.Code, "routing") {
		return nil
	}
	var out []r.Entry
	for _, m := range ktorRoute.FindAllStringSubmatchIndex(s.Code, -1) {
		out = append(out, r.Entry{Kind: r.KindHTTP, Method: strings.ToUpper(s.Code[m[2]:m[3]]), Path: r.JoinRoute("", s.Code[m[4]:m[5]]), Framework: "ktor", Line: s.Line(m[0])})
	}
	return out
}

var (
	javaMain   = regexp.MustCompile(`(?m)\bpublic\s+static\s+void\s+main\s*\(\s*(?:final\s+)?String`)
	kotlinMain = regexp.MustCompile(`(?m)^\s*fun\s+main\s*\(`)
)

func mainEntries(s *r.Source) []r.Entry {
	switch s.Lang {
	case java:
		if loc := javaMain.FindStringIndex(s.Code); loc != nil {
			return []r.Entry{r.MainEntry(s, loc[0], "main")}
		}
	case kotlin:
		if loc := kotlinMain.FindStringIndex(s.Code); loc != nil {
			return []r.Entry{r.MainEntry(s, loc[0], "")}
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Data.
// ---------------------------------------------------------------------------

func entities(s *r.Source) []r.Entity {
	var out []r.Entity
	for _, d := range s.Decls() {
		if !d.Class {
			continue
		}
		e := r.Entity{Class: d.Name, Name: d.Name, File: s.Path, Line: d.Line}
		switch {
		case d.Has("Entity") != nil:
			e.Store, e.Framework = "sql", "jpa"
			e.Name = r.OrDefault(d.Has("Entity").Arg("name"), d.Name)
		case d.Has("Document") != nil && (s.Lang == java || s.Lang == kotlin):
			e.Store, e.Framework = "document", "spring-data"
			e.Table = d.Has("Document").Arg("collection", "value", "indexName")
		default:
			continue
		}
		if t := d.Has("Table"); t != nil && e.Table == "" {
			e.Table = t.Arg("name", "value", "Name")
		}
		out = append(out, e)
	}
	return out
}

var (
	hbmClass = regexp.MustCompile(`<(class|joined-subclass|union-subclass|subclass)\b[^>]*?\bname\s*=\s*"([\w.$]+)"[^>]*>`)
	xmlTable = regexp.MustCompile(`\b(?:table|collection)\s*=\s*"([\w.]+)"`)
)

// xmlEntities are the classes a Hibernate mapping file maps.
func xmlEntities(s *r.Source) []r.Entity {
	var out []r.Entity
	for _, loc := range hbmClass.FindAllStringSubmatchIndex(s.Code, -1) {
		if !strings.Contains(s.Code, "hibernate-mapping") {
			break
		}
		tag := s.Code[loc[0]:loc[1]]
		e := r.Entity{Class: s.Code[loc[4]:loc[5]], Store: "sql", Framework: "hibernate-xml", File: s.Path, Line: s.Line(loc[0])}
		if m := xmlTable.FindStringSubmatch(tag); m != nil {
			e.Table = m[1]
		}
		e.Name = e.Class[strings.LastIndexAny(e.Class, ".$")+1:]
		out = append(out, e)
	}
	return out
}

// queryVia: JPQL and HQL name entities with an alias right after: `from
// Order o`. The linker decides by what the name resolves to.
func queryVia(s *r.Source) string {
	if s.Lang == java || s.Lang == kotlin {
		return "sql|jpql"
	}
	return ""
}

var jpaFind = regexp.MustCompile(`\.(find|getReference|persist|merge|remove)\s*\(\s*([A-Z]\w*)\.class`)

func accesses(s *r.Source) []r.Access {
	var out []r.Access
	for _, m := range jpaFind.FindAllStringSubmatchIndex(s.Code, -1) {
		kind := r.AccessWrite
		if op := s.Code[m[2]:m[3]]; op == "find" || op == "getReference" {
			kind = r.AccessRead
		}
		out = append(out, r.Access{Target: s.Code[m[4]:m[5]], Access: kind, Via: "orm", File: s.Path, Line: s.Line(m[0])})
	}
	return out
}

// ---------------------------------------------------------------------------
// Wiring.
// ---------------------------------------------------------------------------

var (
	guiceBind    = regexp.MustCompile(`\bbind\s*\(\s*([A-Z][\w.]*)(?:\.class|::class)\s*\)\s*\.\s*to\s*\(\s*([A-Z][\w.]*)(?:\.class|::class)`)
	koinSingle   = regexp.MustCompile(`\b(?:single|factory|scoped)\s*<\s*([A-Z]\w*)\s*>\s*\{\s*([A-Z]\w*)\s*\(`)
	hiltBinds    = regexp.MustCompile(`@Binds\b[^;{}]*?\bfun\s+\w+\s*\(\s*\w+\s*:\s*([A-Z]\w*)\s*\)\s*:\s*([A-Z]\w*)`)
	springBean   = regexp.MustCompile(`<bean\b[^>]*>`)
	xmlAttr      = regexp.MustCompile(`\b(id|name|class)\s*=\s*"([^"]*)"`)
	newObject    = regexp.MustCompile(`\bnew\s+([A-Z][\w.]*)\s*[(<{]`)
	kotlinCtor   = regexp.MustCompile(`=\s*([A-Z]\w*)\s*\(`)
	beanReturn   = regexp.MustCompile(`(?:^|\s)([A-Z][\w.]*(?:<[^()]*>)?)\s+[a-z_]\w*\s*\(`)
	kotlinReturn = regexp.MustCompile(`\)\s*:\s*([A-Z][\w.]*)`)
)

func bindings(s *r.Source) []r.Binding {
	out := r.Pairs(s, guiceBind, "guice", false)
	if s.Lang == kotlin {
		out = append(out, r.Pairs(s, koinSingle, "koin", false)...)
		out = append(out, r.Pairs(s, hiltBinds, "hilt", true)...)
	}
	for _, d := range s.Decls() {
		if d.Class || d.Has("Bean", "Provides") == nil {
			continue
		}
		mechanism := "bean"
		if d.Has("Provides") != nil {
			mechanism = "dagger"
		}
		returns := ""
		if m := beanReturn.FindStringSubmatch(" " + d.Text); m != nil && s.Lang != kotlin {
			returns = m[1]
		} else if m := kotlinReturn.FindStringSubmatch(d.Text); m != nil {
			returns = m[1]
		}
		if i := strings.IndexByte(returns, '<'); i > 0 {
			returns = returns[:i]
		}
		if returns == "" {
			continue
		}
		out = append(out, r.Binding{Interface: returns, Implementation: beanImplementation(s.Body(d.Line)), Mechanism: mechanism, File: s.Path, Line: d.Marks[0].Line, DeclLine: d.Line})
	}
	return out
}

// beanImplementation is what a @Bean method constructs: the first `new X(`
// in its body, or Kotlin's `= X(`.
func beanImplementation(body string) string {
	if m := newObject.FindStringSubmatch(body); m != nil {
		return m[1]
	}
	if m := kotlinCtor.FindStringSubmatch(body); m != nil {
		return m[1]
	}
	return ""
}

// xmlBindings are the beans a Spring XML context declares.
func xmlBindings(s *r.Source) []r.Binding {
	var out []r.Binding
	if !strings.Contains(s.Code, "springframework.org/schema/beans") && !strings.Contains(s.Code, "<beans") {
		return nil
	}
	for _, loc := range springBean.FindAllStringIndex(s.Code, -1) {
		attrs := map[string]string{}
		for _, a := range xmlAttr.FindAllStringSubmatch(s.Code[loc[0]:loc[1]], -1) {
			attrs[a[1]] = a[2]
		}
		if attrs["class"] == "" {
			continue
		}
		id := attrs["id"]
		if names := strings.Fields(strings.ReplaceAll(attrs["name"], ",", " ")); id == "" && len(names) > 0 {
			id = names[0]
		}
		out = append(out, r.Bind(s, id, attrs["class"], "spring_xml", loc[0]))
	}
	return out
}
