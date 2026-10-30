package recogniser

import (
	"regexp"
	"strings"
	"sync"
)

// Balanced returns the text between the parenthesis (or bracket) at open
// and its match, strings respected, and the offset just past the match.
func Balanced(text string, open int) (string, int) {
	if open >= len(text) {
		return "", open
	}
	opener := text[open]
	closer := map[byte]byte{'(': ')', '[': ']', '{': '}', '<': '>'}[opener]
	depth := 0
	for i := open; i < len(text); i++ {
		switch text[i] {
		case '"', '\'', '`':
			i = SkipString(text, i) - 1
		case opener:
			depth++
		case closer:
			depth--
			if depth == 0 {
				return text[open+1 : i], i + 1
			}
		}
	}
	return text[open+1:], len(text)
}

// SplitArgs splits an argument list at its top-level commas.
func SplitArgs(args string) []string {
	var out []string
	depth, start := 0, 0
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case '"', '\'', '`':
			i = SkipString(args, i) - 1
		case '(', '[', '{', '<':
			depth++
		case ')', ']', '}', '>':
			if !(args[i] == '>' && i > 0 && (args[i-1] == '=' || args[i-1] == '-')) {
				depth--
			}
		case ',':
			if depth == 0 {
				out = append(out, strings.TrimSpace(args[start:i]))
				start = i + 1
			}
		}
	}
	if rest := strings.TrimSpace(args[start:]); rest != "" {
		out = append(out, rest)
	}
	return out
}

// SplitTop splits an expression at a separator outside strings and
// brackets.
func SplitTop(e, sep string) []string {
	var out []string
	depth, start := 0, 0
	for i := 0; i < len(e); i++ {
		switch e[i] {
		case '"', '\'', '`':
			i = SkipString(e, i) - 1
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			depth--
		default:
			if depth == 0 && strings.HasPrefix(e[i:], sep) {
				if sep == "." && (i+1 < len(e) && (e[i+1] >= '0' && e[i+1] <= '9' || e[i+1] == '.') || i > 0 && e[i-1] != ' ' && e[i+1] != ' ') {
					continue
				}
				out = append(out, strings.TrimSpace(e[start:i]))
				start = i + len(sep)
			}
		}
	}
	return append(out, strings.TrimSpace(e[start:]))
}

// Unquote is a string literal's text, or "" when the argument is not one.
// Python's r"", b"" and f"" prefixes and C#'s @"" are read through.
func Unquote(arg string) string {
	a := strings.TrimSpace(arg)
	a = strings.TrimLeft(a, "rbfuRBFU@$")
	if len(a) < 2 {
		return ""
	}
	q := a[0]
	if (q != '"' && q != '\'' && q != '`') || a[len(a)-1] != q {
		return ""
	}
	return strings.Trim(a, string(q))
}

// FirstString is the first string literal among arguments, positional or
// named, and in an array literal (`{"/a", "/b"}`, `["/a"]`) the first in it.
func FirstString(args []string) string {
	for _, a := range args {
		if v := Unquote(a); v != "" || IsEmptyString(a) {
			return v
		}
		t := strings.TrimSpace(a)
		if (strings.HasPrefix(t, "{") || strings.HasPrefix(t, "[")) && len(t) > 2 {
			for _, x := range SplitArgs(t[1 : len(t)-1]) {
				if v := Unquote(x); v != "" {
					return v
				}
			}
		}
	}
	return ""
}

func IsEmptyString(a string) bool {
	t := strings.TrimSpace(a)
	return t == `""` || t == "''"
}

