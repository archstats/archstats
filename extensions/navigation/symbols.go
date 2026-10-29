package navigation

import (
	"regexp"
	"sort"
	"strings"
)

// What a route reaches is read one call at a time: `orders.place(cart)` in a
// handler is a call into whatever type `orders` was declared as. A file's
// symbols are what it takes to follow such a call without a compiler: the
// declared type of every field, property, parameter and constructed
// variable, the string constants, which fields hold configuration, and every
// call site with the line it is on. Scope is the file, not the block: two
// variables of one name in one file with different types are rare enough to
// accept the first.

type callSite struct {
	Recv string // as written, `this.`/`self.`/`$this->` dropped: "orders", "OrderService", "h.svc", ""
	Name string
	Line int32
	// At is where the call is written, Open its opening parenthesis.
	At, Open int32
}

type symbols struct {
	// vars is a name's declared type, as written (generics kept).
	vars map[string]string
	// consts is a string constant's value.
	consts map[string]string
	// keys is the configuration key a field or parameter is bound to:
	// `@Value("${partner.url}") String partnerUrl`.
	keys map[string]string
	// prefixes are the @ConfigurationProperties prefixes of the types in the
	// file, by type name.
	prefixes map[string]string
	// addrs is what a field or variable was assigned when that is an
	// address: `this.baseUrl = process.env.PARTNER_URL`.
	addrs map[string]string
	calls []callSite
	// auth rules: path patterns with the access a security configuration
	// gives them, and line ranges a middleware group covers.
	authRules  []authRule
	authRanges []authRange
	// appPaths are the paths a JAX-RS application is mounted under:
	// @ApplicationPath("/api").
	appPaths []string
}

type authRule struct {
	Pattern, Method, Rule string
	Line                  int
}

type authRange struct {
	From, To int
	Rule     string
}

