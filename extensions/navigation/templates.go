package navigation

import (
	"regexp"
	"strings"
)

// Every framework writes a path parameter its own way: `{id}` (Spring,
// JAX-RS, ASP.NET, OpenAPI), `:id` (Express, Gin, Rails), `<int:id>`
// (Flask, Django's path()), `{id:int}` (ASP.NET constraints), `[id]` and
// `[...slug]` (Next, Nuxt, SvelteKit), `(?P<id>[0-9]+)` (Django's re_path),
// `${id}` (a template literal in a client). A template writes every one of
// them `{}`, so a route, a contract and a client call can be compared.

var (
	templateRegexGroup = regexp.MustCompile(`\(\?P?<\w+>[^()]*(?:\([^()]*\)[^()]*)*\)`)
	templateParam      = regexp.MustCompile(`^(?:\{[^{}]*\}|:[A-Za-z_]\w*\??|<[^<>]*>|\[\.{0,3}[^\[\]]*\]|\$\{[^{}]*\}|\*\*?|\{\{[^{}]*\}\})$`)
)

// pathTemplate is a route path with each parameter written `{}`, without
// its query string, its trailing slash, or a scheme and host.

func pathTemplate(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return ""
	}
	if i := strings.Index(p, "://"); i >= 0 {
		rest := p[i+3:]
		slash := strings.IndexByte(rest, '/')
		if slash < 0 {
			return "/"
		}
		p = rest[slash:]
	}
	p = templateRegexGroup.ReplaceAllString(p, "{}")
	if i := strings.IndexAny(p, "?#"); i >= 0 {
		p = p[:i]
	}
	p = strings.NewReplacer("^", "", "$/", "/").Replace(p)
	p = strings.TrimSuffix(p, "$")
	var out []string
	for _, seg := range strings.Split(p, "/") {
		seg = strings.TrimSpace(seg)
		if seg == "" {
			continue
		}
		switch {
		case templateParam.MatchString(seg):
			seg = "{}"
		case strings.Contains(seg, "{") && strings.Contains(seg, "}"):
			// `{id}.json`, `v{version}`: a parameter inside a segment.
			seg = rx(`\{[^{}]*\}`).ReplaceAllString(seg, "{}")
		case strings.Contains(seg, "${"):
			seg = rx(`\$\{[^{}]*\}`).ReplaceAllString(seg, "{}")
		}
		out = append(out, seg)
	}
	return "/" + strings.Join(out, "/")
}

// sameTemplate is whether two templates name the same path, a parameter
// matching any one segment, case ignored for the rest.
func sameTemplate(a, b string) bool {
	if a == b {
		return true
	}
	as, bs := strings.Split(a, "/"), strings.Split(b, "/")
	if len(as) != len(bs) {
		return false
	}
	for i := range as {
		if as[i] == "{}" || bs[i] == "{}" {
			continue
		}
		if !strings.EqualFold(as[i], bs[i]) {
			return false
		}
	}
	return true
}

// basePath is the path part of a server URL, `/` for none.
func basePath(server string) string {
	t := pathTemplate(server)
	if t == "" {
		return "/"
	}
	return t
}

// withBase is a template under a base path.
func withBase(base, template string) string {
	if base == "" || base == "/" {
		return template
	}
	if template == "/" {
		return base
	}
	return strings.TrimRight(base, "/") + template
}

// methodsMatch is whether a route answering one method list answers
// another: ANY answers everything.
func methodsMatch(a, b string) bool {
	if a == "" || b == "" || a == "ANY" || b == "ANY" {
		return true
	}
	for _, x := range strings.Split(a, ",") {
		for _, y := range strings.Split(b, ",") {
			if strings.EqualFold(strings.TrimSpace(x), strings.TrimSpace(y)) {
				return true
			}
		}
	}
	return false
}

// antMatch is whether a path template falls under a Spring Security or
// ASP.NET style pattern: `/admin/**`, `/api/*/orders`, `/public/{id}`.
func antMatch(pattern, template string) bool {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		return false
	}
	ps := strings.Split(strings.Trim(pathTemplate(strings.ReplaceAll(pattern, "**", "\x00")), "/"), "/")
	ts := strings.Split(strings.Trim(template, "/"), "/")
	var match func(i, j int) bool
	match = func(i, j int) bool {
		if i == len(ps) {
			return j == len(ts)
		}
		if ps[i] == "\x00" {
			for k := j; k <= len(ts); k++ {
				if match(i+1, k) {
					return true
				}
			}
			return false
		}
		if j == len(ts) {
			return false
		}
		if ps[i] == "*" || ps[i] == "{}" || ts[j] == "{}" || strings.EqualFold(ps[i], ts[j]) {
			return match(i+1, j+1)
		}
		if strings.HasSuffix(ps[i], "*") && strings.HasPrefix(strings.ToLower(ts[j]), strings.ToLower(strings.TrimSuffix(ps[i], "*"))) {
			return match(i+1, j+1)
		}
		return false
	}
	if len(ps) == 1 && ps[0] == "" {
		return template == "/"
	}
	return match(0, 0)
}
