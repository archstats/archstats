package navigation

import (
	"regexp"
	"regexp/syntax"
	"strings"

	r "github.com/archstats/archstats/extensions/navigation/recogniser"
)

// Where the code goes out: an HTTP call, a message sent, a database opened,
// a mail sent, a file put in a bucket. Read from how each client library is
// spelled, gated on the receiver being that client (its declared type, or
// a name no other kind of object goes by), so `map.get(key)` is never an
// HTTP GET. The address is rendered from what the call was given --
// literals, constants, configuration keys, environment variables -- and
// resolved against the configuration files when the snapshot is linked.
//
// An HTTP call made by calling a verb on a client variable is read here, the
// same way in every language; the language's recognisers read the rest:
// declared clients, base addresses, the calls spelled only their way, and
// rule tables for messaging, datastores, mail and the like.

var (
	httpTypes  = regexp.MustCompile(`^(?:RestTemplate|TestRestTemplate|WebClient|RestClient|HttpClient|OkHttpClient|CloseableHttpClient|HttpClients|WebTarget|HttpService|AxiosInstance|AxiosStatic|Session|requests\.Session|httpx\.Client|httpx\.AsyncClient|aiohttp\.ClientSession|ClientSession|http\.Client|resty\.Client|ClientInterface|HttpClientInterface|IRestClient|RestSharp\.RestClient|IFlurlClient|FlurlClient|HttpRequestFactory|Client)$`)
	httpNames  = regexp.MustCompile(`(?i)(^|_)(http|rest|web|api)|(http|rest|web|api)(client|template|service)?$|^_?(http|client|session|httpclient|resttemplate|webclient)$`)
	verbMember = regexp.MustCompile(`(?i)^(get|post|put|patch|delete|del|head|options)(?:for(?:object|entity|location))?(?:as(?:json|xml))?(?:async)?$|^(get)(?:string|stream|bytearray|fromjson)(?:async)?$|^(delete)fromjson(?:async)?$`)
	requestish = map[string]bool{"request": true, "requestasync": true, "exchange": true, "send": true, "sendasync": true, "execute": true, "fetch": true}
)

// rules holds what the extension worked out once about its recognisers'
// rule tables: the literal each pattern cannot match without, and the
// words a receiver gate is made of.
type rules struct {
	outHints  map[*r.OutRule]string
	callWords map[*r.CallRule][]string
	// clientModules are the modules that are HTTP clients in one language
	// alone.
	clientModules map[string]bool
}

func newRules(recs []*r.Recogniser) *rules {
	x := &rules{outHints: map[*r.OutRule]string{}, callWords: map[*r.CallRule][]string{}, clientModules: map[string]bool{}}
	for _, rec := range recs {
		for i := range rec.OutRules {
			x.outHints[&rec.OutRules[i]] = ruleHint(rec.OutRules[i].Re)
		}
		for i := range rec.CallRules {
			x.callWords[&rec.CallRules[i]] = gateWords(rec.CallRules[i].Gate)
		}
		for _, m := range rec.ClientModules {
			x.clientModules[m] = true
		}
	}
	return x
}