// Named is the value of a named argument (`path = "/x"`, `methods: ['GET']`,
// `name="x"`), unquoted when it is a string, as written otherwise.
func Named(args []string, names ...string) string {
	for _, a := range args {
		for _, n := range names {
			for _, sep := range []string{"=", ":"} {
				prefix := n + sep
				t := strings.Join(strings.Fields(a), "")
				if strings.HasPrefix(t, prefix) || strings.HasPrefix(t, n+" "+sep) {
					v := strings.TrimSpace(a[strings.Index(a, sep)+1:])
					if u := Unquote(v); u != "" {
						return u
					}
					if IsEmptyString(v) {
						return "/"
					}
					if strings.HasPrefix(v, "{") || strings.HasPrefix(v, "[") {
						inner := SplitArgs(strings.Trim(v, "{}[]"))
						for _, x := range inner {
							if u := Unquote(x); u != "" {
								return u
							}
						}
						if len(inner) > 0 && !IsEmptyString(inner[0]) {
							return strings.TrimSpace(inner[0])
						}
						return "/"
					}
					if strings.Contains(v, "+") || IsQualified(v) && strings.ToUpper(LastName(v)) == LastName(v) {
						return Expression(v)
					}
					return v
				}
			}
		}
	}
	return ""
}

// Positional are the arguments that are not named.
func Positional(args []string) []string {
	var out []string
	for _, a := range args {
		t := strings.TrimSpace(a)
		if i := strings.IndexAny(t, "=:"); i > 0 && IsIdentifier(strings.TrimSpace(t[:i])) && !strings.HasPrefix(t[i:], "::") && !strings.HasPrefix(t[i:], "==") {
			continue
		}
		out = append(out, t)
	}
	return out
}

func IsIdentifier(s string) bool {
	if s == "" {
		return false
	}
	for i, r := range s {
		if !(r == '_' || r == '$' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || i > 0 && r >= '0' && r <= '9') {
			return false
		}
	}
	return true
}

func IsWordByte(c byte) bool {
	return c == '_' || c == '$' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9'
}

func IsQualified(name string) bool {
	if name == "" {
		return false
	}
	for _, part := range strings.FieldsFunc(name, func(r rune) bool { return r == '.' || r == '\\' }) {
		if !IsIdentifier(part) {
			return false
		}
	}
	return true
}

// LastName is the last identifier in a dotted, scoped or called expression:
// `views.OrderView.as_view()` is OrderView, `handlers.GetUser` is GetUser,
// `[OrderController::class, 'show']` is OrderController.show.
func LastName(expr string) string {
	e := strings.TrimSpace(expr)
	if strings.HasPrefix(e, "[") {
		parts := SplitArgs(strings.Trim(e, "[]"))
		if len(parts) == 2 {
			cls := strings.TrimSuffix(strings.TrimSpace(parts[0]), "::class")
			cls = cls[strings.LastIndexAny(cls, `\.`)+1:]
			if m := Unquote(parts[1]); m != "" {
				return cls + "." + m
			}
			return cls
		}
	}
	if u := Unquote(e); u != "" {
		// Laravel's 'OrderController@show'.
		if i := strings.Index(u, "@"); i > 0 {
			cls := u[:i]
			return cls[strings.LastIndexAny(cls, `\.`)+1:] + "." + u[i+1:]
		}
		return ""
	}
	for _, suffix := range []string{".as_view()", ".as_view", "::class"} {
		e = strings.TrimSuffix(e, suffix)
	}
	if i := strings.Index(e, ".as_view("); i > 0 {
		e = e[:i]
	}
	if i := strings.IndexByte(e, '('); i > 0 {
		e = e[:i]
	}
	e = strings.TrimLeft(e, "&*")
	if i := strings.LastIndexAny(e, `.\:`); i >= 0 {
		e = e[i+1:]
	}
	if !IsIdentifier(e) {
		return ""
	}
	return e
}

// Expression renders a value built from strings and constants:
// `"/" + SECTION_KEY + "/add"` is `/{SECTION_KEY}/add`.
func Expression(v string) string {
	var b strings.Builder
	for _, part := range strings.Split(v, "+") {
		part = strings.TrimSpace(part)
		if u := Unquote(part); u != "" || IsEmptyString(part) {
			b.WriteString(u)
			continue
		}
		if name := LastName(part); name != "" {
			b.WriteString("{" + name + "}")
		}
	}
	return b.String()
}

