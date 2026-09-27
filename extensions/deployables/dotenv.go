package deployables

import (
	"regexp"
	"strings"
)

// .env files do two jobs: compose reads the one beside it to interpolate the
// compose file itself, and services load them as their environment. The OTel
// demo is the case for the first: its images are `${IMAGE_NAME}:${DEMO_VERSION}-cart`,
// and without reading .env none of its 34 images join to anything.

// parseDotenv reads KEY=VALUE lines: `export` prefixes, quotes, comments and
// inline comments on unquoted values, and ${VAR} references to earlier keys.
func parseDotenv(content []byte) []keyValue {
	var out []keyValue
	known := map[string]string{}
	for i, raw := range strings.Split(strings.ReplaceAll(string(content), "\r\n", "\n"), "\n") {
		l := strings.TrimSpace(raw)
		if l == "" || strings.HasPrefix(l, "#") {
			continue
		}
		l = strings.TrimPrefix(l, "export ")
		k, v, ok := strings.Cut(l, "=")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		if k == "" || strings.ContainsAny(k, " \t") {
			continue
		}
		v = strings.TrimSpace(v)
		switch {
		case len(v) >= 2 && v[0] == '"' && strings.LastIndex(v, `"`) > 0:
			v = v[1:strings.LastIndex(v, `"`)]
			v = interpolate(v, known)
		case len(v) >= 2 && v[0] == '\'' && strings.LastIndex(v, `'`) > 0:
			// Single quotes are literal.
			v = v[1:strings.LastIndex(v, `'`)]
		default:
			if j := strings.Index(v, " #"); j >= 0 {
				v = strings.TrimSpace(v[:j])
			}
			v = interpolate(v, known)
		}
		known[k] = v
		out = append(out, keyValue{Key: k, Value: v, Line: i + 1})
	}
	return out
}

func dotenvMap(kvs []keyValue) map[string]string {
	m := make(map[string]string, len(kvs))
	for _, kv := range kvs {
		m[kv.Key] = kv.Value
	}
	return m
}

var interpolation = regexp.MustCompile(`\$\$|\$\{([A-Za-z_][A-Za-z0-9_]*)(?:(:?[-+?])([^}]*))?\}|\$([A-Za-z_][A-Za-z0-9_]*)`)

// interpolate applies compose's rules: ${X}, $X, ${X:-default}, ${X-default},
// ${X:+alt}, ${X:?error} and $$ for a literal dollar. A reference with no
// value and no default is left as written, so it stays visibly unresolved.
func interpolate(s string, vars map[string]string) string {
	if !strings.Contains(s, "$") {
		return s
	}
	return interpolation.ReplaceAllStringFunc(s, func(m string) string {
		if m == "$$" {
			return "$"
		}
		sub := interpolation.FindStringSubmatch(m)
		name, op, word := sub[1], sub[2], sub[3]
		if name == "" {
			name = sub[4]
		}
		v, set := vars[name]
		switch op {
		case ":-":
			if set && v != "" {
				return v
			}
			return interpolate(word, vars)
		case "-":
			if set {
				return v
			}
			return interpolate(word, vars)
		case ":+":
			if set && v != "" {
				return interpolate(word, vars)
			}
			return ""
		case "+":
			if set {
				return interpolate(word, vars)
			}
			return ""
		case ":?", "?":
			if set && (v != "" || op == "?") {
				return v
			}
			return m
		}
		if set {
			return v
		}
		return m
	})
}
