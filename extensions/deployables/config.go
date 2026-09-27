package deployables

import (
	"net"
	"net/url"
	stdpath "path"
	"regexp"
	"strings"
)

// Runtime configuration is where one deployable says where another is. In
// the 26-repository client workspace it is the only record of how the
// services depend on each other: the Java imports end at each repository's
// edge, and `qp-document-storage` appears only as a host in a properties
// file.
//
// Values are read to find hosts, databases, brokers and topics, and nothing
// else of them is kept. A key that names a secret is recorded without its
// value, and a URL is reduced to its host, port and database.

// readProperties reads a Java .properties file: `key=value`, `key: value`
// and `key value`, `#` and `!` comments, and `\` continuations.
func readProperties(file string, content []byte) []configEntry {
	var out []configEntry
	lines := strings.Split(strings.ReplaceAll(string(content), "\r\n", "\n"), "\n")
	for i := 0; i < len(lines); i++ {
		start := i + 1
		l := strings.TrimLeft(lines[i], " \t\f")
		if l == "" || l[0] == '#' || l[0] == '!' {
			continue
		}
		for strings.HasSuffix(l, `\`) && !strings.HasSuffix(l, `\\`) && i+1 < len(lines) {
			i++
			l = l[:len(l)-1] + strings.TrimLeft(lines[i], " \t\f")
		}
		sep := strings.IndexAny(l, "=: \t")
		if sep < 0 {
			out = append(out, configEntry{Key: l, File: file, Line: start})
			continue
		}
		key := strings.TrimSpace(l[:sep])
		value := strings.TrimLeft(l[sep:], " \t")
		if value != "" && (value[0] == '=' || value[0] == ':') {
			value = value[1:]
		}
		out = append(out, configEntry{Key: key, Value: strings.TrimSpace(value), File: file, Line: start})
	}
	return out
}

// readConfigYAML flattens a Spring application.yml (or appsettings.json, which
// is YAML too) into dotted keys. A document that activates on a profile
// carries that profile in its keys' environment, via profileOf.
func readConfigYAML(file string, content []byte) ([]configEntry, map[int]string) {
	var out []configEntry
	profiles := map[int]string{}
	for _, doc := range yamlDocs(content) {
		profile := str(at(doc, "spring", "config", "activate", "on-profile"))
		if profile == "" {
			profile = str(at(doc, "spring", "profiles"))
		}
		for _, fv := range flatten(doc, "") {
			out = append(out, configEntry{Key: fv.Key, Value: fv.Value, File: file, Line: fv.Line})
			if profile != "" {
				profiles[fv.Line] = profile
			}
		}
	}
	return out, profiles
}

// readAppsettings flattens appsettings.json with ':' joined keys, as .NET
// reads them.
func readAppsettings(file string, content []byte) []configEntry {
	docs := yamlDocs(content)
	if len(docs) == 0 {
		return nil
	}
	var out []configEntry
	for _, fv := range flatten(docs[0], "") {
		out = append(out, configEntry{Key: fv.Key, Value: fv.Value, File: file, Line: fv.Line})
	}
	return out
}

// springProfileOf reads the profile from a file name: application-prod.yml
// is the prod profile.
func springProfileOf(file string) string {
	base := stdpath.Base(file)
	for _, prefix := range []string{"application-", "bootstrap-", "appsettings."} {
		if strings.HasPrefix(base, prefix) {
			rest := strings.TrimPrefix(base, prefix)
			if i := strings.LastIndex(rest, "."); i > 0 {
				return rest[:i]
			}
		}
	}
	return ""
}

var secretWords = regexp.MustCompile(`(?i)(pass(word|wd|phrase)?|pwd|secret|token|api[-_.]?key|private[-_.]?key|credential|access[-_.]?key|auth[-_.]?key|signing[-_.]?key|client[-_.]?secret|connection[-_.]?string|sas|cert(ificate)?[-_.]?key)`)

// isSecretKey says whether a configuration key names a secret, so its value
// is never kept.
func isSecretKey(key string) bool {
	if !secretWords.MatchString(key) {
		return false
	}
	// `valueFrom.secretKeyRef.name` and `existingSecret` name a secret kept
	// elsewhere; they hold none, and which secret a service uses is worth
	// seeing.
	lower := strings.ToLower(key)
	for _, ref := range []string{"secretkeyref.name", "secretkeyref.key", "secretref.name", "secretname", "existingsecret", "secret_name", "secret-name"} {
		if strings.HasSuffix(lower, ref) {
			return false
		}
	}
	return true
}

// An endpoint is what a configuration value points at.
type endpoint struct {
	Kind     string // http, datastore, broker, topic
	Vendor   string // postgresql, mongodb, redis, kafka, amqp, ...
	Host     string // normalised; "" for a topic
	Port     string
	Database string
	Topic    string
	// Internal: a bare name or a cluster-local address, which can name a
	// deployable. An external host is a fully qualified name.
	Internal bool
}

var (
	urlInValue      = regexp.MustCompile(`(?i)\b(mongodb(?:\+srv)?://[^\s'"]+|jdbc:[a-z0-9]+(?::[a-z]+)*:(?://|@//|@)[^\s,'"]+|(?:https?|grpcs?|wss?|lb|rediss?|amqps?|postgres(?:ql)?|mysql|mariadb|nats|kafka|mqtt|tcp|sqlserver|configserver:https?)://[^\s,'"]+)`)
	hostPortInValue = regexp.MustCompile(`(?i)(?:^|[\s,;=@])([a-z][a-z0-9-]*(?:\.[a-z0-9-]+)*):(\d{2,5})\b`)
	springDefault   = regexp.MustCompile(`\$\{[^:}]*:([^}]*)\}`)
	anyPlaceholder  = regexp.MustCompile(`\$\{[^}]*\}|\$\([^)]*\)|\{\{[^}]*\}\}`)
)

var datastoreSchemes = map[string]string{
	"postgres": "postgresql", "postgresql": "postgresql", "mysql": "mysql", "mariadb": "mariadb",
	"mongodb": "mongodb", "mongodb+srv": "mongodb", "redis": "redis", "rediss": "redis",
	"sqlserver": "sqlserver", "oracle": "oracle", "h2": "h2", "db2": "db2", "sqlite": "sqlite",
}

var brokerSchemes = map[string]string{
	"amqp": "amqp", "amqps": "amqp", "kafka": "kafka", "nats": "nats", "mqtt": "mqtt",
}

var addressKey = regexp.MustCompile(`(?i)(addr|address|addresses|host|hosts|hostname|url|uri|urls|endpoint|endpoints|server|servers|service|target|upstream|backend|dsn|zone|location|connect|connection|api)(\b|_|$|[^a-z])|(addr|address|host|url|uri|endpoint|server)s?$`)

var brokerKey = regexp.MustCompile(`(?i)(bootstrap[-_.]?servers|brokers?|kafka[-_.]?(host|url|server|address)|rabbit(mq)?[-_.]?(host|addresses)|amqp[-_.]?(host|url)|nats[-_.]?url|eventhub)`)

var topicLeaf = map[string]bool{
	"topic": true, "topics": true, "queue": true, "queues": true, "destination": true,
	"exchange": true, "subject": true, "subscription": true, "channel": true, "stream": true,
}

var topicPrefix = map[string]bool{"topic": true, "queue": true, "exchange": true, "subscription": true, "stream": true}

// Hosts that name the machine itself, or nothing.
var selfHosts = map[string]bool{
	"localhost": true, "127.0.0.1": true, "0.0.0.0": true, "host.docker.internal": true,
	"::1": true, "[::1]": true, "localhost.localdomain": true,
}

// normalizeHost reduces a host to what can name a deployable. It returns ""
// for the machine itself, an IP address, or documentation hosts.
func normalizeHost(h string) (string, bool) {
	h = strings.ToLower(strings.Trim(strings.TrimSpace(h), "[]."))
	if h == "" || selfHosts[h] || net.ParseIP(h) != nil {
		return "", false
	}
	for _, noise := range []string{"example.com", "example.org", "example.net", "w3.org", "schema.org", "json-schema.org", "xmlns.com"} {
		if h == noise || strings.HasSuffix(h, "."+noise) {
			return "", false
		}
	}
	for _, suffix := range []string{".svc.cluster.local", ".cluster.local", ".svc"} {
		if strings.HasSuffix(h, suffix) {
			first, _, _ := strings.Cut(h, ".")
			return first, true
		}
	}
	if !strings.Contains(h, ".") {
		return h, true
	}
	return h, false
}

// classify reads one configuration value for what it points at.
func classify(key, value string) []endpoint {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	// Spring's ${NAME:default} reads as its default; any other placeholder
	// is unknown and dropped rather than guessed.
	value = springDefault.ReplaceAllString(value, "$1")
	value = anyPlaceholder.ReplaceAllString(value, "")
	var out []endpoint
	lowerKey := strings.ToLower(key)

	if ts := topicsOf(lowerKey, value); len(ts) > 0 {
		for _, t := range ts {
			out = append(out, endpoint{Kind: "topic", Topic: t})
		}
		return out
	}

	urls := urlInValue.FindAllString(value, -1)
	for _, raw := range urls {
		if e, ok := parseEndpointURL(raw); ok {
			out = append(out, e)
		}
	}
	if len(urls) > 0 {
		return out
	}
	broker := brokerKey.MatchString(lowerKey)
	// A bare host:port is read only where the key says it holds an address;
	// `pickup:10` in a pricing rule is not a service called pickup.
	if !broker && !addressKey.MatchString(lowerKey) {
		return out
	}
	for _, m := range hostPortInValue.FindAllStringSubmatch(value, -1) {
		host, internal := normalizeHost(m[1])
		if host == "" {
			continue
		}
		e := endpoint{Kind: "http", Host: host, Port: m[2], Internal: internal}
		if broker {
			e.Kind, e.Vendor = "broker", "kafka"
			if strings.Contains(lowerKey, "rabbit") || strings.Contains(lowerKey, "amqp") {
				e.Vendor = "amqp"
			}
		}
		out = append(out, e)
	}
	return out
}

func parseEndpointURL(raw string) (endpoint, bool) {
	raw = strings.TrimRight(raw, ".;)")
	lower := strings.ToLower(raw)
	if strings.HasPrefix(lower, "configserver:") {
		raw = raw[len("configserver:"):]
		lower = lower[len("configserver:"):]
	}
	if strings.HasPrefix(lower, "jdbc:") {
		return parseJDBC(raw[len("jdbc:"):])
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return endpoint{}, false
	}
	scheme := strings.ToLower(u.Scheme)
	hostPart := u.Host
	// mongodb://a:27017,b:27017/db names a replica set; its first host is
	// enough to say which datastore it is.
	if i := strings.Index(hostPart, ","); i >= 0 {
		hostPart = hostPart[:i]
	}
	h, port, err := net.SplitHostPort(hostPart)
	if err != nil {
		h = hostPart
	}
	host, internal := normalizeHost(h)
	if host == "" {
		return endpoint{}, false
	}
	e := endpoint{Host: host, Port: port, Internal: internal}
	switch {
	case datastoreSchemes[scheme] != "":
		e.Kind, e.Vendor = "datastore", datastoreSchemes[scheme]
		e.Database = strings.Trim(u.Path, "/")
		if e.Vendor == "redis" {
			e.Database = ""
		}
	case brokerSchemes[scheme] != "":
		e.Kind, e.Vendor = "broker", brokerSchemes[scheme]
	default:
		e.Kind = "http"
		if scheme == "lb" {
			// Spring Cloud load-balanced names are service names.
			e.Internal = true
		}
	}
	return e, true
}

var (
	jdbcURL    = regexp.MustCompile(`(?i)^([a-z0-9]+)(?::[a-z]+)*://([^/:;?]+)(?::(\d+))?(?:/([^;?]*))?`)
	jdbcOracle = regexp.MustCompile(`(?i)^oracle:[a-z]+:@(?://)?([^:/]+)(?::(\d+))?[:/]([^?\s]+)`)
	jdbcMSDB   = regexp.MustCompile(`(?i)databaseName=([^;]+)`)
)

func parseJDBC(rest string) (endpoint, bool) {
	if m := jdbcOracle.FindStringSubmatch(rest); m != nil {
		host, internal := normalizeHost(m[1])
		if host == "" {
			return endpoint{}, false
		}
		return endpoint{Kind: "datastore", Vendor: "oracle", Host: host, Port: m[2], Database: m[3], Internal: internal}, true
	}
	m := jdbcURL.FindStringSubmatch(rest)
	if m == nil {
		return endpoint{}, false
	}
	host, internal := normalizeHost(m[2])
	if host == "" {
		return endpoint{}, false
	}
	vendor := strings.ToLower(m[1])
	if v := datastoreSchemes[vendor]; v != "" {
		vendor = v
	}
	db := m[4]
	if d := jdbcMSDB.FindStringSubmatch(rest); d != nil {
		db = d[1]
	}
	return endpoint{Kind: "datastore", Vendor: vendor, Host: host, Port: m[3], Database: db, Internal: internal}, true
}

var keySplit = regexp.MustCompile(`[._\-:\[\]]+`)

// topicsOf reads a value as topic or queue names when its key says it holds
// them: `topic.name`, `spring.cloud.stream.bindings.x.destination`,
// `ORDER_QUEUE`, `kafka.topics`.
func topicsOf(lowerKey, value string) []string {
	parts := keySplit.Split(lowerKey, -1)
	var clean []string
	for _, p := range parts {
		if p != "" {
			clean = append(clean, p)
		}
	}
	if len(clean) == 0 {
		return nil
	}
	last := clean[len(clean)-1]
	isTopic := topicLeaf[last]
	if !isTopic && (last == "name" || last == "names") && len(clean) >= 2 && topicPrefix[clean[len(clean)-2]] {
		isTopic = true
	}
	if !isTopic && len(clean) >= 2 && (last == "topic" || last == "queue") {
		isTopic = true
	}
	if !isTopic {
		for _, p := range clean {
			if strings.HasSuffix(p, "topic") || strings.HasSuffix(p, "queue") {
				if last != "id" && last != "group" && last != "partitions" && last != "replicas" && last != "enabled" && last != "count" && last != "size" && last != "timeout" {
					isTopic = strings.HasSuffix(last, "topic") || strings.HasSuffix(last, "queue") || last == "name"
				}
			}
		}
	}
	if !isTopic {
		return nil
	}
	var out []string
	for _, t := range strings.Split(value, ",") {
		t = strings.TrimSpace(t)
		if t == "" || strings.ContainsAny(t, " /\\") || strings.Contains(t, "://") {
			continue
		}
		switch strings.ToLower(t) {
		case "true", "false", "null", "none", "auto", "earliest", "latest":
			continue
		}
		if _, err := url.Parse(t); err != nil {
			continue
		}
		allDigits := true
		for _, r := range t {
			if r < '0' || r > '9' {
				allDigits = false
				break
			}
		}
		if allDigits {
			continue
		}
		out = append(out, t)
	}
	return out
}

// applicationName finds `spring.application.name` among entries.
func applicationName(entries []configEntry) string {
	for _, e := range entries {
		if e.Key == "spring.application.name" && e.Value != "" && !strings.Contains(e.Value, "${") {
			return e.Value
		}
	}
	return ""
}