// outbound reads every outbound call in a file.
func (e *extension) outbound(s *source, sy *symbols) ([]outCall, []clientBase) {
	code := s.Code
	recs := s.Language().Recognisers()
	var out []outCall
	seen := map[int]bool{}
	add := func(c outCall, at int) {
		if seen[at] {
			return
		}
		seen[at] = true
		c.Line = s.Line(at)
		if c.Resilience == "" {
			c.Resilience = resilienceIn(s.Statement(at))
		}
		out = append(out, c)
	}
	addresses(s, sy)
	var bases []clientBase
	for _, rec := range recs {
		if rec.ClientBases != nil {
			bases = append(bases, rec.ClientBases(s, sy)...)
		}
	}

	// Declared clients: Feign, MicroProfile, HttpExchange, Retrofit, Refit.
	for _, rec := range recs {
		if rec.DeclaredClients != nil {
			out = append(out, rec.DeclaredClients(s)...)
		}
	}

	// Imperative HTTP calls on a client variable: x.get(url), x.PostAsync(url).
	for _, cs := range sy.Calls {
		if cs.Recv == "" {
			continue
		}
		recv, member := cs.Recv, cs.Name
		method := httpVerbOf(member)
		reqish := requestish[strings.ToLower(member)]
		if method == "" && !reqish {
			continue
		}
		if !e.isHTTPClient(s, recv, sy, bases) {
			continue
		}
		inner, _ := balanced(code, int(cs.Open))
		args := splitArgs(inner)
		if len(args) == 0 {
			continue
		}
		addr := args[0]
		if reqish || method == "" {
			if mm := methodLiteral(args[0]); mm != "" {
				method = mm
				if len(args) < 2 {
					continue
				}
				addr = args[1]
			} else if len(args) > 1 {
				if mm := methodLiteral(args[1]); mm != "" {
					method = mm
				}
			}
			if method == "" {
				method = "ANY"
			}
		}
		rendered := s.RenderAddr(addr, sy)
		_, typed := sy.Vars[lastSegment(recv)]
		if rendered == "" || !looksLikeAddr(rendered, addr) && !(typed && method != "") {
			continue
		}
		add(outCall{Kind: "http", Client: s.ClientName(recv, sy), Method: method, Addr: rendered, Recv: lastSegment(recv)}, int(cs.At))
	}

	// Calls on a receiver that sends a message, sends mail, opens a session.
	lower := ""
	for _, rec := range recs {
		for i := range rec.CallRules {
			rl := &rec.CallRules[i]
			if !rl.Reads(s) {
				continue
			}
			if words := e.rules.callWords[rl]; len(words) > 0 {
				if lower == "" {
					lower = strings.ToLower(code)
				}
				if !containsAny(lower, words) {
					continue
				}
			}
			for _, cs := range sy.Calls {
				if cs.Recv == "" || !rl.Members.MatchString(cs.Name) {
					continue
				}
				if rl.Recv != nil && !rl.Recv.MatchString(cs.Recv) || rl.Gate != "" && !gateMatches(rl.Gate, cs.Recv, sy) {
					continue
				}
				rest := code[cs.Open:min(len(code), int(cs.Open)+400)]
				var tail []int
				if rl.Tail != nil {
					if tail = rl.Tail.FindStringSubmatchIndex(rest); tail == nil {
						continue
					}
				}
				body, _ := balanced(code, int(cs.Open))
				if rl.Tail != nil && strings.HasSuffix(rest[:tail[1]], "{") {
					body, _ = balanced(code, int(cs.Open)+tail[1]-1)
				}
				c := outCall{Kind: rl.Kind, Client: rl.Client}
				args := splitArgs(body)
				switch {
				case rl.Key != "":
					v := objectKey(body, rl.Key)
					if v == "" && rl.Arg >= 0 && rl.Arg < len(positional(args)) {
						v = positional(args)[rl.Arg]
					}
					if v != "" {
						c.Addr = s.RenderAddr(v, sy)
					}
				case rl.Arg >= 0 && rl.Arg < len(args):
					c.Addr = s.RenderAddr(args[rl.Arg], sy)
				}
				if tail != nil && len(tail) > 2 && tail[2] >= 0 {
					c.Target = rest[tail[2]:tail[3]]
					if c.Addr == "" {
						c.Addr = c.Target
					}
				}
				add(c, int(cs.At))
			}
		}
	}

	// The calls each language spells its own way.
	for _, rec := range recs {
		if rec.Calls != nil {
			rec.Calls(s, sy, add)
		}
	}

	// Table rules: messaging, datastores, mail, LDAP, storage, file transfer,
	// gRPC, GraphQL, SOAP.
	for _, rec := range recs {
		for i := range rec.OutRules {
			rl := &rec.OutRules[i]
			if hint := e.rules.outHints[rl]; !rl.Reads(s) || hint != "" && !strings.Contains(code, hint) {
				continue
			}
			for _, m := range rl.Re.FindAllStringSubmatchIndex(code, -1) {
				if rl.Gate != "" && m[2] >= 0 {
					recv := normalRecv(strings.Join(strings.Fields(code[m[2]:m[3]]), ""))
					if !gateMatches(rl.Gate, recv, sy) {
						continue
					}
				} else if rl.Gate != "" {
					continue
				}
				c := outCall{Kind: rl.Kind, Client: rl.Client, Method: rl.Method}
				last := code[m[1]-1]
				var body string
				if last == '(' || last == '{' {
					body, _ = balanced(code, m[1]-1)
				}
				args := splitArgs(body)
				switch {
				case rl.Key != "":
					v := objectKey(body, rl.Key)
					if v == "" && rl.Arg >= 0 && rl.Arg < len(positional(args)) {
						v = positional(args)[rl.Arg]
					}
					if v != "" {
						c.Addr = s.RenderAddr(v, sy)
					}
				case rl.Arg >= 0 && rl.Arg < len(args):
					c.Addr = s.RenderAddr(args[rl.Arg], sy)
				}
				// A captured name is the target: a gRPC service, a message type.
				if g := len(m) / 2; g > 1 {
					lastGroup := m[len(m)-2:]
					if lastGroup[0] >= 0 && (rl.Gate == "" || g > 3) {
						c.Target = code[lastGroup[0]:lastGroup[1]]
						c.Target = c.Target[strings.LastIndexByte(c.Target, '\\')+1:]
					}
				}
				if c.Kind == "message" && c.Addr == "" && c.Target != "" {
					c.Addr = c.Target
				}
				add(c, m[0])
			}
		}
	}
	return out, bases
}

