package recogniser

import (
	"regexp"
	"strings"
)

var (
	envRead    = regexp.MustCompile(`^(?:process\s*\.\s*env|import\s*\.\s*meta\s*\.\s*env)\s*(?:\.\s*([A-Za-z_]\w*)|\[\s*['"]([^'"]+)['"]\s*\])$|^(?:os\s*\.\s*(?:getenv|Getenv|environ\s*\.\s*get)|System\s*\.\s*getenv|Environment\s*\.\s*GetEnvironmentVariable|getenv|env|Deno\s*\.\s*env\s*\.\s*get)\s*\(\s*['"]([^'"]+)['"]|^os\s*\.\s*environ\s*\[\s*['"]([^'"]+)['"]\s*\]$`)
	cfgRead    = regexp.MustCompile(`^(?:[\w.]*[cC]onfig(?:uration)?|_config|_configuration|[\w.]*[sS]ettings|[\w.]*[eE]nv(?:ironment)?)\s*\[\s*["']([^"']+)["']\s*\]$|\.\s*(?:GetValue\s*(?:<[^<>]*>)?|getProperty|getRequiredProperty|GetString|getString|get|GetConnectionString)\s*\(\s*["']([^"']+)["']|^config\s*\(\s*['"]([^'"]+)['"]|^viper\s*\.\s*Get\w*\s*\(\s*"([^"]+)"`)
	djangoSet  = regexp.MustCompile(`^settings\s*\.\s*([A-Z][A-Z0-9_]*)$`)
	fmtCall    = regexp.MustCompile(`^(?:String\s*\.\s*format|fmt\s*\.\s*Sprintf|sprintf|string\s*\.\s*Format|String\s*\.\s*Format|format)\s*\(`)
	wrapCall   = regexp.MustCompile(`^(?:URI\s*\.\s*create|new\s+URI|new\s+Uri|new\s+URL|URI|URL|Uri\s*\.\s*parse|url\s*\.\s*Parse|URI\s*\.\s*parse|java\.net\.URI\s*\.\s*create|UriComponentsBuilder\s*\.\s*fromHttpUrl|UriComponentsBuilder\s*\.\s*fromUriString|urljoin|urllib\.parse\.urljoin|Path\s*\.\s*Combine|path\s*\.\s*join|url\s*\.\s*JoinPath|Url\s*\.\s*Combine)\s*\(`)
	interp     = regexp.MustCompile(`\$\{([^{}]+)\}|\{\$?([A-Za-z_][\w.>-]*(?:\([^(){}]*\))?)\}|\$([A-Za-z_][\w]*)`)
	pyFmtCall  = regexp.MustCompile(`^(["'][^"']*["'])\s*\.\s*format\s*\(`)
	stripCalls = regexp.MustCompile(`\.\s*(?:toString|toUriString|build|toUri|encode|expand|buildAndExpand|trim|strip|rstrip|TrimEnd|TrimStart|Trim|String)\s*\([^()]*\)$`)
)

// RenderAddr renders an address expression: literals as they are, every
// other part as a marker the linker can resolve.
func (s *Source) RenderAddr(expr string, sy *Symbols) string {
	return s.render(strings.TrimSpace(expr), sy, 0)
}

