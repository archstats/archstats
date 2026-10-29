package navigation

import (
	"path"
	"regexp"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// Where a system goes out to is rarely in the code. The code says
// `@Value("${partner.base-url}")` or `config["Partner:BaseUrl"]` or
// `process.env.PARTNER_URL`; the address is in application-prod.yml, an
// appsettings file, a .env file, Helm values or compose. These readers keep
// only what names a place: URLs, hosts, connection strings, and the
// references between keys (`${PARTNER_URL:http://localhost:8081}`). Any
// other value is never kept, and a kept value loses its user and password.

// A cfgEntry is one key of runtime configuration that names a place, or
// refers to another key that might.
type cfgEntry struct {
	Key    string
	Value  string
	Env    string
	Format string
	Line   int
}

// A cfgTuning is a key that tunes how a client calls out: a timeout, a
// retry, a circuit breaker. Its value is not kept.
type cfgTuning struct {
	Key  string
	Kind string
	Line int
}

// A cfgService is a name other containers reach this one by: a compose
// service, a Kubernetes Service. A call to it stays in the workspace.
type cfgService struct {
	Name string
	Line int
}

type configFacts struct {
	entries  []cfgEntry
	tuning   []cfgTuning
	services []cfgService
}

var (
	secretKey     = regexp.MustCompile(`(?i)(secret|passw|pwd|token|api[-_.]?key|apikey|private[-_.]?key|credential|signing|salt|cert(ificate)?[-_.]?(data|content)|client[-_.]?secret|user(name)?$|login$|account[-_.]?key)`)
	placeKey      = regexp.MustCompile(`(?i)(ur[il]s?|endpoints?|hosts?|hostname|address(es)?|servers|brokers|bootstrap[-_.]?servers|base[-_.]?(url|uri|address|path)|connection[-_.]?strings?|dsn|authority|issuer|jwk[-_.]?set|wsdl|nodes|addrs?)$`)
	placeValue    = regexp.MustCompile(`(?i)^(?:[a-z][a-z0-9+.-]*://|jdbc:|(?:server|data source|host|address|addr|endpoint)\s*=|mongodb(\+srv)?:)`)
	hostPort      = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9.-]*:\d{2,5}(,[A-Za-z0-9][A-Za-z0-9.-]*:\d{2,5})*$`)
	reference     = regexp.MustCompile(`\$\{([A-Za-z_][\w.-]*)(?::-?([^}]*))?\}|\benv\(\s*['"]([A-Za-z_]\w*)['"](?:\s*,\s*['"]([^'"]*)['"])?\s*\)|\bos\.(?:environ\.get|getenv)\(\s*['"]([A-Za-z_]\w*)['"](?:\s*,\s*['"]([^'"]*)['"])?\s*\)|\bos\.environ\[\s*['"]([A-Za-z_]\w*)['"]\s*\]|\benv\.(?:str|url|db|cache_url)\(\s*['"]([A-Za-z_]\w*)['"]`)
	tuningKey     = regexp.MustCompile(`(?i)(connect|read|write|request|socket|response|call)?[-_.]?time[-_.]?out|retr(y|ies)|max[-_.]?attempts|circuit[-_.]?breaker|resilience4j|polly|backoff`)
	userInfo      = regexp.MustCompile(`(://)[^/@\s]+@`)
	connSecret    = regexp.MustCompile(`(?i)(password|pwd|user id|uid|user|username|accountkey|sharedaccesskey)\s*=\s*[^;]*;?`)
	urlSecretPart = regexp.MustCompile(`(?i)([?&](?:password|pwd|token|key|secret|sig|signature|access_token|apikey)=)[^&]*`)
)

// environmentNames are the deploy environments a file name can mean, DTAP
// and OTAP included, normalised.
var environmentNames = map[string]string{
	"local": "local", "dev": "dev", "develop": "dev", "development": "dev", "o": "dev", "ont": "dev", "ontwikkel": "dev",
	"test": "test", "tst": "test", "t": "test", "testing": "test", "qa": "test", "sit": "test",
	"acc": "acc", "acceptance": "acc", "accept": "acc", "acceptatie": "acc", "a": "acc", "uat": "acc", "staging": "acc", "stage": "acc", "stg": "acc", "preprod": "acc", "pre-prod": "acc",
	"prod": "prod", "production": "prod", "prd": "prod", "p": "prod", "productie": "prod", "live": "prod",
}

// defaults are the names of a file that holds every environment's values.
var defaults = map[string]bool{"": true, "default": true, "defaults": true, "base": true, "common": true, "shared": true, "example": true, "sample": true, "template": true, "dist": true, "defaults-example": true}

// environmentOf is the environment a configuration file is for, from its
// name (`application-prod.yml`, `appsettings.Production.json`, `.env.acc`,
// `values-p.yaml`) or a folder (`overlays/prod/`, `environments/acc/`).
// Empty means every environment.

func environmentOf(p string) string {
	base := strings.ToLower(path.Base(p))
	stem := base
	for _, prefix := range []string{"application", "bootstrap", "appsettings", "values", "settings", "docker-compose", "compose", "config"} {
		if strings.HasPrefix(stem, prefix) {
			stem = strings.TrimPrefix(stem, prefix)
			break
		}
	}
	stem = strings.TrimPrefix(stem, ".env")
	stem = strings.TrimSuffix(stem, path.Ext(stem))
	stem = strings.Trim(stem, ".-_")
	if name := strings.TrimSuffix(base, ".env"); stem == "" && name != base {
		// prod.env names its environment; config.env does not.
		if _, ok := environmentNames[name]; ok {
			stem = name
		}
	}
	if defaults[stem] {
		stem = ""
	}
	if stem != "" {
		if env, ok := environmentNames[stem]; ok {
			return env
		}
		if len(stem) > 1 {
			return stem
		}
	}
	for _, seg := range strings.Split(strings.ToLower(path.Dir(p)), "/") {
		if env, ok := environmentNames[seg]; ok && len(seg) > 1 {
			return env
		}
	}
	return ""
}

// configFormat is how a file holds configuration, or "" when it does not.
func configFormat(p, lang string) string {
	base := strings.ToLower(path.Base(p))
	ext := path.Ext(base)
	switch {
	case (strings.HasPrefix(base, "application") || strings.HasPrefix(base, "bootstrap")) && (ext == ".yml" || ext == ".yaml" || ext == ".properties"):
		return "spring"
	case strings.HasPrefix(base, "appsettings") && ext == ".json":
		return "dotnet"
	case ext == ".config" && (base == "web.config" || base == "app.config" || strings.HasSuffix(base, ".exe.config") || strings.HasPrefix(base, "web.") || strings.HasPrefix(base, "app.")):
		return "dotnet_xml"
	case base == ".env" || strings.HasPrefix(base, ".env.") || ext == ".env":
		return "dotenv"
	case (strings.HasPrefix(base, "docker-compose") || strings.HasPrefix(base, "compose")) && (ext == ".yml" || ext == ".yaml"):
		return "compose"
	case strings.HasPrefix(base, "values") && (ext == ".yml" || ext == ".yaml"):
		return "helm"
	case lang == langPHP && strings.Contains("/"+p, "/config/") && !strings.Contains(p, "/vendor/"):
		return "laravel"
	case lang == langPython && (strings.HasPrefix(base, "settings") || strings.Contains(p, "/settings/")):
		return "django"
	case ext == ".json" && strings.Contains("/"+p, "/config/") && environmentOf(p) != "" || ext == ".json" && strings.Contains("/"+p, "/config/") && base == "default.json":
		return "node"
	case lang == langYAML:
		return "yaml"
	}
	return ""
}

// readConfig reads one configuration file.
func readConfig(p, format string, content []byte) *configFacts {
	env := environmentOf(p)
	cf := &configFacts{}
	add := func(key, value string, line int, envOf string) {
		key = strings.TrimSpace(key)
		if key == "" {
			return
		}
		if tuningKey.MatchString(key) && !placeKey.MatchString(key) {
			cf.tuning = append(cf.tuning, cfgTuning{Key: key, Kind: tuningKind(key), Line: line})
			return
		}
		if v, ok := keepValue(key, value); ok {
			e := env
			if envOf != "" {
				e = envOf
			}
			cf.entries = append(cf.entries, cfgEntry{Key: key, Value: v, Env: e, Format: format, Line: line})
		}
	}
	text := string(content)
	switch format {
	case "spring":
		if strings.HasSuffix(strings.ToLower(p), ".properties") {
			readProperties(text, func(k, v string, line int) { add(k, v, line, "") })
			return cf
		}
		readYAML(content, ".", func(k, v string, line int, profile string) { add(k, v, line, profile) })
	case "dotnet":
		readYAML([]byte(stripJSONComments(text)), ":", func(k, v string, line int, _ string) { add(k, v, line, "") })
	case "node":
		readYAML(content, ".", func(k, v string, line int, _ string) { add(k, v, line, "") })
	case "helm":
		readYAML(content, ".", func(k, v string, line int, _ string) { add(k, v, line, "") })
	case "dotenv":
		readProperties(text, func(k, v string, line int) { add(strings.TrimPrefix(k, "export "), v, line, "") })
	case "dotnet_xml":
		readDotnetXML(text, add)
	case "laravel":
		readNested(text, "=>", strings.TrimSuffix(path.Base(p), ".php"), add)
	case "django":
		readNested(text, ":", "", add)
		for _, m := range pyAssign.FindAllStringSubmatchIndex(text, -1) {
			add(text[m[2]:m[3]], strings.TrimSpace(text[m[4]:m[5]]), strings.Count(text[:m[0]], "\n")+1, "")
		}
	case "compose", "yaml":
		readManifests(content, cf, add)
	}
	return cf
}

var pyAssign = rx(`(?m)^([A-Z][A-Z0-9_]*)\s*=\s*(.+)$`)

func tuningKind(key string) string {
	k := strings.ToLower(key)
	switch {
	case strings.Contains(k, "circuit"):
		return "circuit_breaker"
	case strings.Contains(k, "retr") || strings.Contains(k, "attempt") || strings.Contains(k, "backoff"):
		return "retry"
	case strings.Contains(k, "resilience4j") || strings.Contains(k, "polly"):
		return "retry"
	}
	return "timeout"
}

// keepValue is the value worth keeping for a key, sanitised, and whether to
// keep it at all.
func keepValue(key, value string) (string, bool) {
	v := strings.TrimSpace(strings.Trim(strings.TrimSpace(value), `"'`))
	if v == "" || len(v) > 600 {
		return "", false
	}
	place := placeKey.MatchString(key)
	ref := reference.MatchString(v)
	if ref && !place {
		// A reference is kept when it leads to a place: PARTNER_URL, or a
		// default that is an address.
		name, def, _ := refersTo(v)
		if !placeKey.MatchString(name) && !placeValue.MatchString(def) && !hostPort.MatchString(def) && !placeValue.MatchString(v) {
			return "", false
		}
	}
	if secretKey.MatchString(key) && !(place && (placeValue.MatchString(v) || ref)) {
		// A secret is never kept, nor a value under a key that might hold one,
		// unless the key names a place and the value is an address.
		return "", false
	}
	switch {
	case placeValue.MatchString(v), ref, hostPort.MatchString(v):
	case place && !strings.ContainsAny(v, " \t") && v != "true" && v != "false":
	default:
		return "", false
	}
	return sanitize(v), true
}

// sanitize takes the user, the password and any secret-looking query part
// out of an address.
func sanitize(v string) string {
	v = userInfo.ReplaceAllString(v, "$1")
	v = urlSecretPart.ReplaceAllString(v, "$1***")
	if strings.Contains(v, ";") && strings.Contains(v, "=") {
		v = strings.TrimSpace(connSecret.ReplaceAllString(v, ""))
	}
	return v
}

func readProperties(text string, add func(k, v string, line int)) {
	for i, raw := range strings.Split(text, "\n") {
		t := strings.TrimSpace(raw)
		if t == "" || t[0] == '#' || t[0] == '!' {
			continue
		}
		sep := strings.IndexAny(t, "=:")
		if sep <= 0 {
			continue
		}
		add(strings.TrimSpace(t[:sep]), strings.TrimSpace(t[sep+1:]), i+1)
	}
}

// readYAML flattens every document of a YAML (or JSON) file to keys joined
// with sep. A Spring document that says which profile it is for passes it
// on.
func readYAML(content []byte, sep string, add func(k, v string, line int, profile string)) {
	dec := yaml.NewDecoder(strings.NewReader(string(content)))
	for {
		var doc yaml.Node
		if err := dec.Decode(&doc); err != nil {
			return
		}
		if len(doc.Content) == 0 {
			continue
		}
		var flat []struct {
			k, v string
			line int
		}
		profile := ""
		flatten(doc.Content[0], "", sep, func(k, v string, line int) {
			lk := strings.ToLower(k)
			if lk == "spring.config.activate.on-profile" || lk == "spring.profiles" {
				if env, ok := environmentNames[strings.ToLower(v)]; ok {
					profile = env
				} else {
					profile = strings.ToLower(v)
				}
			}
			flat = append(flat, struct {
				k, v string
				line int
			}{k, v, line})
		})
		for _, f := range flat {
			add(f.k, f.v, f.line, profile)
		}
	}
}

func flatten(n *yaml.Node, prefix, sep string, add func(k, v string, line int)) {
	join := func(k string) string {
		if prefix == "" {
			return k
		}
		return prefix + sep + k
	}
	switch n.Kind {
	case yaml.MappingNode:
		for i := 0; i+1 < len(n.Content); i += 2 {
			flatten(n.Content[i+1], join(n.Content[i].Value), sep, add)
		}
	case yaml.SequenceNode:
		for i, c := range n.Content {
			// Kubernetes env lists: - name: X / value: Y.
			if c.Kind == yaml.MappingNode {
				name, value, line := "", "", c.Line
				for j := 0; j+1 < len(c.Content); j += 2 {
					switch c.Content[j].Value {
					case "name":
						name = c.Content[j+1].Value
					case "value":
						value, line = c.Content[j+1].Value, c.Content[j+1].Line
					}
				}
				if name != "" && value != "" {
					add(name, value, line)
					continue
				}
			}
			// Compose environment lists: - KEY=value.
			if c.Kind == yaml.ScalarNode && strings.Contains(c.Value, "=") && !strings.Contains(c.Value, " ") {
				kv := strings.SplitN(c.Value, "=", 2)
				add(kv[0], kv[1], c.Line)
				continue
			}
			flatten(c, join(strconv.Itoa(i)), sep, add)
		}
	case yaml.ScalarNode:
		add(prefix, n.Value, n.Line)
	case yaml.AliasNode:
		if n.Alias != nil {
			flatten(n.Alias, prefix, sep, add)
		}
	}
}

// readManifests reads compose files and Kubernetes manifests: environment
// values, ConfigMap data, and the service names that are internal hosts.
func readManifests(content []byte, cf *configFacts, add func(k, v string, line int, env string)) {
	dec := yaml.NewDecoder(strings.NewReader(string(content)))
	for {
		var doc yaml.Node
		if err := dec.Decode(&doc); err != nil {
			return
		}
		if len(doc.Content) == 0 || doc.Content[0].Kind != yaml.MappingNode {
			continue
		}
		root := doc.Content[0]
		get := func(n *yaml.Node, key string) *yaml.Node {
			if n == nil || n.Kind != yaml.MappingNode {
				return nil
			}
			for i := 0; i+1 < len(n.Content); i += 2 {
				if n.Content[i].Value == key {
					return n.Content[i+1]
				}
			}
			return nil
		}
		if services := get(root, "services"); services != nil && services.Kind == yaml.MappingNode {
			for i := 0; i+1 < len(services.Content); i += 2 {
				cf.services = append(cf.services, cfgService{Name: services.Content[i].Value, Line: services.Content[i].Line})
				if env := get(services.Content[i+1], "environment"); env != nil {
					flatten(env, "", ".", func(k, v string, line int) { add(k, v, line, "") })
				}
			}
			continue
		}
		kind := get(root, "kind")
		if kind == nil {
			continue
		}
		switch kind.Value {
		case "Service":
			if name := get(get(root, "metadata"), "name"); name != nil {
				cf.services = append(cf.services, cfgService{Name: name.Value, Line: name.Line})
			}
		case "ConfigMap":
			if data := get(root, "data"); data != nil {
				flatten(data, "", ".", func(k, v string, line int) { add(k, v, line, "") })
			}
		case "Deployment", "StatefulSet", "DaemonSet", "Job", "CronJob", "Pod":
			var walk func(n *yaml.Node)
			walk = func(n *yaml.Node) {
				if n == nil {
					return
				}
				if n.Kind == yaml.MappingNode {
					for i := 0; i+1 < len(n.Content); i += 2 {
						if n.Content[i].Value == "env" && n.Content[i+1].Kind == yaml.SequenceNode {
							flatten(n.Content[i+1], "", ".", func(k, v string, line int) { add(k, v, line, "") })
							continue
						}
						walk(n.Content[i+1])
					}
				}
				if n.Kind == yaml.SequenceNode {
					for _, c := range n.Content {
						walk(c)
					}
				}
			}
			walk(get(root, "spec"))
		}
	}
}

var (
	xmlAddKey   = regexp.MustCompile(`<add\b[^>]*\bkey\s*=\s*"([^"]+)"[^>]*\bvalue\s*=\s*"([^"]*)"`)
	xmlConn     = regexp.MustCompile(`<add\b[^>]*\bname\s*=\s*"([^"]+)"[^>]*\bconnectionString\s*=\s*"([^"]*)"`)
	xmlEndpoint = regexp.MustCompile(`<endpoint\b[^>]*>`)
)

func readDotnetXML(text string, add func(k, v string, line int, env string)) {
	line := func(off int) int { return strings.Count(text[:off], "\n") + 1 }
	for _, m := range xmlAddKey.FindAllStringSubmatchIndex(text, -1) {
		add(text[m[2]:m[3]], text[m[4]:m[5]], line(m[0]), "")
	}
	for _, m := range xmlConn.FindAllStringSubmatchIndex(text, -1) {
		add("ConnectionStrings:"+text[m[2]:m[3]], text[m[4]:m[5]], line(m[0]), "")
	}
	for _, loc := range xmlEndpoint.FindAllStringIndex(text, -1) {
		attrs := map[string]string{}
		for _, a := range rx(`\b(address|contract|name|binding)\s*=\s*"([^"]*)"`).FindAllStringSubmatch(text[loc[0]:loc[1]], -1) {
			attrs[a[1]] = a[2]
		}
		if attrs["address"] == "" {
			continue
		}
		name := attrs["name"]
		if name == "" {
			name = attrs["contract"]
		}
		add("system.serviceModel.client:"+name+":address", attrs["address"], line(loc[0]), "")
	}
}

// readNested reads a nested PHP array (`'url' => env('X')`) or Python dict
// (`'HOST': os.environ.get('X')`) into dotted keys, by following the
// brackets that open under a key.
func readNested(text, arrow, root string, add func(k, v string, line int, env string)) {
	keyed := regexp.MustCompile(`['"]([\w.-]+)['"]\s*` + regexp.QuoteMeta(arrow) + `\s*`)
	var stack []string
	depth := 0
	pushedAt := map[int]bool{}
	line := 1
	i := 0
	for i < len(text) {
		c := text[i]
		switch c {
		case '\n':
			line++
			i++
			continue
		case '#':
			if arrow == ":" {
				end := strings.IndexByte(text[i:], '\n')
				if end < 0 {
					return
				}
				i += end
				continue
			}
		case '[', '{', '(':
			depth++
			i++
			continue
		case ']', '}', ')':
			if pushedAt[depth] {
				pushedAt[depth] = false
				if len(stack) > 0 {
					stack = stack[:len(stack)-1]
				}
			}
			depth--
			i++
			continue
		case '\'', '"':
			loc := keyed.FindStringSubmatchIndex(text[i:])
			if loc != nil && loc[0] == 0 {
				key := text[i+loc[2] : i+loc[3]]
				j := i + loc[1]
				if j < len(text) && (text[j] == '[' || text[j] == '{' || strings.HasPrefix(text[j:], "array(")) {
					if strings.HasPrefix(text[j:], "array(") {
						j += 5
					}
					depth++
					pushedAt[depth] = true
					stack = append(stack, key)
					i = j + 1
					continue
				}
				end := j
				for end < len(text) && text[end] != ',' && text[end] != '\n' {
					if text[end] == '(' {
						_, e := balanced(text, end)
						end = e
						continue
					}
					end++
				}
				full := strings.Join(append(append([]string{}, stack...), key), ".")
				if root != "" {
					full = root + "." + full
				}
				add(full, strings.TrimSpace(text[j:end]), line, "")
				line += strings.Count(text[i:end], "\n")
				i = end
				continue
			}
			i = skipString(text, i, langJS)
			continue
		}
		i++
	}
}

// stripJSONComments removes // and /* */ comments outside strings, which
// .NET allows in appsettings files and JSON does not.
func stripJSONComments(text string) string {
	return blankComments(text, langJS)
}

// normKey is a configuration key as every binder sees it: Spring's relaxed
// binding, .NET's `:` and `__`, an environment variable's upper case and
// underscores. `partner.base-url`, `Partner:BaseUrl` and PARTNER_BASEURL
// are all `partnerbaseurl`.
func normKey(k string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(k) {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// refersTo is the key a value refers to and the default it carries.
func refersTo(v string) (string, string, bool) {
	m := reference.FindStringSubmatch(v)
	if m == nil {
		return "", "", false
	}
	for i := 1; i+1 < len(m); i += 2 {
		if m[i] != "" {
			return m[i], m[i+1], true
		}
	}
	if m[len(m)-1] != "" {
		return m[len(m)-1], "", true
	}
	return "", "", false
}
