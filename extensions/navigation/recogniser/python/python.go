// Package python reads Python for navigation: Flask, FastAPI, Sanic and
// Django routes, Celery, Click and APScheduler, Django's and SQLAlchemy's
// models, Django REST framework's permissions, Django settings, and Python's
// HTTP, messaging and data clients.
package python

import (
	"regexp"
	"strings"

	r "github.com/archstats/archstats/extensions/navigation/recogniser"
)

const python = "python"

var (
	pyDef   = regexp.MustCompile(`^\s*(?:async\s+)?def\s+([A-Za-z_]\w*)`)
	pyClass = regexp.MustCompile(`^\s*class\s+([A-Za-z_]\w*)`)
)

// Plugin is Python.
func Plugin() r.Plugin {
	return r.Plugin{
		Name: "python",
		Languages: []*r.Language{{
			Name: python, Extensions: []string{".py"}, Kind: r.Code,
			Syntax: r.Syntax{
				LineComments: []string{"#"},
				Strings:      true,
				TripleQuotes: true,
				Marks:        r.AtMarks,
				Classify: func(t string) (bool, string) {
					if m := pyClass.FindStringSubmatch(t); m != nil {
						return true, m[1]
					}
					if m := pyDef.FindStringSubmatch(t); m != nil {
						return false, m[1]
					}
					return false, ""
				},
				TypeName: func(line string) string {
					if m := pyClass.FindStringSubmatch(line); m != nil {
						return m[1]
					}
					return ""
				},
				BraceInterpolation: true,
				Statements:         r.NewlineStatements,
			},
		}},
		Recognisers: []*r.Recogniser{{
			Languages:     []string{python},
			MemberEntries: memberEntries,
			CallEntries: func(s *r.Source) []r.Entry {
				if strings.HasSuffix(s.Path, "urls.py") || strings.Contains(s.Code, "urlpatterns") {
					return djangoURLs(s)
				}
				return nil
			},
			MainEntries: mainEntries,
			AuthMarks:   []string{"login_required", "permission_required", "user_passes_test", "staff_member_required", "superuser_required", "jwt_required", "auth_required", "requires_auth"},
			DocMarks:    []string{"extend_schema", "swagger_auto_schema"},
			Entities:    entities,
			Accesses:    accesses,
			ModelBase:   modelBase,
			Declared: func(s *r.Source, sy *r.Symbols) {
				r.ColonTypes(s, sy)
				for _, m := range withAs.FindAllStringSubmatch(s.Code, -1) {
					sy.SetVar(m[2], m[1])
				}
			},
			Modules:     r.ModuleConstructed,
			Constants:   r.AssignedConstants,
			ClassAuth:   classAuth,
			ClientBases: clientBases,
			Calls:       calls,
			ClientName: func(s *r.Source, lt, recv string) string {
				if lt == "session" {
					return "requests"
				}
				return ""
			},
			ClientModules: []string{"requests", "httpx"},
			Resilience:    map[string]string{"retry": "retry", "backoff": "retry", "on_exception": "retry", "circuit": "circuit_breaker"},
			OutRules:      outRules,
			CallRules:     callRules,
			ConfigFormat: func(p string) string {
				base := strings.ToLower(p[strings.LastIndexByte(p, '/')+1:])
				if strings.HasPrefix(base, "settings") || strings.Contains(p, "/settings/") {
					return "django"
				}
				return ""
			},
			ReadConfig: func(_, _, text string, add func(key, value string, line int, env string)) {
				r.ReadNested(text, ":", "", add)
				for _, m := range pyAssign.FindAllStringSubmatchIndex(text, -1) {
					add(text[m[2]:m[3]], strings.TrimSpace(text[m[4]:m[5]]), strings.Count(text[:m[0]], "\n")+1, "")
				}
			},
		}},
	}
}

var pyAssign = regexp.MustCompile(`(?m)^([A-Z][A-Z0-9_]*)\s*=\s*(.+)$`)

// ---------------------------------------------------------------------------
// Entry points.
// ---------------------------------------------------------------------------

var httpVerbs = map[string]bool{"GET": true, "POST": true, "PUT": true, "DELETE": true, "PATCH": true, "HEAD": true, "OPTIONS": true}