var (
	// Type name: `OrderService orders`, `IRepository<Order> _repo`.
	typedName = regexp.MustCompile(`\b([A-Z][\w]*(?:\.[A-Z]\w*)*(?:\s*<[^;=(){}<>]*(?:<[^;=(){}<>]*>[^;=(){}<>]*)*>)?(?:\[\])?\??)\s+(_?[a-z][\w$]*)\s*(?:[;=,)]|$|:\s*[A-Z])`)
	// C# property: `public IOrderService Orders { get; }`.
	csProperty = regexp.MustCompile(`\b([A-Z]\w*(?:<[^;=(){}]*?>)?)\s+([A-Z]\w*)\s*\{\s*(?:get|init|private get)`)
	// name: Type, as Kotlin, TypeScript, Swift, Scala and Python hints write it.
	nameColonType = regexp.MustCompile(`(?:^|[\s(,{])(?:(?:private|protected|public|readonly|override|internal|val|var|let|const|lateinit)\s+)*([a-z_$][\w$]*)\s*[?!]?\s*:\s*([A-Z][\w.$]*(?:\s*<[^;=(){}<>]*(?:<[^;=(){}<>]*>[^;=(){}<>]*)*>)?)`)
	// PHP: `private OrderService $orders`, `OrderService $orders` in a signature.
	phpTyped = regexp.MustCompile(`\??\\?([A-Z][\w\\]*)\s+\$(\w+)`)
	// x = new Foo(, x := NewFoo(, x = Foo(, self.x = Foo(, x = &Foo{.
	constructed = regexp.MustCompile(`(?:\b(?:self|this)\s*(?:\.|->)\s*|\$this->|\b)([a-z_$][\w$]*)\s*(?::=|=)\s*(?:await\s+)?(?:new\s+|&)?((?:[a-z_]\w*\.)?(?:New)?[A-Z][\w]*)\s*[({]`)
	// x = module.Client(  (Python, Go): the type keeps its module.
	moduleConstructed = regexp.MustCompile(`\b([a-z_][\w]*)\s*(?::=|=)\s*([a-z_]\w*\.(?:[A-Z]\w*|create|createClient|Session|Client|AsyncClient|New\w*))\s*\(`)
	// with X(...) as name / async with aiohttp.ClientSession() as session.
	withAs = regexp.MustCompile(`\bwith\s+([\w.]+)\s*\([^()]*(?:\([^()]*\)[^()]*)*\)\s+as\s+(\w+)`)
	// self.x = x: a field that holds what a typed parameter was.
	assignedFrom = regexp.MustCompile(`(?:\b(?:self|this)\s*\.\s*|\$this->)(\w+)\s*=\s*\$?(\w+)\s*;?\s*$`)
	// Go receivers and parameters: (h *Handler), (svc orders.Service).
	goParam = regexp.MustCompile(`\b([a-z_]\w*)\s+\*?((?:[a-z_]\w*\.)?[A-Z]\w*)\b`)
	goFunc  = regexp.MustCompile(`(?m)^func\s*(\([^)]*\))?\s*\w*\s*(?:\[[^\]]*\])?\s*\(([^)]*)\)`)
	// String constants.
	javaConst   = regexp.MustCompile(`\b(?:static\s+final|final\s+static|const)\s+(?:String|string|var)?\s*([A-Za-z_]\w*)\s*=\s*("(?:[^"\\]|\\.)*"(?:\s*\+\s*(?:"(?:[^"\\]|\\.)*"|[A-Z_][A-Z0-9_]*))*)\s*;`)
	scriptConst = regexp.MustCompile(`(?m)^\s*(?:export\s+)?(?:const|let|var|val|const val)?\s*([A-Za-z_][\w]*)\s*(?::\s*\w+)?\s*=\s*(["'` + "`" + `][^"'` + "`" + `\n]*["'` + "`" + `])\s*;?\s*$`)
	goConst     = regexp.MustCompile(`\b([A-Za-z_]\w*)\s*(?:string\s*)?=\s*("[^"\n]*")`)
	// Configuration bound to a field or parameter.
	valueKey       = regexp.MustCompile(`@(?:Value|ConfigProperty)\s*\(\s*(?:name\s*=\s*|value\s*=\s*)?"\$?\{?([^}:"]+)(?::[^}"]*)?\}?"[^)]*\)\s*(?:(?:private|protected|public|final|lateinit|val|var|readonly)\s+)*(?:[A-Z][\w.<>]*\s+)?([a-z_]\w*)`)
	optionsSection = regexp.MustCompile(`Configure\s*<\s*(\w+)\s*>\s*\(\s*[\w.]*GetSection\s*\(\s*"([^"]+)"|AddOptions\s*<\s*(\w+)\s*>\s*\(\s*\)\s*\.\s*Bind(?:Configuration)?\s*\(\s*(?:[\w.]*GetSection\s*\(\s*)?"([^"]+)"`)
	configProps    = regexp.MustCompile(`@ConfigurationProperties\s*\(\s*(?:prefix\s*=\s*|value\s*=\s*)?"([^"]+)"`)
	callPattern    = regexp.MustCompile(`([A-Za-z_$][\w$]*(?:\s*(?:\.|->|\?\.|::)\s*[A-Za-z_$][\w$]*)*)\s*(?:\.|->|\?\.|::)\s*([A-Za-z_$][\w$]*)\s*(?:<[^<>()]*>\s*)?\(`)
	barePattern    = regexp.MustCompile(`(?:^|[^\w$.>:@\\])([A-Za-z_][\w]*)\s*\(`)
	notCalls       = map[string]bool{"if": true, "for": true, "foreach": true, "while": true, "switch": true, "catch": true, "return": true, "new": true, "function": true, "typeof": true, "sizeof": true, "super": true, "this": true, "assert": true, "synchronized": true, "using": true, "lock": true, "elif": true, "with": true, "not": true, "and": true, "or": true, "in": true, "await": true, "yield": true, "def": true, "fun": true, "func": true, "class": true, "struct": true, "when": true, "match": true, "case": true, "throw": true, "delete": true, "void": true, "print": true, "len": true, "make": true, "append": true, "isset": true, "empty": true, "array": true, "list": true, "require": true, "import": true, "fn": true, "go": true, "defer": true, "do": true, "else": true, "try": true, "except": true, "finally": true, "raise": true, "lambda": true, "async": true, "get": true, "set": true, "init": true, "where": true, "from": true, "select": true}
	securityMatch  = regexp.MustCompile(`\.(?:requestMatchers|antMatchers|mvcMatchers|regexMatchers|pathMatchers|securityMatcher)\s*\(`)
	securityRule   = regexp.MustCompile(`^\s*\.\s*(permitAll|authenticated|denyAll|anonymous|fullyAuthenticated|rememberMe|hasRole|hasAnyRole|hasAuthority|hasAnyAuthority|access|hasIpAddress)\s*\(([^()]*(?:\([^()]*\)[^()]*)*)\)`)
	anyRequest     = regexp.MustCompile(`\.anyRequest\s*\(\s*\)\s*\.\s*(permitAll|authenticated|denyAll|hasRole|hasAnyRole|hasAuthority|hasAnyAuthority|access)\s*\(([^()]*)\)`)
	kotlinAuthz    = regexp.MustCompile(`\bauthorize\s*\(\s*(?:HttpMethod\.(\w+)\s*,\s*)?("[^"]*"|anyRequest)\s*,\s*(permitAll|authenticated|denyAll|hasRole\s*\([^)]*\)|hasAnyRole\s*\([^)]*\)|hasAuthority\s*\([^)]*\)|hasAnyAuthority\s*\([^)]*\))`)
	jsUseAuth      = regexp.MustCompile(`\b(?:app|router|server)\s*\.\s*use\s*\(\s*(["'][^"']+["'])\s*,\s*([A-Za-z_$][\w$.]*)`)
	laravelGroup   = regexp.MustCompile(`Route::middleware\s*\(\s*([^()]*)\)\s*(?:->\s*\w+\s*\([^()]*\)\s*)*->\s*group\s*\(`)
	authish        = regexp.MustCompile(`(?i)auth|guard|login|permission|role|jwt|token|protect|secur|admin|session|verify|passport|acl`)
)

