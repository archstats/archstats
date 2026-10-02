package navigation

import (
	"path/filepath"
	"sort"
	"strings"
)

// The languages the recognisers read, by what their source looks like rather
// than by grammar: where comments are and how strings are quoted is all a
// lexical reader needs to know.
const (
	langJava       = "java"
	langKotlin     = "kotlin"
	langScala      = "scala"
	langCSharp     = "csharp"
	langGo         = "go"
	langJS         = "js"
	langPython     = "python"
	langPHP        = "php"
	langSwift      = "swift"
	langDart       = "dart"
	langXML        = "xml"
	langYAML       = "yaml"
	langPrisma     = "prisma"
	langMarkdown   = "markdown"
	langUnreadable = ""
)

func languageOf(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".java":
		return langJava
	case ".kt", ".kts":
		return langKotlin
	case ".scala":
		return langScala
	case ".cs":
		return langCSharp
	case ".go":
		return langGo
	case ".js", ".jsx", ".mjs", ".cjs", ".ts", ".tsx", ".mts", ".cts", ".vue", ".svelte":
		return langJS
	case ".py":
		return langPython
	case ".php":
		return langPHP
	case ".swift":
		return langSwift
	case ".dart":
		return langDart
	case ".xml":
		return langXML
	case ".yaml", ".yml":
		return langYAML
	case ".prisma":
		return langPrisma
	case ".md", ".mdx", ".markdown", ".adoc", ".asciidoc", ".rst":
		return langMarkdown
	}
	return langUnreadable
}

// source is a file as the recognisers read it: its text with every comment
// blanked to spaces (newlines kept, so offsets and lines still match the
// file), the raw text for the few things written inside comments (PHP
// docblock annotations), and the start of each line for turning an offset
// into a line number.
type source struct {
	path  string
	lang  string
	raw   string
	code  string
	lines []int

	declsCache []decl
	declsDone  bool
	types      []typeDecl
	typesDone  bool
}

func newSource(path, lang string, content []byte) *source {
	raw := string(content)
	s := &source{path: path, lang: lang, raw: raw, code: blankComments(raw, lang)}
	s.lines = append(s.lines, 0)
	for i := 0; i < len(raw); i++ {
		if raw[i] == '\n' {
			s.lines = append(s.lines, i+1)
		}
	}
	return s
}

// line is the 1-based line an offset falls on.
func (s *source) line(offset int) int {
	return sort.Search(len(s.lines), func(i int) bool { return s.lines[i] > offset })
}

// lineText is the code on a 1-based line, comments blanked.
func (s *source) lineText(line int) string {
	if line < 1 || line > len(s.lines) {
		return ""
	}
	start := s.lines[line-1]
	end := len(s.code)
	if line < len(s.lines) {
		end = s.lines[line] - 1
	}
	return s.code[start:end]
}

// blankComments replaces comments with spaces and leaves strings alone, so a
// pattern can look for `@GetMapping("/orders")` without matching one that
// was commented out, and still read the path inside the string.
func blankComments(text, lang string) string {
	if lang == langMarkdown || lang == langUnreadable {
		return text
	}
	b := []byte(text)
	hash := lang == langPython || lang == langYAML || lang == langPHP
	slash := lang != langPython && lang != langYAML && lang != langXML
	blank := func(from, to int) {
		for i := from; i < to && i < len(b); i++ {
			if b[i] != '\n' {
				b[i] = ' '
			}
		}
	}
	i := 0
	for i < len(b) {
		c := b[i]
		switch {
		case lang == langXML && strings.HasPrefix(text[i:], "<!--"):
			end := strings.Index(text[i+4:], "-->")
			if end < 0 {
				end = len(text) - i - 4
			}
			blank(i, i+4+end+3)
			i += 4 + end + 3
		case slash && c == '/' && i+1 < len(b) && b[i+1] == '/':
			end := strings.IndexByte(text[i:], '\n')
			if end < 0 {
				end = len(text) - i
			}
			blank(i, i+end)
			i += end
		case slash && c == '/' && i+1 < len(b) && b[i+1] == '*':
			end := strings.Index(text[i+2:], "*/")
			if end < 0 {
				end = len(text) - i - 2
			}
			blank(i, i+2+end+2)
			i += 2 + end + 2
		case hash && c == '#' && !(lang == langPHP && i+1 < len(b) && b[i+1] == '['):
			end := strings.IndexByte(text[i:], '\n')
			if end < 0 {
				end = len(text) - i
			}
			blank(i, i+end)
			i += end
		case c == '"' || c == '\'' || c == '`':
			if lang == langXML || lang == langYAML || lang == langMarkdown {
				i++
				continue
			}
			i = skipString(text, i, lang)
		default:
			i++
		}
	}
	return string(b)
}

// skipString returns the offset just past the string literal starting at i:
// triple-quoted and backquoted strings may span lines, others end at the
// line's end if they are not closed.
func skipString(text string, i int, lang string) int {
	q := text[i]
	if (q == '"' || q == '\'') && strings.HasPrefix(text[i:], strings.Repeat(string(q), 3)) && (lang == langPython || lang == langJava || lang == langKotlin || lang == langScala || lang == langSwift || lang == langDart || lang == langCSharp) {
		end := strings.Index(text[i+3:], strings.Repeat(string(q), 3))
		if end < 0 {
			return len(text)
		}
		return i + 3 + end + 3
	}
	if q == '`' && lang != langJS && lang != langGo {
		return i + 1
	}
	if q == '\'' && (lang == langGo || lang == langJava || lang == langCSharp || lang == langKotlin || lang == langScala || lang == langSwift) {
		// A character literal, or an apostrophe in a language that has none
		// between quotes: never more than a few characters.
		end := strings.IndexByte(text[i+1:min(len(text), i+8)], '\'')
		if end < 0 {
			return i + 1
		}
		return i + 1 + end + 1
	}
	j := i + 1
	for j < len(text) {
		switch text[j] {
		case '\\':
			if q != '`' || lang == langJS {
				j += 2
				continue
			}
		case '\n':
			if q != '`' {
				return j
			}
		case q:
			return j + 1
		}
		j++
	}
	return j
}