// isHTTPClient is whether a receiver is an HTTP client: by its declared
// type, a module that is one, a variable a base was bound to, or a name
// nothing else goes by.
func (e *extension) isHTTPClient(s *source, recv string, sy *symbols, bases []clientBase) bool {
	if recv == "" || recv == "this" {
		return false
	}
	recs := s.Language().Recognisers()
	last := lastSegment(recv)
	if t, ok := sy.Vars[last]; ok {
		g := genericless(t)
		if httpTypes.MatchString(g) || httpTypes.MatchString(g[strings.LastIndexAny(g, `.\`)+1:]) && g != "Client" {
			if g != "Client" {
				return true
			}
			for _, rec := range recs {
				if rec.BareClient != nil && rec.BareClient(s) {
					return true
				}
			}
			return false
		}
		if strings.Contains(strings.ToLower(g), "redis") || strings.Contains(strings.ToLower(g), "cache") || strings.Contains(strings.ToLower(g), "repository") {
			return false
		}
	}
	for _, b := range bases {
		if b.Name == last {
			return true
		}
	}
	if r.HTTPModules[recv] {
		if !e.rules.clientModules[recv] {
			return true
		}
		// A module that is an HTTP client in one language alone.
		for _, rec := range recs {
			if contains(rec.ClientModules, recv) {
				return true
			}
		}
		return false
	}
	for _, rec := range recs {
		if contains(rec.NotClients, recv) {
			return false
		}
	}
	if httpNames.MatchString(last) && !rx(`(?i)request$|response$|context$|handler$|server$|router$|app$`).MatchString(last) {
		_, typed := sy.Vars[last]
		return !typed || httpTypes.MatchString(genericless(sy.Vars[last]))
	}
	return false
}

func gateMatches(gate, recv string, sy *symbols) bool {
	re := rx(gate)
	last := lastSegment(recv)
	if re.MatchString(last) {
		return true
	}
	if t, ok := sy.Vars[last]; ok && re.MatchString(genericless(t)) {
		return true
	}
	return false
}

// httpVerbOf is the method a client member sends: getForObject is GET,
// PostAsJsonAsync is POST.
func httpVerbOf(member string) string {
	m := verbMember.FindStringSubmatch(member)
	if m == nil {
		return ""
	}
	v := strings.ToUpper(m[1] + m[2] + m[3])
	if v == "DEL" {
		v = "DELETE"
	}
	return v
}

func mergeResilience(a, b string) string {
	set := map[string]bool{}
	for _, x := range strings.Split(a+","+b, ",") {
		if x != "" {
			set[x] = true
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

var (
	assignLine  = regexp.MustCompile(`^\s*(?:(?:private|public|protected|internal|readonly|static|final|const|let|var|val|export)\s+)*(?:[A-Z][\w<>.?]*\s+)?(?:self\s*\.\s*|this\s*\.\s*|\$this->|\$)?([A-Za-z_]\w*)\s*(?::\s*[\w<>?.|]+\s*)?(?::=|=)\s*(.+?)\s*;?\s*$`)
	objectField = regexp.MustCompile(`^\s*['"]?([A-Za-z_]\w*)['"]?\s*(?::|=>)\s*(.+?)\s*,?\s*$`)
)

// addresses records what fields and variables were assigned when it is an
// address, so a call made with the field renders the address it holds.
func addresses(s *source, sy *symbols) {
	for line := 1; line <= len(s.Lines); line++ {
		t := s.LineText(line)
		if !strings.ContainsAny(t, "=:") {
			continue
		}
		m := assignLine.FindStringSubmatch(t)
		if m == nil {
			m = objectField.FindStringSubmatch(t)
		}
		if m == nil || strings.HasPrefix(strings.TrimSpace(m[2]), "=") {
			continue
		}
		if _, done := sy.Addrs[m[1]]; done {
			continue
		}
		rendered := s.RenderAddr(m[2], sy)
		if strings.HasPrefix(rendered, "{env:") || strings.HasPrefix(rendered, "{config:") || strings.HasPrefix(rendered, "{prop:") || strings.Contains(rendered, "://") && !strings.ContainsAny(rendered, " \t") {
			sy.Addrs[strings.Clone(m[1])] = strings.Clone(rendered)
		}
	}
}

// ruleHint is a literal a rule's pattern cannot match without, so a file
// that lacks it is never searched: the patterns that start at any
// identifier are what a scan pays for.
func ruleHint(pattern *regexp.Regexp) string {
	re, err := syntax.Parse(pattern.String(), syntax.Perl)
	if err != nil {
		return ""
	}
	return longest(required(re.Simplify()))
}

// required are literals a match must contain.
func required(re *syntax.Regexp) []string {
	switch re.Op {
	case syntax.OpLiteral:
		if re.Flags&syntax.FoldCase != 0 {
			return nil
		}
		return []string{string(re.Rune)}
	case syntax.OpCapture, syntax.OpPlus:
		return required(re.Sub[0])
	case syntax.OpRepeat:
		if re.Min > 0 {
			return required(re.Sub[0])
		}
	case syntax.OpConcat:
		var out []string
		for _, sub := range re.Sub {
			out = append(out, required(sub)...)
		}
		return out
	}
	return nil
}

func longest(list []string) string {
	best := ""
	for _, l := range list {
		if len(l) > len(best) {
			best = l
		}
	}
	if len(best) < 3 {
		return ""
	}
	return best
}

// gateWords are the words a rule's receiver gate is made of, when it is
// nothing but words: a file without any of them has no receiver to gate.
func gateWords(gate string) []string {
	g := strings.TrimPrefix(gate, "(?i)")
	if g == "" || strings.ContainsAny(g, `^$()[]\.*+?{}`) {
		return nil
	}
	return strings.Split(strings.ToLower(g), "|")
}

func containsAny(text string, words []string) bool {
	for _, w := range words {
		if strings.Contains(text, w) {
			return true
		}
	}
	return false
}