func (s *Source) render(e string, sy *Symbols, depth int) string {
	if e == "" || depth > 6 {
		return ""
	}
	e = strings.TrimSpace(stripCalls.ReplaceAllString(strings.TrimSpace(e), ""))
	for strings.HasPrefix(e, "(") && strings.HasSuffix(e, ")") {
		inner, end := Balanced(e, 0)
		if end != len(e) {
			break
		}
		e = strings.TrimSpace(inner)
	}
	// Wrappers that build a URI from a string: URI.create(x), new URL(p, base).
	if loc := wrapCall.FindStringIndex(e); loc != nil {
		inner, end := Balanced(e, loc[1]-1)
		if end == len(e) || strings.HasPrefix(strings.TrimSpace(e[end:]), ".") {
			args := SplitArgs(inner)
			switch len(args) {
			case 0:
				return ""
			case 1:
				return s.render(args[0], sy, depth+1)
			}
			name := e[:loc[1]-1]
			if strings.Contains(name, "URL") && !strings.Contains(name, "URI") {
				// new URL(path, base)
				return JoinAddr(s.render(args[1], sy, depth+1), s.render(args[0], sy, depth+1))
			}
			out := s.render(args[0], sy, depth+1)
			for _, a := range args[1:] {
				out = JoinAddr(out, s.render(a, sy, depth+1))
			}
			return out
		}
	}
	// Formatting: String.format("%s/orders/%d", base, id), "{}/x".format(b).
	if loc := fmtCall.FindStringIndex(e); loc != nil {
		inner, _ := Balanced(e, loc[1]-1)
		args := SplitArgs(inner)
		if len(args) > 0 {
			return s.format(Unquote(args[0]), args[1:], sy, depth)
		}
	}
	if m := pyFmtCall.FindStringSubmatchIndex(e); m != nil {
		inner, _ := Balanced(e, m[1]-1)
		return s.format(Unquote(e[m[2]:m[3]]), SplitArgs(inner), sy, depth)
	}
	if i := strings.Index(e, `" % `); i > 0 || strings.Index(e, `' % `) > 0 {
		if i <= 0 {
			i = strings.Index(e, `' % `)
		}
		rest := strings.TrimSpace(e[i+3:])
		rest = strings.Trim(rest, "()")
		return s.format(Unquote(e[:i+1]), SplitArgs(rest), sy, depth)
	}
	// Concatenation: +, or the language's own.
	parts := SplitTop(e, "+")
	if len(parts) == 1 && s.syntax().DotConcat {
		parts = SplitTop(e, ".")
	}
	if len(parts) > 1 {
		var b strings.Builder
		for _, p := range parts {
			b.WriteString(s.render(p, sy, depth+1))
		}
		return b.String()
	}
	// A string, interpolations included.
	if lit, ok := LiteralText(e); ok {
		return s.interpolate(lit, sy, depth)
	}
	return s.token(e, sy, depth)
}

// LiteralText is the text of a string literal of any kind: "x", 'x', `x`,
// f"x", $"x", @"x", r"x", """x""".
func LiteralText(e string) (string, bool) {
	t := strings.TrimLeft(e, "fFrRbBuU$@")
	if len(t) < 2 {
		return "", false
	}
	q := t[0]
	if q != '"' && q != '\'' && q != '`' {
		return "", false
	}
	if strings.HasPrefix(t, `"""`) && strings.HasSuffix(t, `"""`) && len(t) >= 6 {
		return t[3 : len(t)-3], true
	}
	if t[len(t)-1] != q {
		return "", false
	}
	return t[1 : len(t)-1], true
}

// interpolate renders the expressions inside a literal: `${base}/x`,
// f"{base}/x", "$base/x", "{$this->base}/x".
func (s *Source) interpolate(lit string, sy *Symbols, depth int) string {
	if !strings.ContainsAny(lit, "{$") {
		return lit
	}
	// An OpenAPI-style template `/orders/{id}` is a literal path parameter,
	// not an interpolation, unless the language interpolates braces.
	x := s.syntax()
	return interp.ReplaceAllStringFunc(lit, func(m string) string {
		g := interp.FindStringSubmatch(m)
		switch {
		case g[1] != "":
			return s.token(strings.TrimSpace(g[1]), sy, depth+1)
		case g[2] != "":
			if !x.BraceInterpolation && !strings.HasPrefix(m, "{$") {
				return m
			}
			return s.token(strings.TrimSpace(strings.TrimPrefix(g[2], "$")), sy, depth+1)
		case g[3] != "":
			if x.DollarInterpolation {
				return s.token(g[3], sy, depth+1)
			}
		}
		return m
	})
}

// format fills %s, %d, %v, {} and {0} in a format string with its rendered
// arguments.
func (s *Source) format(f string, args []string, sy *Symbols, depth int) string {
	i := 0
	next := func() string {
		if i >= len(args) {
			return "{}"
		}
		a := s.render(args[i], sy, depth+1)
		i++
		return a
	}
	return Rx(`%[-+ #0-9.]*[sdvqfx]|\{\d*\}|\{[a-z_]\w*\}`).ReplaceAllStringFunc(f, func(string) string { return next() })
}