// memberEntries are web decorators (@app.route, @bp.get, @router.post), and
// Django's signals, Celery's tasks, APScheduler's jobs and Click's commands.
func memberEntries(s *r.Source, d r.Decl, class r.ClassContext, e r.Entry) []r.Entry {
	var out []r.Entry
	for _, m := range d.Marks {
		if !strings.Contains(m.Full, ".") {
			continue
		}
		verb := strings.ToUpper(m.Name)
		p := r.FirstString(r.Positional(m.Args))
		if (m.Name == "route" || m.Name == "api_route" || m.Name == "websocket" || httpVerbs[verb]) && (p == "" && len(m.Args) == 0 || strings.HasPrefix(p, "/")) {
			x := e
			x.Kind, x.Line, x.Path, x.Framework = r.KindHTTP, m.Line, p, web(s)
			switch {
			case httpVerbs[verb]:
				x.Method = verb
			case m.Name == "websocket":
				x.Method = "WS"
			default:
				x.Method = strings.ToUpper(r.Named(m.Args, "methods"))
				if x.Method == "" {
					x.Method = "GET"
				}
			}
			out = append(out, x)
		}
	}
	for _, m := range d.Marks {
		x := e
		x.Line = m.Line
		switch {
		case m.Name == "receiver":
			x.Kind, x.Framework = r.KindEvent, "django-signals"
			if p := r.Positional(m.Args); len(p) > 0 {
				x.Path = p[0]
			}
		case m.Name == "shared_task" || m.Name == "task" && strings.Contains(m.Full, "."), m.Name == "actor" && strings.Contains(m.Full, "dramatiq"):
			x.Kind, x.Framework, x.Path = r.KindJob, "celery", m.Arg("name")
		case m.Name == "periodic_task":
			x.Kind, x.Framework = r.KindSchedule, "celery"
		case m.Name == "scheduled_job":
			x.Kind, x.Framework = r.KindSchedule, "apscheduler"
			x.Path = strings.Join(r.Positional(m.Args), " ")
		case m.Name == "command" || m.Name == "group" && strings.Contains(m.Full, "click"):
			x.Kind, x.Framework = r.KindCLI, "click"
			x.Path = m.Arg("name")
			if x.Path == "" {
				x.Path = strings.ReplaceAll(d.Name, "_", "-")
			}
		default:
			continue
		}
		out = append(out, x)
	}
	return out
}

func web(s *r.Source) string {
	switch {
	case strings.Contains(s.Raw, "fastapi"):
		return "fastapi"
	case strings.Contains(s.Raw, "flask"):
		return "flask"
	case strings.Contains(s.Raw, "sanic"):
		return "sanic"
	}
	return "python-web"
}

var (
	pyURL         = regexp.MustCompile(`\b(path|re_path|url)\s*\(`)
	getClassAlias = regexp.MustCompile(`\b([a-z_]\w*)\s*=\s*get_class\(\s*["'][\w.]+["']\s*,\s*["'](\w+)["']`)
)

func djangoURLs(s *r.Source) []r.Entry {
	// django-oscar's apps hold their views as attributes loaded by name
	// (`list_view = get_class("dashboard.views", "ListView")`) so a project
	// can override them; the class named is the handler.
	aliases := map[string]string{}
	for _, m := range getClassAlias.FindAllStringSubmatch(s.Code, -1) {
		aliases[m[1]] = m[2]
	}
	var out []r.Entry
	for _, loc := range pyURL.FindAllStringSubmatchIndex(s.Code, -1) {
		if loc[0] > 0 && (s.Code[loc[0]-1] == '.' || r.IsWordByte(s.Code[loc[0]-1])) {
			continue
		}
		inner, _ := r.Balanced(s.Code, loc[1]-1)
		args := r.SplitArgs(inner)
		if len(args) < 2 {
			continue
		}
		p := r.Unquote(args[0])
		if p == "" && !r.IsEmptyString(args[0]) {
			continue
		}
		view := args[1]
		if strings.HasPrefix(view, "include(") || strings.HasSuffix(view, ".urls") || strings.Contains(view, ".urls[") {
			continue
		}
		p = strings.TrimSuffix(strings.TrimPrefix(p, "^"), "$")
		auth := ""
		// login_required(OrderView.as_view()): the wrapper is the access rule.
		for {
			w := r.Rx(`^\s*([A-Za-z_][\w.]*)\s*\(`).FindStringSubmatchIndex(view)
			if w == nil || !r.Authish.MatchString(view[w[2]:w[3]]) {
				break
			}
			auth = strings.TrimPrefix(auth+", "+r.LastName(view[w[2]:w[3]]), ", ")
			inner, _ := r.Balanced(view, w[1]-1)
			args := r.SplitArgs(inner)
			if len(args) == 0 {
				break
			}
			view = args[0]
		}
		handler := r.LastName(view)
		if cls, ok := aliases[handler]; ok {
			handler = cls
		}
		out = append(out, r.Entry{Kind: r.KindHTTP, Method: "ANY", Path: r.JoinRoute("", p), Framework: "django", Handler: handler, Line: s.Line(loc[0]), Auth: auth})
	}
	return out
}