// ObjectKey is the value a named key holds in an object literal, keyword
// arguments, a Go struct literal or a PHP array: `baseURL: x`, `base_url=x`,
// `Topic: x`, `'base_uri' => x`.
func ObjectKey(body, key string) string {
	for _, part := range SplitArgs(body) {
		t := strings.TrimSpace(part)
		for _, sep := range []string{"=>", ":", "="} {
			i := strings.Index(t, sep)
			if i <= 0 {
				continue
			}
			k := strings.Trim(strings.TrimSpace(t[:i]), `"'`)
			if strings.EqualFold(k, key) {
				return strings.TrimSpace(t[i+len(sep):])
			}
			break
		}
	}
	return ""
}

// JoinRoute joins a prefix and a path into one route, each with one leading
// slash and none doubled; an empty path is the prefix itself.
func JoinRoute(prefix, p string) string {
	prefix, p = strings.TrimSpace(prefix), strings.TrimSpace(p)
	if prefix == "" && p == "" {
		return "/"
	}
	joined := strings.TrimRight(prefix, "/") + "/" + strings.TrimLeft(p, "/")
	if !strings.HasPrefix(joined, "/") {
		joined = "/" + joined
	}
	for strings.Contains(joined, "//") {
		joined = strings.ReplaceAll(joined, "//", "/")
	}
	if len(joined) > 1 {
		joined = strings.TrimRight(joined, "/")
	}
	return joined
}

func JoinIf(sep, s string) string {
	if s == "" {
		return ""
	}
	return sep + s
}

func Genericless(name string) string {
	if i := strings.IndexAny(name, "<[("); i > 0 {
		name = name[:i]
	}
	return strings.TrimSpace(name)
}

func LastSegment(recv string) string {
	r := strings.NewReplacer("->", ".", "?.", ".", "::", ".").Replace(recv)
	return r[strings.LastIndexByte(r, '.')+1:]
}

// NormalRecv drops the receiver's own-object prefix: `this.orders` and
// `self.orders` and `$this->orders` are all `orders`; `this` alone is "this".
func NormalRecv(r string) string {
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

var methodWord = Rx(`(?i)^(?:HttpMethod\s*\.\s*|Method\s*\.\s*|http\s*\.\s*Method|["'])?(get|post|put|patch|delete|head|options)["']?$`)

// MethodLiteral is the HTTP method an argument names: "GET",
// HttpMethod.Post, http.MethodGet, Method.Get.
func MethodLiteral(arg string) string {
	a := strings.TrimSpace(arg)
	if m := methodWord.FindStringSubmatch(a); m != nil {
		return strings.ToUpper(m[1])
	}
	return ""
}

func OrDefault(v, fallback string) string {
	if v == "" {
		return fallback
	}
	return v
}

func Contains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

func Itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}

var patterns sync.Map

// Rx is a pattern compiled once: the readers write their patterns where
// they use them, and a scan reads every file with them.
func Rx(pattern string) *regexp.Regexp {
	if re, ok := patterns.Load(pattern); ok {
		return re.(*regexp.Regexp)
	}
	re := regexp.MustCompile(pattern)
	patterns.Store(pattern, re)
	return re
}

// Authish is a name that says it guards access: a middleware, a wrapper.
var Authish = regexp.MustCompile(`(?i)auth|guard|login|permission|role|jwt|token|protect|secur|admin|session|verify|passport|acl`)

// NotCalls are words followed by a parenthesis that are not a call.
var NotCalls = map[string]bool{"if": true, "for": true, "foreach": true, "while": true, "switch": true, "catch": true, "return": true, "new": true, "function": true, "typeof": true, "sizeof": true, "super": true, "this": true, "assert": true, "synchronized": true, "using": true, "lock": true, "elif": true, "with": true, "not": true, "and": true, "or": true, "in": true, "await": true, "yield": true, "def": true, "fun": true, "func": true, "class": true, "struct": true, "when": true, "match": true, "case": true, "throw": true, "delete": true, "void": true, "print": true, "len": true, "make": true, "append": true, "isset": true, "empty": true, "array": true, "list": true, "require": true, "import": true, "fn": true, "go": true, "defer": true, "do": true, "else": true, "try": true, "except": true, "finally": true, "raise": true, "lambda": true, "async": true, "get": true, "set": true, "init": true, "where": true, "from": true, "select": true}
