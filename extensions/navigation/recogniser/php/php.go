// Package php reads PHP for navigation: Symfony's attribute, annotation and
// YAML routes, its messenger, console and scheduler, Laravel's routes,
// Doctrine and Eloquent, Symfony's and Laravel's containers, Laravel's
// configuration, and PHP's HTTP, messaging and mail clients. It also reads
// the XML Doctrine and Symfony are configured in.
package php

import (
	"path"
	"regexp"
	"strings"

	r "github.com/archstats/archstats/extensions/navigation/recogniser"
)

const php = "php"

// Plugin is PHP.
func Plugin() r.Plugin {
	return r.Plugin{
		Name: "php",
		Languages: []*r.Language{{
			Name: php, Extensions: []string{".php"}, Kind: r.Code,
			Syntax: r.Syntax{
				LineComments:        []string{"//", "#"},
				NotComments:         []string{"#["},
				BlockComments:       [][2]string{{"/*", "*/"}},
				Strings:             true,
				Marks:               r.BracketMarks,
				MarkOpen:            "#[",
				DocblockMarks:       true,
				TypeName:            r.CLikeTypeName,
				BraceInterpolation:  true,
				DollarInterpolation: true,
				DotConcat:           true,
			},
		}},
		Recognisers: []*r.Recogniser{
			{
				Languages:     []string{php},
				ClassMark:     classMark,
				ClassEntries:  classEntries,
				MemberEntries: memberEntries,
				CallEntries:   laravelRoutes,
				AuthMarks:     []string{"IsGranted", "Security"},
				DocMarks:      []string{"OA"},
				Entities:      entities,
				Accesses:      accesses,
				ModelBase: func(key string, _ func() []string) (string, bool) {
					if key == "Model" {
						return "eloquent", true
					}
					return "", false
				},
				Bindings: func(s *r.Source) []r.Binding {
					out := r.Pairs(s, laravelBind, "laravel", false)
					// Symfony's PHP service config: ->set('id', Class::class) and
					// ->alias(Interface::class, 'id').
					out = append(out, r.Pairs(s, symfonySet, "symfony_php", false)...)
					return append(out, r.Pairs(s, symfonyPHPAlias, "symfony_alias", false)...)
				},
				Declared: func(s *r.Source, sy *r.Symbols) {
					for _, m := range phpTyped.FindAllStringSubmatch(s.Code, -1) {
						sy.SetVar(m[2], m[1][strings.LastIndexByte(m[1], '\\')+1:])
					}
				},
				Constants: func(s *r.Source, sy *r.Symbols) {
					r.AssignedConstants(s, sy)
					for _, m := range r.Rx(`\bconst\s+([A-Z_][A-Z0-9_]*)\s*=\s*(['"][^'"]*['"])`).FindAllStringSubmatch(s.Code, -1) {
						sy.SetConst(m[1], r.Unquote(m[2]))
					}
				},
				Security:    security,
				ClientBases: clientBases,
				Calls:       calls,
				ClientName: func(s *r.Source, lt, recv string) string {
					if n := r.ClientLibrary(lt, recv); n != "" {
						return n
					}
					if strings.Contains(lt, "httpclientinterface") {
						return "symfony-http-client"
					}
					return "guzzle"
				},
				BareClient:    func(s *r.Source) bool { return strings.Contains(s.Raw, "GuzzleHttp") },
				ClientModules: []string{"Http"},
				OutRules:      outRules,
				ConfigFormat: func(p string) string {
					if strings.Contains("/"+p, "/config/") && !strings.Contains(p, "/vendor/") {
						return "laravel"
					}
					return ""
				},
				ReadConfig: func(p, _, text string, add func(key, value string, line int, env string)) {
					r.ReadNested(text, "=>", strings.TrimSuffix(path.Base(p), ".php"), add)
				},
			},
			{
				Languages:     []string{"yaml"},
				Routes:        yamlRoutes,
				RoutingFile:   isRoutingFile,
				ImportTargets: importTargets,
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
	if m.Name == "Route" {
		c.Prefix = m.Arg("value", "path", "prefix", "Template", "template")
	}
}

// classEntries are the entry points a class is by its attributes alone: a
// message handler, a command, an event listener.
func classEntries(s *r.Source, d r.Decl) []r.Entry {
	e := r.Entry{Class: d.Name, Line: d.Line, DeclLine: d.Line}
	switch {
	case d.Has("AsMessageHandler") != nil:
		e.Kind, e.Framework = r.KindMessage, "symfony-messenger"
	case d.Has("AsCommand") != nil:
		e.Kind, e.Framework, e.Path = r.KindCLI, "symfony-console", d.Has("AsCommand").Arg("name")
	case d.Has("AsEventListener") != nil:
		e.Kind, e.Framework, e.Path = r.KindEvent, "symfony", d.Has("AsEventListener").Arg("event")
		e.Mountable = true
	default:
		return nil
	}
	return []r.Entry{e}
}

var consumers = map[string]r.Consumer{
	"AsMessageHandler": {Kind: r.KindMessage, Framework: "symfony-messenger"},
	"AsEventListener":  {Kind: r.KindEvent, Framework: "symfony", Names: []string{"event"}},
	"AsCronTask":       {Kind: r.KindSchedule, Framework: "symfony-scheduler", Names: []string{"expression"}},
	"AsPeriodicTask":   {Kind: r.KindSchedule, Framework: "symfony-scheduler", Names: []string{"frequency"}},
}

// memberEntries are a Symfony controller action's routes, and the messages,
// events and schedules a method consumes.
func memberEntries(s *r.Source, d r.Decl, class r.ClassContext, e r.Entry) []r.Entry {
	var out []r.Entry
	if m := d.Has("Route"); m != nil {
		x := e
		x.Kind, x.Framework, x.Line, x.Mountable = r.KindHTTP, "symfony", m.Line, true
		x.Method = strings.ToUpper(r.Named(m.Args, "methods"))
		if x.Method == "" {
			x.Method = "ANY"
		}
		x.Path = r.JoinRoute(class.Prefix, m.Arg("path", "name"))
		out = append(out, x)
	}
	for _, m := range d.Marks {
		if c, ok := consumers[m.Name]; ok {
			x := c.Consume(m, e)
			x.Mountable = x.Framework == "symfony"
			out = append(out, x)
		}
	}
	return out
}

var laravelRoute = regexp.MustCompile(`Route::(get|post|put|patch|delete|options|any|match|resource|apiResource)\s*\(`)

func laravelRoutes(s *r.Source) []r.Entry {
	var out []r.Entry
	for _, loc := range laravelRoute.FindAllStringSubmatchIndex(s.Code, -1) {
		verb := s.Code[loc[2]:loc[3]]
		inner, end := r.Balanced(s.Code, loc[1]-1)
		args := r.SplitArgs(inner)
		method := strings.ToUpper(verb)
		if verb == "match" && len(args) > 1 {
			method = strings.ToUpper(strings.Join(regexp.MustCompile(`[A-Za-z]+`).FindAllString(args[0], -1), ","))
			args = args[1:]
		}
		if verb == "any" {
			method = "ANY"
		}
		if len(args) < 1 {
			continue
		}
		e := r.Entry{Kind: r.KindHTTP, Method: method, Path: r.JoinRoute("", r.Unquote(args[0])), Framework: "laravel", Line: s.Line(loc[0])}
		if len(args) > 1 {
			e.Handler = r.LastName(args[1])
		}
		tail := s.Code[end:min(len(s.Code), end+300)]
		if mw := r.Rx(`^(?:\s*->\s*\w+\s*\([^()]*\))*?\s*->\s*middleware\s*\(([^()]*)\)`).FindStringSubmatch(tail); mw != nil && r.Authish.MatchString(mw[1]) {
			e.Auth = "middleware(" + strings.Join(strings.Fields(mw[1]), " ") + ")"
		}
		if verb == "resource" || verb == "apiResource" {
			out = append(out, resourceRoutes(e, verb == "apiResource", tail)...)
			continue
		}
		out = append(out, e)
	}
	return out
}

// A Laravel resource route is its controller's standard actions, each
// answering its method by its name: index and show GET, store POST, update
// PUT or PATCH, destroy DELETE, and for a web resource create and edit GET.
// ->only() and ->except() narrow them.
type resourceAction struct{ name, method, path string }

var resourceActions = []resourceAction{
	{"index", "GET", ""}, {"create", "GET", "/create"}, {"store", "POST", ""}, {"show", "GET", "/{id}"},
	{"edit", "GET", "/{id}/edit"}, {"update", "PUT,PATCH", "/{id}"}, {"destroy", "DELETE", "/{id}"},
}

func resourceRoutes(e r.Entry, api bool, tail string) []r.Entry {
	names := func(call string) map[string]bool {
		m := r.Rx(`^(?:\s*->\s*\w+\s*\([^()]*\))*?\s*->\s*` + call + `\s*\(([^()]*)\)`).FindStringSubmatch(tail)
		if m == nil {
			return nil
		}
		set := map[string]bool{}
		for _, n := range regexp.MustCompile(`['"](\w+)['"]`).FindAllStringSubmatch(m[1], -1) {
			set[n[1]] = true
		}
		return set
	}
	only, except := names("only"), names("except")
	// photos.comments is /photos/{photo}/comments/{comment}.
	segs := strings.Split(strings.Trim(e.Path, "/"), ".")
	base, param := "", ""
	for i, seg := range segs {
		param = singular(path.Base(seg))
		if i < len(segs)-1 {
			base += "/" + seg + "/{" + param + "}"
		} else {
			base += "/" + seg
		}
	}
	var out []r.Entry
	for _, a := range resourceActions {
		if api && (a.name == "create" || a.name == "edit") || only != nil && !only[a.name] || except[a.name] {
			continue
		}
		x := e
		x.Method = a.method
		x.Path = r.JoinRoute("", base+strings.ReplaceAll(a.path, "{id}", "{"+param+"}"))
		if e.Handler != "" {
			x.Handler = e.Handler + "." + a.name
		}
		out = append(out, x)
	}
	return out
}

func singular(w string) string {
	switch {
	case strings.HasSuffix(w, "ies") && len(w) > 3:
		return w[:len(w)-3] + "y"
	case strings.HasSuffix(w, "sses"), strings.HasSuffix(w, "xes"), strings.HasSuffix(w, "ches"), strings.HasSuffix(w, "shes"):
		return w[:len(w)-2]
	case strings.HasSuffix(w, "s") && !strings.HasSuffix(w, "ss"):
		return w[:len(w)-1]
	}
	return w
}

// Symfony routes written in YAML: a name, then an indented block with a path,
// the methods and the controller. A block with a resource and a prefix
// imports another routing file under that prefix; the prefixes are joined
// when linking, once every routing file has been read.

var (
	yamlTopKey   = regexp.MustCompile(`^([A-Za-z_][\w.\-]*):\s*$`)
	yamlField    = regexp.MustCompile(`^\s+(_?[a-z_]+):\s*(.*?)\s*$`)
	routingFiles = regexp.MustCompile(`(?i)(^|/)(config/routes?|routing)(/|\.ya?ml$|_[\w-]*\.ya?ml$)|(^|/)routes?\.ya?ml$|(^|/)routing[\w-]*\.ya?ml$`)
)

func isRoutingFile(p string) bool { return routingFiles.MatchString(p) }

// yamlRoutes reads a routing file's routes and the files it imports.
func yamlRoutes(s *r.Source) ([]r.Entry, []r.RouteImport) {
	if !isRoutingFile(s.Path) {
		return nil, nil
	}
	var out []r.Entry
	var imports []r.RouteImport
	type block struct {
		line   int
		fields map[string]string
	}
	var blocks []block
	var cur *block
	for line := 1; line <= len(s.Lines); line++ {
		text := s.LineText(line)
		if m := yamlTopKey.FindStringSubmatch(text); m != nil {
			blocks = append(blocks, block{line: line, fields: map[string]string{}})
			cur = &blocks[len(blocks)-1]
			continue
		}
		if cur == nil {
			continue
		}
		if m := yamlField.FindStringSubmatch(text); m != nil {
			key := m[1]
			if _, seen := cur.fields[key]; !seen {
				cur.fields[key] = strings.Trim(m[2], `'"`)
			}
		}
	}
	for _, b := range blocks {
		if res := b.fields["resource"]; res != "" {
			imports = append(imports, r.RouteImport{Resource: res, Prefix: b.fields["prefix"]})
			continue
		}
		p, ok := b.fields["path"]
		if !ok {
			continue
		}
		e := r.Entry{Kind: r.KindHTTP, Method: "ANY", Path: r.JoinRoute("", p), Framework: "symfony", Mountable: true, Line: b.line, File: s.Path}
		if m := strings.Trim(b.fields["methods"], "[] "); m != "" {
			e.Method = strings.ToUpper(strings.Join(strings.Fields(strings.NewReplacer(",", " ", "'", " ", `"`, " ", "[", " ", "]", " ").Replace(m)), ","))
		}
		ctrl := b.fields["controller"]
		if ctrl == "" {
			ctrl = b.fields["_controller"]
		}
		e.Handler = symfonyController(ctrl)
		out = append(out, e)
	}
	return out, imports
}

// symfonyController is a controller as a handler: `App\Controller\X::show`
// is X.show, `X` alone is X, and a service id is kept whole, for the linker
// to look up among the service bindings.
func symfonyController(c string) string {
	c = strings.TrimSpace(c)
	if c == "" {
		return ""
	}
	cls, method := c, ""
	if i := strings.Index(c, "::"); i > 0 {
		cls, method = c[:i], c[i+2:]
	} else if i := strings.LastIndex(c, ":"); i > 0 && !strings.Contains(c, `\`) {
		cls, method = c[:i], c[i+1:]
	}
	if strings.Contains(cls, `\`) {
		cls = cls[strings.LastIndexByte(cls, '\\')+1:]
	}
	if method != "" {
		return cls + "." + method
	}
	return cls
}

// importTargets are the routing files a resource names: a path relative to
// the importing file, or a bundle path (`@SyliusAdminBundle/Resources/...`)
// matched by what follows the bundle, preferring the directory the bundle's
// name ends with.
func importTargets(from, resource string, routing []string) []string {
	resource = strings.TrimSpace(resource)
	if !strings.HasPrefix(resource, "@") {
		want := path.Join(path.Dir(from), resource)
		for _, rt := range routing {
			if rt == want {
				return []string{rt}
			}
		}
		return nil
	}
	slash := strings.IndexByte(resource, '/')
	if slash < 0 {
		return nil
	}
	bundle, rest := resource[1:slash], resource[slash:]
	var matches []string
	for _, rt := range routing {
		if strings.HasSuffix(rt, rest) {
			matches = append(matches, rt)
		}
	}
	if len(matches) <= 1 {
		return matches
	}
	for i := 1; i < len(bundle); i++ {
		if bundle[i] >= 'A' && bundle[i] <= 'Z' {
			for _, rt := range matches {
				if strings.Contains(rt, "/"+bundle[i:]+"/") {
					return []string{rt}
				}
			}
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Data and wiring.
// ---------------------------------------------------------------------------

var (
	phpTable        = regexp.MustCompile(`protected\s+\$table\s*=\s*["']([\w.]+)["']`)
	eloquent        = regexp.MustCompile(`\b([A-Z]\w*)::(where\w*|find\w*|all|create|update\w*|destroy|query|first\w*|insert|upsert|firstOrCreate|updateOrCreate|with|select|count|paginate|get)\s*\(`)
	doctrineXML     = regexp.MustCompile(`<(entity|mapped-superclass|document)\b[^>]*?\bname\s*=\s*"([\w\\]+)"[^>]*>`)
	xmlTable        = regexp.MustCompile(`\b(?:table|collection)\s*=\s*"([\w.]+)"`)
	laravelBind     = regexp.MustCompile(`->(?:bind|singleton|scoped|bindIf|singletonIf)\s*\(\s*\\?([A-Z][\w\\]*)::class\s*,\s*\\?([A-Z][\w\\]*)::class`)
	symfonySvc      = regexp.MustCompile(`<service\b[^>]*>`)
	symfonyAlias    = regexp.MustCompile(`<service\b[^>]*\balias\s*=\s*"([^"]+)"[^>]*\bid\s*=\s*"([^"]+)"|<service\b[^>]*\bid\s*=\s*"([^"]+)"[^>]*\balias\s*=\s*"([^"]+)"`)
	symfonySet      = regexp.MustCompile(`->set\(\s*['"]([^'"]+)['"]\s*,\s*\\?([A-Z][\w\\]*)::class`)
	symfonyPHPAlias = regexp.MustCompile(`->alias\(\s*\\?([A-Z][\w\\]*)::class\s*,\s*['"]([^'"]+)['"]`)
	xmlAttr         = regexp.MustCompile(`\b(id|name|class)\s*=\s*"([^"]*)"`)
	phpTyped        = regexp.MustCompile(`\??\\?([A-Z][\w\\]*)\s+\$(\w+)`)
)

// entities are Doctrine's mapped classes and Eloquent's $table.
func entities(s *r.Source) []r.Entity {
	var out []r.Entity
	for _, d := range s.Decls() {
		if !d.Class {
			continue
		}
		e := r.Entity{Class: d.Name, Name: d.Name, File: s.Path, Line: d.Line}
		switch {
		case d.Has("Entity") != nil:
			e.Store, e.Framework = "sql", "doctrine"
		case d.Has("Document") != nil:
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
	for _, m := range phpTable.FindAllStringSubmatchIndex(s.Code, -1) {
		line := s.Line(m[0])
		out = append(out, r.Entity{Class: s.TypeAt(line), Name: s.TypeAt(line), Table: s.Code[m[2]:m[3]], Store: "sql", Framework: "eloquent", File: s.Path, Line: line})
	}
	return out
}

// xmlEntities are the classes a Doctrine mapping file maps.
func xmlEntities(s *r.Source) []r.Entity {
	var out []r.Entity
	for _, loc := range doctrineXML.FindAllStringSubmatchIndex(s.Code, -1) {
		if !strings.Contains(s.Code, "doctrine") {
			break
		}
		tag := s.Code[loc[0]:loc[1]]
		e := r.Entity{Class: s.Code[loc[4]:loc[5]], Store: "sql", Framework: "doctrine-xml", File: s.Path, Line: s.Line(loc[0])}
		if s.Code[loc[2]:loc[3]] == "document" {
			e.Store = "document"
		}
		if m := xmlTable.FindStringSubmatch(tag); m != nil {
			e.Table = m[1]
		}
		e.Name = e.Class[strings.LastIndexByte(e.Class, '\\')+1:]
		out = append(out, e)
	}
	return out
}

func accesses(s *r.Source) []r.Access {
	var out []r.Access
	for _, m := range eloquent.FindAllStringSubmatchIndex(s.Code, -1) {
		op := s.Code[m[4]:m[5]]
		kind := r.AccessRead
		if strings.HasPrefix(op, "update") || op == "create" || op == "destroy" || op == "insert" || op == "upsert" || op == "firstOrCreate" || op == "updateOrCreate" {
			kind = r.AccessWrite
		}
		out = append(out, r.Access{Target: s.Code[m[2]:m[3]], Access: kind, Via: "orm", File: s.Path, Line: s.Line(m[0])})
	}
	return out
}

// xmlBindings are the services and aliases a Symfony XML container
// declares.
func xmlBindings(s *r.Source) []r.Binding {
	if !strings.Contains(s.Code, "symfony.com/schema/dic/services") {
		return nil
	}
	var out []r.Binding
	for _, loc := range symfonySvc.FindAllStringIndex(s.Code, -1) {
		attrs := map[string]string{}
		for _, a := range xmlAttr.FindAllStringSubmatch(s.Code[loc[0]:loc[1]], -1) {
			attrs[a[1]] = a[2]
		}
		if attrs["id"] != "" && attrs["class"] != "" {
			out = append(out, r.Bind(s, attrs["id"], attrs["class"], "symfony_xml", loc[0]))
		}
	}
	for _, m := range symfonyAlias.FindAllStringSubmatchIndex(s.Code, -1) {
		if m[2] >= 0 {
			out = append(out, r.Bind(s, s.Code[m[4]:m[5]], s.Code[m[2]:m[3]], "symfony_alias", m[0]))
		} else {
			out = append(out, r.Bind(s, s.Code[m[6]:m[7]], s.Code[m[8]:m[9]], "symfony_alias", m[0]))
		}
	}
	return out
}

// ---------------------------------------------------------------------------
// Symbols and outbound calls.
// ---------------------------------------------------------------------------

var laravelGroup = regexp.MustCompile(`Route::middleware\s*\(\s*([^()]*)\)\s*(?:->\s*\w+\s*\([^()]*\)\s*)*->\s*group\s*\(`)

// security reads the lines a Laravel middleware group covers.
func security(s *r.Source, sy *r.Symbols) {
	code := s.Code
	if !strings.Contains(code, "Route::middleware") {
		return
	}
	for _, loc := range laravelGroup.FindAllStringSubmatchIndex(code, -1) {
		mw := strings.Join(strings.Fields(code[loc[2]:loc[3]]), " ")
		if !r.Authish.MatchString(mw) {
			continue
		}
		_, end := r.Balanced(code, loc[1]-1)
		sy.AuthRanges = append(sy.AuthRanges, r.AuthRange{From: s.Line(loc[0]), To: s.Line(end), Rule: "middleware(" + mw + ")"})
	}
}

// clientBases are Guzzle clients made with a base_uri and Symfony's
// HttpClient::createForBaseUri.
func clientBases(s *r.Source, sy *r.Symbols) []r.ClientBase {
	code := s.Code
	if !r.Rx(`GuzzleHttp|base_uri|createForBaseUri`).MatchString(code) {
		return nil
	}
	var out []r.ClientBase
	add := func(name, expr string, at int) {
		if name == "" || strings.TrimSpace(expr) == "" {
			return
		}
		out = append(out, r.ClientBase{Name: name, Addr: s.RenderAddr(expr, sy), Resilience: r.ResilienceIn(s.Statement(at)), Line: s.Line(at)})
	}
	for _, m := range r.Rx(`\$(?:this->)?(\w+)\s*=\s*new\s+\\?(?:GuzzleHttp\\)?Client\s*\(\s*\[`).FindAllStringSubmatchIndex(code, -1) {
		inner, _ := r.Balanced(code, m[1]-1)
		add(code[m[2]:m[3]], r.ObjectKey(inner, "base_uri"), m[0])
	}
	for _, m := range r.Rx(`\$(?:this->)?(\w+)\s*=\s*\\?(?:[\w\\]*\\)?HttpClient::createForBaseUri\s*\(`).FindAllStringSubmatchIndex(code, -1) {
		inner, _ := r.Balanced(code, m[1]-1)
		if args := r.SplitArgs(inner); len(args) > 0 {
			add(code[m[2]:m[3]], args[0], m[0])
		}
	}
	return out
}

var (
	laravelHTTP = regexp.MustCompile(`\bHttp::`)
	phpCurl     = regexp.MustCompile(`\bcurl_init\s*\(|CURLOPT_URL\s*,`)
)

// calls are Laravel's Http facade and curl.
func calls(s *r.Source, sy *r.Symbols, add func(r.OutCall, int)) {
	code := s.Code
	for _, loc := range laravelHTTP.FindAllStringIndex(code, -1) {
		stmt := s.StatementFrom(loc[0])
		mm := r.Rx(`(?:::|->)\s*(get|post|put|patch|delete|head|send)\s*\(`).FindStringSubmatchIndex(stmt)
		if mm == nil {
			continue
		}
		inner, _ := r.Balanced(stmt, mm[1]-1)
		args := r.SplitArgs(inner)
		method := strings.ToUpper(stmt[mm[2]:mm[3]])
		if method == "SEND" && len(args) > 1 {
			method = r.MethodLiteral(args[0])
			args = args[1:]
		}
		if len(args) == 0 {
			continue
		}
		addr := s.RenderAddr(args[0], sy)
		if b := r.Rx(`baseUrl\s*\(`).FindStringIndex(stmt); b != nil {
			bi, _ := r.Balanced(stmt, b[1]-1)
			addr = strings.TrimRight(s.RenderAddr(bi, sy), "/") + "/" + strings.TrimLeft(addr, "/")
		}
		add(r.OutCall{Kind: "http", Client: "laravel-http", Method: method, Addr: addr}, loc[0])
	}
	for _, loc := range phpCurl.FindAllStringIndex(code, -1) {
		start := loc[1]
		if code[loc[1]-1] == '(' {
			start = loc[1] - 1
		}
		var arg string
		if code[loc[1]-1] == '(' {
			inner, _ := r.Balanced(code, start)
			if args := r.SplitArgs(inner); len(args) > 0 {
				arg = args[0]
			}
		} else {
			end := strings.IndexAny(code[start:], ");")
			if end > 0 {
				arg = code[start : start+end]
			}
		}
		if strings.TrimSpace(arg) == "" {
			continue
		}
		add(r.OutCall{Kind: "http", Client: "curl", Method: "ANY", Addr: s.RenderAddr(arg, sy)}, loc[0])
	}
}

var outRules = []r.OutRule{
	// Messaging.
	{Re: regexp.MustCompile(`\$[\w>-]*[bB]us\s*->\s*dispatch\s*\(\s*new\s+\\?([\w\\]+)`), Kind: "message", Client: "symfony-messenger", Arg: -1},
	{Re: regexp.MustCompile(`\bbasic_publish\s*\(`), Kind: "message", Client: "amqp", Arg: 2},

	// Mail, LDAP, SOAP.
	{Re: regexp.MustCompile(`\bMail::(?:to|send|queue|raw)\s*\(`), Kind: "mail", Client: "laravel-mail", Arg: -1},
	{Re: regexp.MustCompile(`\bldap_connect\s*\(`), Kind: "ldap", Client: "php-ldap", Arg: 0},
	{Re: regexp.MustCompile(`\bnew\s+\\?SoapClient\s*\(`), Kind: "soap", Client: "php-soap", Arg: 0},
}