func (s *source) symbols() *symbols {
	sy := &symbols{vars: map[string]string{}, consts: map[string]string{}, keys: map[string]string{}, prefixes: map[string]string{}, addrs: map[string]string{}}
	intern := map[string]string{}
	in := func(v string) string {
		if x, ok := intern[v]; ok {
			return x
		}
		c := strings.Clone(v)
		intern[c] = c
		return c
	}
	setVar := func(name, typ string) {
		name, typ = strings.TrimPrefix(strings.TrimSpace(name), "$"), strings.TrimSpace(typ)
		if name == "" || typ == "" {
			return
		}
		if _, ok := sy.vars[name]; !ok {
			sy.vars[in(name)] = in(strings.Join(strings.Fields(typ), ""))
		}
	}
	code := s.code
	switch s.lang {
	case langJava, langCSharp, langDart, langScala, langKotlin, langJS, langSwift:
		if s.lang != langKotlin && s.lang != langSwift && s.lang != langScala {
			for _, m := range typedName.FindAllStringSubmatch(code, -1) {
				setVar(m[2], m[1])
			}
		}
		if s.lang == langCSharp {
			for _, m := range csProperty.FindAllStringSubmatch(code, -1) {
				setVar(m[2], m[1])
			}
		}
		if s.lang != langJava && s.lang != langCSharp {
			for _, m := range nameColonType.FindAllStringSubmatch(code, -1) {
				setVar(m[1], m[2])
			}
		}
	case langPython:
		for _, m := range nameColonType.FindAllStringSubmatch(code, -1) {
			setVar(m[1], m[2])
		}
		for _, m := range withAs.FindAllStringSubmatch(code, -1) {
			setVar(m[2], m[1])
		}
	case langPHP:
		for _, m := range phpTyped.FindAllStringSubmatch(code, -1) {
			setVar(m[2], m[1][strings.LastIndexByte(m[1], '\\')+1:])
		}
	case langGo:
		for _, m := range goFunc.FindAllStringSubmatch(code, -1) {
			for _, part := range []string{m[1], m[2]} {
				for _, p := range goParam.FindAllStringSubmatch(part, -1) {
					setVar(p[1], p[2])
				}
			}
		}
		// Struct fields: `svc *orders.Service` on its own line inside a struct.
		for _, loc := range rx(`\bstruct\s*\{`).FindAllStringIndex(code, -1) {
			body, _ := balanced(code, loc[1]-1)
			for _, line := range strings.Split(body, "\n") {
				if p := goParam.FindStringSubmatch(strings.TrimSpace(line)); p != nil && strings.HasPrefix(strings.TrimSpace(line), p[1]) {
					setVar(p[1], p[2])
				}
			}
		}
	}
	for _, m := range constructed.FindAllStringSubmatch(code, -1) {
		typ := m[2]
		if s.lang == langGo {
			typ = strings.Replace(typ, ".New", ".", 1)
			typ = strings.TrimPrefix(typ, "New")
		}
		setVar(m[1], typ)
	}
	if s.lang == langPython || s.lang == langGo || s.lang == langJS {
		for _, m := range moduleConstructed.FindAllStringSubmatch(code, -1) {
			setVar(m[1], m[2])
		}
	}
	// self.repo = repo: the field takes the parameter's type.
	for line := 1; line <= len(s.lines); line++ {
		if m := assignedFrom.FindStringSubmatch(s.lineText(line)); m != nil && m[1] != m[2] {
			if t, ok := sy.vars[m[2]]; ok {
				setVar(m[1], t)
			}
		} else if m != nil {
			// self.repo = repo with the same name: already the parameter's.
			continue
		}
	}

	// Constants.
	var constRe *regexp.Regexp
	switch s.lang {
	case langJava, langCSharp, langKotlin, langScala, langDart:
		constRe = javaConst
	case langGo:
		constRe = goConst
	}
	if constRe != nil {
		for _, m := range constRe.FindAllStringSubmatch(code, -1) {
			sy.consts[in(m[1])] = in(evalConcat(m[2], sy.consts))
		}
	}
	if s.lang == langJS || s.lang == langPython || s.lang == langKotlin || s.lang == langPHP || s.lang == langDart || s.lang == langSwift {
		for _, m := range scriptConst.FindAllStringSubmatch(code, -1) {
			if u := unquote(m[2]); u != "" {
				sy.consts[in(m[1])] = in(u)
			}
		}
	}
	if s.lang == langPHP {
		for _, m := range rx(`\bconst\s+([A-Z_][A-Z0-9_]*)\s*=\s*(['"][^'"]*['"])`).FindAllStringSubmatch(code, -1) {
			sy.consts[in(m[1])] = in(unquote(m[2]))
		}
	}

	// Configuration bound to fields.
	for _, m := range valueKey.FindAllStringSubmatch(code, -1) {
		sy.keys[in(m[2])] = in(m[1])
	}
	for _, loc := range configProps.FindAllStringSubmatchIndex(code, -1) {
		if t := s.typeNear(s.line(loc[0])); t != "" {
			sy.prefixes[in(t)] = in(code[loc[2]:loc[3]])
		}
	}
	// .NET options: Configure<PartnerOptions>(config.GetSection("Partner")).
	for _, m := range optionsSection.FindAllStringSubmatch(code, -1) {
		if s.lang != langCSharp {
			break
		}
		sy.prefixes[in(m[1]+m[3])] = in(m[2] + m[4])
	}

	// Call sites.
	for _, m := range callPattern.FindAllStringSubmatchIndex(code, -1) {
		recv := strings.Join(strings.Fields(code[m[2]:m[3]]), "")
		sy.calls = append(sy.calls, callSite{Recv: in(normalRecv(recv)), Name: in(code[m[4]:m[5]]), Line: int32(s.line(m[0])), At: int32(m[0]), Open: int32(m[1] - 1)})
	}
	for _, m := range barePattern.FindAllStringSubmatchIndex(code, -1) {
		name := code[m[2]:m[3]]
		if notCalls[name] || notCalls[strings.ToLower(name)] {
			continue
		}
		sy.calls = append(sy.calls, callSite{Name: in(name), Line: int32(s.line(m[2])), At: int32(m[2]), Open: int32(m[1] - 1)})
	}
	sort.SliceStable(sy.calls, func(i, j int) bool { return sy.calls[i].Line < sy.calls[j].Line })

	for _, hint := range []string{"Matcher", "anyRequest", "authorize", "ApplicationPath", ".use(", "Route::middleware"} {
		if strings.Contains(code, hint) {
			s.securityConfig(sy)
			break
		}
	}
	return sy
}