var pyMain = regexp.MustCompile(`(?m)^if\s+__name__\s*==\s*['"]__main__['"]\s*:`)

// mainEntries are a Django management command, or a __main__ guard.
func mainEntries(s *r.Source) []r.Entry {
	if m := regexp.MustCompile(`(^|/)management/commands/([a-z_][a-z0-9_]*)\.py$`).FindStringSubmatch(s.Path); m != nil && m[2] != "__init__" && strings.Contains(s.Code, "class Command") {
		line := strings.Count(s.Code[:strings.Index(s.Code, "class Command")], "\n") + 1
		return []r.Entry{{Kind: r.KindCLI, Framework: "django-admin", Path: m[2], Class: "Command", Line: line, DeclLine: line}}
	}
	if loc := pyMain.FindStringIndex(s.Code); loc != nil {
		return []r.Entry{r.MainEntry(s, loc[0], "")}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Data.
// ---------------------------------------------------------------------------

var (
	tableName    = regexp.MustCompile(`__tablename__\s*=\s*["']([\w.]+)["']`)
	dbTable      = regexp.MustCompile(`\bdb_table\s*=\s*["']([\w.]+)["']`)
	djangoORM    = regexp.MustCompile(`\b([A-Z]\w*)\.(?:objects|_default_manager|all_objects)\.(\w+)`)
	djangoWrites = map[string]bool{"create": true, "update": true, "delete": true, "bulk_create": true, "bulk_update": true, "get_or_create": true, "update_or_create": true, "save": true}
)

// entities are SQLAlchemy's __tablename__ and Django's Meta.db_table.
func entities(s *r.Source) []r.Entity {
	var out []r.Entity
	types := s.TypeDecls()
	for _, m := range tableName.FindAllStringSubmatchIndex(s.Code, -1) {
		line := s.Line(m[0])
		out = append(out, r.Entity{Class: s.TypeAt(line), Name: s.TypeAt(line), Table: s.Code[m[2]:m[3]], Store: "sql", Framework: "sqlalchemy", File: s.Path, Line: line})
	}
	for _, m := range dbTable.FindAllStringSubmatchIndex(s.Code, -1) {
		line := s.Line(m[0])
		// Meta is the class the option sits in; the model is the one
		// around it, the last class before Meta.
		name := ""
		for _, t := range types {
			if t.Line <= line && t.Name != "Meta" {
				name = t.Name
			}
		}
		out = append(out, r.Entity{Class: name, Name: name, Table: s.Code[m[2]:m[3]], Store: "sql", Framework: "django", File: s.Path, Line: line})
	}
	return out
}

func accesses(s *r.Source) []r.Access {
	var out []r.Access
	for _, m := range djangoORM.FindAllStringSubmatchIndex(s.Code, -1) {
		kind := r.AccessRead
		if djangoWrites[s.Code[m[4]:m[5]]] {
			kind = r.AccessWrite
		}
		out = append(out, r.Access{Target: s.Code[m[2]:m[3]], Access: kind, Via: "orm", File: s.Path, Line: s.Line(m[0])})
	}
	return out
}

// Supertypes that make a class a mapped model, for ORMs that say so by
// inheritance rather than by annotation. A bare Model is Django's.
var modelBases = map[string]string{
	"models.Model": "django", "Model": "django", "db.Model": "flask-sqlalchemy", "DeclarativeBase": "sqlalchemy",
	"Document": "mongoengine", "SQLModel": "sqlmodel",
}

func modelBase(key string, imports func() []string) (string, bool) {
	fw, ok := modelBases[key]
	if !ok {
		return "", false
	}
	if fw == "django" && !strings.Contains(strings.Join(imports(), " "), "django") && key != "models.Model" {
		return "", true
	}
	return fw, true
}

// ---------------------------------------------------------------------------
// Symbols and outbound calls.
// ---------------------------------------------------------------------------

var (
	// with X(...) as name / async with aiohttp.ClientSession() as session.
	withAs      = regexp.MustCompile(`\bwith\s+([\w.]+)\s*\([^()]*(?:\([^()]*\)[^()]*)*\)\s+as\s+(\w+)`)
	pyClassHead = regexp.MustCompile(`(?m)^class\s+(\w+)\s*(?:\([^)]*\))?\s*:`)
	drfAccess   = regexp.MustCompile(`(?m)^\s+(permission_classes)\s*=\s*[\[(]([^\])]*)[\])]`)
)

// classAuth is what each class says about access: Django REST framework's
// permission_classes, which apply to every route the view answers.
func classAuth(s *r.Source) map[string]string {
	if !strings.Contains(s.Code, "permission_classes") {
		return nil
	}
	heads := pyClassHead.FindAllStringSubmatchIndex(s.Code, -1)
	out := map[string]string{}
	for i, h := range heads {
		end := len(s.Code)
		if i+1 < len(heads) {
			end = heads[i+1][0]
		}
		for _, m := range drfAccess.FindAllStringSubmatch(s.Code[h[1]:end], -1) {
			out[s.Code[h[2]:h[3]]] = "permission_classes(" + strings.Join(strings.Fields(strings.Trim(m[2], " ,")), " ") + ")"
		}
	}
	return out
}

// clientBases are httpx clients made with a base_url; requests sessions
// don't carry one.
func clientBases(s *r.Source, sy *r.Symbols) []r.ClientBase {
	code := s.Code
	if !strings.Contains(code, "httpx") {
		return nil
	}
	var out []r.ClientBase
	for _, m := range r.Rx(`\b([a-z_]\w*)\s*=\s*httpx\s*\.\s*(?:Async)?Client\s*\(`).FindAllStringSubmatchIndex(code, -1) {
		inner, _ := r.Balanced(code, m[1]-1)
		name, expr := code[m[2]:m[3]], r.ObjectKey(inner, "base_url")
		if name == "" || strings.TrimSpace(expr) == "" {
			continue
		}
		out = append(out, r.ClientBase{Name: name, Addr: s.RenderAddr(expr, sy), Resilience: r.ResilienceIn(s.Statement(m[0])), Line: s.Line(m[0])})
	}
	return out
}

var (
	pyURLOpen  = regexp.MustCompile(`\b(?:urlopen|urllib\.request\.Request|urllib2\.Request|Request)\s*\(`)
	celeryCall = regexp.MustCompile(`\b([a-z_]\w*)\s*\.\s*(delay|apply_async)\s*\(`)
)

// calls are urllib's requests and Celery tasks sent.
func calls(s *r.Source, sy *r.Symbols, add func(r.OutCall, int)) {
	code := s.Code
	for _, loc := range pyURLOpen.FindAllStringIndex(code, -1) {
		if loc[0] > 0 && (code[loc[0]-1] == '.' && !strings.HasSuffix(code[max(0, loc[0]-20):loc[0]], "urllib.request.")) {
			continue
		}
		inner, _ := r.Balanced(code, loc[1]-1)
		args := r.SplitArgs(inner)
		if len(args) == 0 {
			continue
		}
		rendered := s.RenderAddr(args[0], sy)
		if !r.LooksLikeAddr(rendered, args[0]) {
			continue
		}
		add(r.OutCall{Kind: "http", Client: "urllib", Method: "GET", Addr: rendered}, loc[0])
	}
	for _, m := range celeryCall.FindAllStringSubmatchIndex(code, -1) {
		name := code[m[2]:m[3]]
		if r.NotCalls[name] {
			continue
		}
		add(r.OutCall{Kind: "message", Client: "celery", Method: "TASK", Target: name, Addr: name}, m[0])
	}
}

var callRules = []r.CallRule{
	{Members: regexp.MustCompile(`^(?:send|produce|send_and_wait)$`), Kind: "message", Client: "kafka", Arg: 0, Key: "topic", Gate: `(?i)produc|kafka`},
	{Members: regexp.MustCompile(`^(?:connect)$`), Recv: regexp.MustCompile(`^(?:[\w.]*(?:ssh|sftp|client)\w*)$`), Kind: "file_transfer", Client: "paramiko", Arg: 0, Key: "hostname", Gate: `(?i)ssh|sftp|paramiko`},
}

var outRules = []r.OutRule{
	// Messaging.
	{Re: regexp.MustCompile(`\bbasic_publish\s*\(`), Kind: "message", Client: "amqp", Arg: 1, Key: "routing_key"},
	{Re: regexp.MustCompile(`\.\s*send_message\s*\(`), Kind: "message", Client: "sqs", Arg: -1, Key: "QueueUrl"},
	{Re: regexp.MustCompile(`\.\s*publish\s*\(\s*TopicArn`), Kind: "message", Client: "sns", Arg: -1, Key: "TopicArn"},

	// Databases, caches, search, mail, LDAP, storage, file transfer.
	{Re: regexp.MustCompile(`\b(?:create_engine|create_async_engine|psycopg2\.connect|psycopg\.connect|pymysql\.connect|asyncpg\.connect|asyncpg\.create_pool)\s*\(`), Kind: "sql", Client: "python-db", Arg: 0, Key: "dsn"},
	{Re: regexp.MustCompile(`\b(?:MongoClient|AsyncIOMotorClient|motor\.motor_asyncio\.AsyncIOMotorClient|pymongo\.MongoClient)\s*\(`), Kind: "nosql", Client: "mongodb", Arg: 0, Key: "host"},
	{Re: regexp.MustCompile(`\bredis\s*\.\s*(?:Redis|StrictRedis|from_url|Redis\s*\.\s*from_url|asyncio\s*\.\s*Redis)\s*\(`), Kind: "cache", Client: "redis-py", Arg: 0, Key: "host"},
	{Re: regexp.MustCompile(`\bElasticsearch\s*\(`), Kind: "search", Client: "elasticsearch", Arg: 0, Key: "hosts"},
	{Re: regexp.MustCompile(`\bsmtplib\s*\.\s*SMTP(?:_SSL)?\s*\(`), Kind: "mail", Client: "smtp", Arg: 0, Key: "host"},
	{Re: regexp.MustCompile(`\bldap3\s*\.\s*Server\s*\(|\bServer\s*\(\s*['"]ldaps?://`), Kind: "ldap", Client: "ldap3", Arg: 0},
	{Re: regexp.MustCompile(`\bboto3\s*\.\s*(?:client|resource)\s*\(\s*['"]s3['"]`), Kind: "storage", Client: "s3", Arg: -1},
	{Re: regexp.MustCompile(`\b(?:BlobServiceClient(?:\s*\.\s*from_connection_string)?|storage\s*\.\s*Client|Minio)\s*\(`), Kind: "storage", Client: "object-storage", Arg: 0},
	{Re: regexp.MustCompile(`\bftplib\s*\.\s*FTP(?:_TLS)?\s*\(|\bFTP(?:_TLS)?\s*\(`), Kind: "file_transfer", Client: "ftp", Arg: 0, Key: "host"},

	// gRPC, SOAP.
	{Re: regexp.MustCompile(`\bgrpc\s*\.\s*(?:aio\s*\.\s*)?(?:insecure|secure)_channel\s*\(`), Kind: "grpc", Client: "grpcio", Arg: 0},
	{Re: regexp.MustCompile(`\b\w+_pb2_grpc\s*\.\s*(\w+)Stub\s*\(`), Kind: "grpc", Client: "grpcio", Arg: -1},
	{Re: regexp.MustCompile(`\bzeep\s*\.\s*(?:Client|AsyncClient)\s*\(|\bClient\s*\(\s*(?:wsdl\s*=\s*)?['"][^'"]+\?wsdl`), Kind: "soap", Client: "zeep", Arg: 0, Key: "wsdl"},
}