// token renders one non-literal part of an address.
func (s *Source) token(e string, sy *Symbols, depth int) string {
	e = strings.TrimSpace(e)
	if e == "" {
		return ""
	}
	if m := envRead.FindStringSubmatch(e); m != nil {
		return "{env:" + m[1] + m[2] + m[3] + m[4] + "}"
	}
	if m := cfgRead.FindStringSubmatch(e); m != nil {
		key := m[1] + m[2] + m[3] + m[4]
		if strings.Contains(e, "GetConnectionString") {
			key = "ConnectionStrings:" + key
		}
		return "{config:" + key + "}"
	}
	if m := djangoSet.FindStringSubmatch(e); m != nil {
		return "{config:" + m[1] + "}"
	}
	if a, ok := sy.Addrs[LastSegment(NormalRecv(strings.Join(strings.Fields(e), "")))]; ok && depth < 5 {
		return a
	}
	name := NormalRecv(strings.Join(strings.Fields(e), ""))
	name = strings.TrimSuffix(strings.TrimSuffix(name, "()"), "!!")
	if i := strings.IndexByte(name, '('); i > 0 {
		// A call that builds the address: named by what it calls.
		name = name[:i]
	}
	root, member := name, ""
	if i := strings.LastIndexAny(name, ".>"); i > 0 {
		root, member = strings.TrimRight(name[:i], "-"), name[i+1:]
		root = LastSegment(root)
	}
	if member == "" {
		if v, ok := sy.Consts[root]; ok {
			return v
		}
		if k, ok := sy.Keys[root]; ok {
			return "{config:" + k + "}"
		}
		if t, ok := sy.Vars[root]; ok && strings.HasPrefix(strings.ToLower(Genericless(t)), "uri") {
			return "{var:" + root + "}"
		}
		if !IsIdentifier(strings.TrimPrefix(root, "$")) {
			return "{}"
		}
		return "{var:" + strings.TrimPrefix(root, "$") + "}"
	}
	// props.getBaseUrl(), options.Value.BaseUrl, settings.base_url: a
	// member of a configuration-bound type.
	if v, ok := sy.Consts[member]; ok {
		return v
	}
	prop := strings.TrimPrefix(member, "get")
	if prop != member && len(prop) > 0 {
		prop = strings.ToLower(prop[:1]) + prop[1:]
	}
	if t, ok := sy.Vars[root]; ok {
		typ := Genericless(t)
		if (typ == "IOptions" || typ == "IOptionsMonitor" || typ == "IOptionsSnapshot") && strings.Contains(t, "<") {
			typ = strings.Trim(t[strings.IndexByte(t, '<')+1:], "<> ")
		}
		return "{prop:" + typ[strings.LastIndexAny(typ, `.\`)+1:] + ":" + prop + "}"
	}
	if k, ok := sy.Keys[member]; ok {
		return "{config:" + k + "}"
	}
	if strings.ToUpper(member) == member && len(member) > 1 {
		return "{const:" + member + "}"
	}
	return "{var:" + member + "}"
}

func JoinAddr(base, p string) string {
	if base == "" {
		return p
	}
	if p == "" {
		return base
	}
	if strings.Contains(p, "://") {
		return p
	}
	return strings.TrimRight(base, "/") + "/" + strings.TrimLeft(p, "/")
}

// LooksLikeAddr is whether a rendered address can be an HTTP address: a
// path, a URL, or something built from configuration. `get(key)` on a
// generic client with a bare variable is not enough.
func LooksLikeAddr(rendered, raw string) bool {
	r := strings.TrimSpace(rendered)
	switch {
	case r == "":
		return false
	case strings.HasPrefix(r, "/"), strings.Contains(r, "://"), strings.HasPrefix(r, "{config:"), strings.HasPrefix(r, "{env:"), strings.HasPrefix(r, "{prop:"):
		return true
	case strings.HasPrefix(r, "{var:"):
		v := strings.ToLower(r)
		return strings.Contains(v, "url") || strings.Contains(v, "uri") || strings.Contains(v, "endpoint") || strings.Contains(v, "path") || strings.Contains(v, "/") || strings.Contains(v, "host") || strings.Contains(v, "address")
	}
	// A relative path without its slash: "users/{id}", "api/orders".
	_, literal := LiteralText(strings.TrimSpace(raw))
	return Rx(`^[a-z][\w.-]*/[\w{}./:-]*$`).MatchString(r) && literal
}

var (
	timeoutArg = regexp.MustCompile(`(?i)\b(timeout|connect_?timeout|read_?timeout|request_?timeout|callTimeout|responseTimeout|Timeout)\s*[:=(]|->timeout\s*\(|\.timeout\s*\(|\.Timeout\s*=|['"]timeout['"]\s*=>`)
	retryArg   = regexp.MustCompile(`(?i)->retry\s*\(|\.retry(?:When|Backoff)?\s*\(|max_retries|retries\s*[:=]|AddPolicyHandler|AddTransientHttpErrorPolicy|AddStandardResilienceHandler|AddResilienceHandler|Retry\.(?:backoff|fixedDelay|max)`)
	circuitArg = regexp.MustCompile(`(?i)CircuitBreaker|circuitBreaker|AddStandardResilienceHandler`)
)

// ResilienceIn is what a call's options say about its resilience.
func ResilienceIn(text string) string {
	var r []string
	if timeoutArg.MatchString(text) {
		r = append(r, "timeout")
	}
	if retryArg.MatchString(text) {
		r = append(r, "retry")
	}
	if circuitArg.MatchString(text) {
		r = append(r, "circuit_breaker")
	}
	return strings.Join(r, ",")
}

// ResilienceOf is what a member's marks say about its resilience, by the
// language's recognisers.
func (s *Source) ResilienceOf(marks []Mark) string {
	set := map[string]bool{}
	for _, m := range marks {
		for _, r := range s.language.recognisers {
			if v, ok := r.Resilience[m.Name]; ok {
				set[v] = true
			}
		}
	}
	var out []string
	for _, k := range []string{"timeout", "retry", "circuit_breaker"} {
		if set[k] {
			out = append(out, k)
		}
	}
	return strings.Join(out, ",")
}

// HTTPModules are modules that are HTTP clients: `axios.get(url)`.
var HTTPModules = map[string]bool{"axios": true, "requests": true, "httpx": true, "got": true, "ky": true, "superagent": true, "Http": true, "Unirest": true, "unirest": true, "needle": true, "ofetch": true}

// ClientName is the library a call goes through, from the receiver's type
// or name.
func (s *Source) ClientName(recv string, sy *Symbols) string {
	last := LastSegment(recv)
	t := Genericless(sy.Vars[last])
	if t == "" {
		t = recv
	}
	lt := strings.ToLower(t)
	name := ""
	for _, r := range s.language.recognisers {
		if r.ClientName != nil {
			name = r.ClientName(s, lt, recv)
			break
		}
	}
	if name == "" {
		name = ClientLibrary(lt, recv)
	}
	if name != "" {
		return name
	}
	if HTTPModules[recv] {
		return recv
	}
	return "http"
}

// ClientLibrary is the client library a type or receiver names, among those
// that name themselves: RestTemplate, axios, httpx; "" when it does not.
func ClientLibrary(lt, recv string) string {
	switch {
	case strings.Contains(lt, "resttemplate"):
		return "resttemplate"
	case strings.Contains(lt, "webclient"):
		return "webclient"
	case strings.Contains(lt, "restclient"):
		return "restclient"
	case strings.Contains(lt, "okhttp"):
		return "okhttp"
	case strings.Contains(lt, "httpservice"):
		return "nestjs-http"
	case strings.Contains(lt, "axios") || recv == "axios":
		return "axios"
	case strings.Contains(lt, "httpx"):
		return "httpx"
	case strings.Contains(lt, "aiohttp") || strings.Contains(lt, "clientsession"):
		return "aiohttp"
	case strings.Contains(lt, "requests"):
		return "requests"
	case strings.Contains(lt, "resty"):
		return "resty"
	}
	return ""
}