// typeNear is the type declared on or just below a line: the class an
// annotation above it marks.
func (s *source) typeNear(line int) string {
	for _, t := range s.typeDecls() {
		if t.Line >= line && t.Line <= line+4 {
			return t.Name
		}
	}
	return ""
}

var primitive = map[string]bool{"string": true, "int": true, "long": true, "boolean": true, "bool": true, "double": true, "float": true, "byte": true, "short": true, "char": true, "object": true, "integer": true, "number": true, "any": true, "void": true, "str": true, "dict": true, "list": true, "optional": true, "map": true, "set": true, "date": true, "datetime": true, "localdate": true, "localdatetime": true, "instant": true, "uuid": true, "bigdecimal": true, "decimal": true, "guid": true, "unit": true, "nothing": true, "array": true, "record": true, "promise": true, "observable": true, "task": true, "error": true, "exception": true, "context": true, "request": true, "response": true, "httpservletrequest": true, "httpservletresponse": true, "model": true, "pageable": true, "principal": true, "authentication": true, "logger": true, "log": true, "ilogger": true, "cancellationtoken": true}

// normalRecv drops the receiver's own-object prefix: `this.orders` and
// `self.orders` and `$this->orders` are all `orders`; `this` alone is "this".
func normalRecv(r string) string {
	for _, p := range []string{"this.", "self.", "$this->", "this->", "this?.", "@"} {
		if strings.HasPrefix(r, p) {
			return strings.TrimPrefix(r, p)
		}
	}
	switch r {
	case "self", "$this", "base", "super", "parent", "static":
		return "this"
	}
	return r
}