// literal is a string in the code: its unquoted text and where it starts.
type literal struct {
	text   string
	offset int
	end    int
}

// literals are the file's string literals in order, read from the code with
// comments blanked. Raw, triple-quoted and template strings are included; an
// interpolation is left in the text as written.
func (s *source) literals() []literal {
	var out []literal
	text := s.code
	i := 0
	for i < len(text) {
		c := text[i]
		if c != '"' && c != '\'' && c != '`' {
			i++
			continue
		}
		if s.lang == langXML || s.lang == langYAML || s.lang == langMarkdown || s.lang == langUnreadable {
			i++
			continue
		}
		end := skipString(text, i, s.lang)
		q := 1
		if end-i >= 6 && (strings.HasPrefix(text[i:], `"""`) || strings.HasPrefix(text[i:], `'''`)) {
			q = 3
		}
		if end-q > i+q {
			out = append(out, literal{text: text[i+q : end-q], offset: i, end: end})
		} else {
			out = append(out, literal{offset: i, end: end})
		}
		i = max(end, i+1)
	}
	return out
}

// balanced returns the text between the parenthesis (or bracket) at open and
// its match, strings respected, and the offset just past the match.
func balanced(text string, open int) (string, int) {
	if open >= len(text) {
		return "", open
	}
	opener := text[open]
	closer := map[byte]byte{'(': ')', '[': ']', '{': '}', '<': '>'}[opener]
	depth := 0
	for i := open; i < len(text); i++ {
		switch text[i] {
		case '"', '\'', '`':
			i = skipString(text, i, langJS) - 1
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

// splitArgs splits an argument list at its top-level commas.
func splitArgs(args string) []string {
	var out []string
	depth, start := 0, 0
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case '"', '\'', '`':
			i = skipString(args, i, langJS) - 1
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

// unquote is a string literal's text, or "" when the argument is not one.
// Python's r"", b"" and f"" prefixes and C#'s @"" are read through.
func unquote(arg string) string {
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

// firstString is the first string literal among arguments, positional or
// named, and in an array literal (`{"/a", "/b"}`, `["/a"]`) the first in it.
func firstString(args []string) string {
	for _, a := range args {
		if v := unquote(a); v != "" || isEmptyString(a) {
			return v
		}
		t := strings.TrimSpace(a)
		if (strings.HasPrefix(t, "{") || strings.HasPrefix(t, "[")) && len(t) > 2 {
			for _, x := range splitArgs(t[1 : len(t)-1]) {
				if v := unquote(x); v != "" {
					return v
				}
			}
		}
	}
	return ""
}

func isEmptyString(a string) bool {
	t := strings.TrimSpace(a)
	return t == `""` || t == "''"
}

// named is the value of a named argument (`path = "/x"`, `methods: ['GET']`,
// `name="x"`), unquoted when it is a string, as written otherwise.
func named(args []string, names ...string) string {
	for _, a := range args {
		for _, n := range names {
			for _, sep := range []string{"=", ":"} {
				prefix := n + sep
				t := strings.Join(strings.Fields(a), "")
				if strings.HasPrefix(t, prefix) || strings.HasPrefix(t, n+" "+sep) {
					v := strings.TrimSpace(a[strings.Index(a, sep)+1:])
					if u := unquote(v); u != "" {
						return u
					}
					if isEmptyString(v) {
						return "/"
					}
					if strings.HasPrefix(v, "{") || strings.HasPrefix(v, "[") {
						inner := splitArgs(strings.Trim(v, "{}[]"))
						for _, x := range inner {
							if u := unquote(x); u != "" {
								return u
							}
						}
						if len(inner) > 0 && !isEmptyString(inner[0]) {
							return strings.TrimSpace(inner[0])
						}
						return "/"
					}
					if strings.Contains(v, "+") || isQualified(v) && strings.ToUpper(lastName(v)) == lastName(v) {
						return expression(v)
					}
					return v
				}
			}
		}
	}
	return ""
}

// positional are the arguments that are not named.
func positional(args []string) []string {
	var out []string
	for _, a := range args {
		t := strings.TrimSpace(a)
		if i := strings.IndexAny(t, "=:"); i > 0 && isIdentifier(strings.TrimSpace(t[:i])) && !strings.HasPrefix(t[i:], "::") && !strings.HasPrefix(t[i:], "==") {
			continue
		}
		out = append(out, t)
	}
	return out
}

func isIdentifier(s string) bool {
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

// lastName is the last identifier in a dotted, scoped or called expression:
// `views.OrderView.as_view()` is OrderView, `handlers.GetUser` is GetUser,
// `[OrderController::class, 'show']` is OrderController.show.
func lastName(expr string) string {
	e := strings.TrimSpace(expr)
	if strings.HasPrefix(e, "[") {
		parts := splitArgs(strings.Trim(e, "[]"))
		if len(parts) == 2 {
			cls := strings.TrimSuffix(strings.TrimSpace(parts[0]), "::class")
			cls = cls[strings.LastIndexAny(cls, `\.`)+1:]
			if m := unquote(parts[1]); m != "" {
				return cls + "." + m
			}
			return cls
		}
	}
	if u := unquote(e); u != "" {
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
	if !isIdentifier(e) {
		return ""
	}
	return e
}