// evalConcat is a constant built from strings and other constants.
func evalConcat(expr string, consts map[string]string) string {
	var b strings.Builder
	for _, part := range strings.Split(expr, "+") {
		part = strings.TrimSpace(part)
		if u := unquote(part); u != "" {
			b.WriteString(u)
		} else if v, ok := consts[part]; ok {
			b.WriteString(v)
		}
	}
	return b.String()
}

// securityConfig reads the access a security configuration gives paths:
// Spring Security's matchers (Java and the Kotlin DSL), Express's
// app.use('/admin', requireAuth), and Laravel's middleware groups.
func (s *source) securityConfig(sy *symbols) {
	code := s.code
	switch s.lang {
	case langJava, langKotlin:
		for _, m := range rx(`@ApplicationPath\s*\(\s*"([^"]*)"`).FindAllStringSubmatch(code, -1) {
			sy.appPaths = append(sy.appPaths, m[1])
		}
		// A filter chain's securityMatcher scopes its rules, anyRequest()
		// included: anyRequest in a chain for /actuator/** is not every
		// route.
		type scope struct {
			at       int
			patterns []string
		}
		var scopes []scope
		chains := rx(`\bSecurityFilterChain\b|\bSecurityWebFilterChain\b`).FindAllStringIndex(code, -1)
		chainOf := func(at int) int {
			c := -1
			for _, ch := range chains {
				if ch[0] <= at {
					c = ch[0]
				}
			}
			return c
		}
		for _, loc := range rx(`\.\s*securityMatchers?\s*\(`).FindAllStringIndex(code, -1) {
			inner, _ := balanced(code, loc[1]-1)
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
			inner, end := balanced(code, loc[1]-1)
			rule := securityRule.FindStringSubmatch(code[end:min(len(code), end+300)])
			if rule == nil {
				continue
			}
			r := rule[1]
			if strings.TrimSpace(rule[2]) != "" {
				r += "(" + strings.Join(strings.Fields(rule[2]), " ") + ")"
			}
			method, patterns := matcherArgs(inner)
			for _, p := range patterns {
				sy.authRules = append(sy.authRules, authRule{Pattern: p, Method: method, Rule: r, Line: s.line(loc[0])})
			}
		}
		for _, m := range anyRequest.FindAllStringSubmatchIndex(code, -1) {
			r := code[m[2]:m[3]]
			if a := strings.TrimSpace(code[m[4]:m[5]]); a != "" {
				r += "(" + a + ")"
			}
			patterns, known := scopeOf(m[0])
			if !known {
				continue
			}
			for _, p := range patterns {
				sy.authRules = append(sy.authRules, authRule{Pattern: p, Rule: r, Line: s.line(m[0])})
			}
		}
		for _, m := range kotlinAuthz.FindAllStringSubmatchIndex(code, -1) {
			p := "/**"
			if q := unquote(code[m[4]:m[5]]); q != "" {
				p = q
			}
			method := ""
			if m[2] >= 0 {
				method = code[m[2]:m[3]]
			}
			sy.authRules = append(sy.authRules, authRule{Pattern: p, Method: method, Rule: strings.Join(strings.Fields(code[m[6]:m[7]]), ""), Line: s.line(m[0])})
		}
	case langJS:
		for _, m := range jsUseAuth.FindAllStringSubmatchIndex(code, -1) {
			name := code[m[4]:m[5]]
			if !authish.MatchString(name) {
				continue
			}
			sy.authRules = append(sy.authRules, authRule{Pattern: strings.TrimRight(unquote(code[m[2]:m[3]]), "/") + "/**", Rule: lastName(name), Line: s.line(m[0])})
		}
	case langPHP:
		for _, loc := range laravelGroup.FindAllStringSubmatchIndex(code, -1) {
			mw := strings.Join(strings.Fields(code[loc[2]:loc[3]]), " ")
			if !authish.MatchString(mw) {
				continue
			}
			_, end := balanced(code, loc[1]-1)
			sy.authRanges = append(sy.authRanges, authRange{From: s.line(loc[0]), To: s.line(end), Rule: "middleware(" + mw + ")"})
		}
	}
}

// authMarks are the marks that say who may call a route.
var authMarks = map[string]bool{
	"PreAuthorize": true, "PostAuthorize": true, "Secured": true, "RolesAllowed": true, "PermitAll": true, "DenyAll": true,
	"Authorize": true, "AllowAnonymous": true, "UseGuards": true, "Roles": true, "Public": true, "IsGranted": true,
	"Security": true, "RequiresPermissions": true, "RequiresRoles": true, "RequiresAuthentication": true, "Authenticated": true,
	"login_required": true, "permission_required": true, "user_passes_test": true, "staff_member_required": true,
	"superuser_required": true, "jwt_required": true, "auth_required": true, "requires_auth": true, "Anonymous": true,
}

// authLike is a mark whose name says it guards access, for the ones a
// codebase defines itself: [AuthorizeAdmin], [CheckPermission(...)],
// @RequiresTenantAccess, @UseGuards-style decorators.
var authLike = regexp.MustCompile(`(?i)authori[sz]|permission|^checkaccess|^requires?(auth|role|login|scope|user|tenant|access)|guard|^roles?$|^secured$|^isgranted$|anonymous`)

func isAuthMark(name string) bool {
	return authMarks[name] || authLike.MatchString(name) && !strings.Contains(strings.ToLower(name), "antiforgery")
}

// docMarks say a route is described for its contract by annotations.
var docMarks = map[string]bool{
	"Operation": true, "ApiOperation": true, "SwaggerOperation": true, "ProducesResponseType": true, "ApiResponse": true,
	"ApiResponses": true, "OpenApiOperation": true, "ApiOkResponse": true, "Tag": true, "Api": true, "ApiTags": true,
	"extend_schema": true, "swagger_auto_schema": true, "OA": true,
}

// markText renders a mark for the auth column: its name and, when short,
// its arguments.
func markText(m mark) string {
	if len(m.Args) == 0 {
		return m.Name
	}
	a := strings.Join(strings.Fields(strings.Join(m.Args, ", ")), " ")
	if len(a) > 80 {
		a = a[:77] + "…"
	}
	return m.Name + "(" + a + ")"
}

var (
	pyClassHead = regexp.MustCompile(`(?m)^class\s+(\w+)\s*(?:\([^)]*\))?\s*:`)
	drfAccess   = regexp.MustCompile(`(?m)^\s+(permission_classes)\s*=\s*[\[(]([^\])]*)[\])]`)
)

// classAuth is what each Python class says about access: Django REST
// framework's permission_classes, which apply to every route the view
// answers.
func (s *source) classAuth() map[string]string {
	if s.lang != langPython || !strings.Contains(s.code, "permission_classes") {
		return nil
	}
	heads := pyClassHead.FindAllStringSubmatchIndex(s.code, -1)
	out := map[string]string{}
	for i, h := range heads {
		end := len(s.code)
		if i+1 < len(heads) {
			end = heads[i+1][0]
		}
		for _, m := range drfAccess.FindAllStringSubmatch(s.code[h[1]:end], -1) {
			out[s.code[h[2]:h[3]]] = "permission_classes(" + strings.Join(strings.Fields(strings.Trim(m[2], " ,")), " ") + ")"
		}
	}
	return out
}

// matcherArgs reads a matcher's method and path patterns, through a wrapper
// such as API_MATCHER.matcher(HttpMethod.GET, "/api/*/x").
func matcherArgs(inner string) (string, []string) {
	method := ""
	var patterns []string
	if m := rx(`HttpMethod\s*\.\s*(\w+)`).FindStringSubmatch(inner); m != nil {
		method = m[1]
	}
	for _, m := range rx(`"([^"]*)"`).FindAllStringSubmatch(inner, -1) {
		if strings.HasPrefix(m[1], "/") || m[1] == "*" || strings.HasPrefix(m[1], "**") {
			patterns = append(patterns, m[1])
		}
	}
	return method, patterns
}
