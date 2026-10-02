package navigation

import (
	"regexp"
	"sort"
	"strings"
)

// A mark is one annotation, attribute or decorator as written:
// `@GetMapping("/orders")`, `[HttpPost("place")]`, `#[Route('/x')]`,
// `@app.get("/items")`.
type mark struct {
	// Name is the last part of the name: GetMapping, HttpPost, get.
	Name string
	// Full is the name as written, qualifier included: app.get, ORM\Entity.
	Full string
	Args []string
	Line int
}

// arg is a mark's value: a named argument, or the first positional one. A
// value built from constants (`"/" + SECTION_KEY`) is kept with each
// constant in braces, so a route reads `/{SECTION_KEY}` rather than losing
// the part only the compiler knows.
func (m mark) arg(names ...string) string {
	if v := named(m.Args, names...); v != "" {
		return v
	}
	pos := positional(m.Args)
	if v := firstString(pos); v != "" {
		if len(pos) > 0 && strings.Contains(pos[0], "+") {
			return expression(pos[0])
		}
		return v
	}
	if len(pos) > 0 && pos[0] != "" && !strings.ContainsAny(pos[0][:1], "{[") {
		if strings.Contains(pos[0], "+") || isQualified(pos[0]) && strings.ToUpper(lastName(pos[0])) == lastName(pos[0]) {
			return expression(pos[0])
		}
	}
	return ""
}

// expression renders a value built from strings and constants:
// `"/" + SECTION_KEY + "/add"` is `/{SECTION_KEY}/add`.
func expression(v string) string {
	var b strings.Builder
	for _, part := range strings.Split(v, "+") {
		part = strings.TrimSpace(part)
		if u := unquote(part); u != "" || isEmptyString(part) {
			b.WriteString(u)
			continue
		}
		if name := lastName(part); name != "" {
			b.WriteString("{" + name + "}")
		}
	}
	return b.String()
}

// A decl is what a run of marks is written on: a type or a member, its name,
// and the line its own text starts on.
type decl struct {
	Marks []mark
	// Class is true for a type declaration; Name is then the type's name.
	Class bool
	Name  string
	Line  int
	// Text is the declaration's first line, comments blanked.
	Text string
}

func (d decl) has(names ...string) *mark {
	for i := range d.Marks {
		for _, n := range names {
			if d.Marks[i].Name == n {
				return &d.Marks[i]
			}
		}
	}
	return nil
}

var (
	atMark             = regexp.MustCompile(`@([A-Za-z_][\w]*(?:\s*\.\s*[A-Za-z_]\w*)*(?::[A-Za-z_]\w*)?)`)
	classDecl          = regexp.MustCompile(`^\s*(?:(?:public|private|protected|internal|abstract|final|open|sealed|static|data|export|default|partial|readonly|inner|value|annotation|actual|expect|declare|async|enum|companion)\s+)*(class|interface|object|record|struct|enum|trait|protocol|extension)\s+([A-Za-z_]\w*)`)
	memberName         = regexp.MustCompile(`([A-Za-z_$][\w$]*)\s*(?:<[^()]*>)?\s*\(`)
	pyDef              = regexp.MustCompile(`^\s*(?:async\s+)?def\s+([A-Za-z_]\w*)`)
	pyClass            = regexp.MustCompile(`^\s*class\s+([A-Za-z_]\w*)`)
	keywordsBeforeName = map[string]bool{"if": true, "for": true, "while": true, "switch": true, "catch": true, "return": true, "new": true, "function": true, "fun": true, "def": true, "func": true, "sizeof": true, "typeof": true}
)

// decls reads every annotated declaration in a file.
func (s *source) decls() []decl {
	if s.declsDone {
		return s.declsCache
	}
	s.declsDone = true
	switch s.lang {
	case langCSharp, langPHP:
		s.declsCache = s.bracketDecls()
	case langJava, langKotlin, langScala, langJS, langPython, langDart, langSwift:
		s.declsCache = s.atDecls()
	}
	return s.declsCache
}

// atDecls reads `@Name(args)` runs, the way Java, Kotlin, TypeScript and
// Python write them.
func (s *source) atDecls() []decl {
	text := s.code
	var out []decl
	strs := s.literals()
	inString := func(offset int) bool {
		k := sort.Search(len(strs), func(k int) bool { return strs[k].end > offset })
		return k < len(strs) && strs[k].offset < offset
	}
	i := 0
	for i < len(text) {
		at := strings.IndexByte(text[i:], '@')
		if at < 0 {
			break
		}
		start := i + at
		// An @ inside an identifier or an email-like run is not a mark; in a
		// string it was left alone by the lexer, so check what precedes it.
		if start > 0 && (isWordByte(text[start-1]) || text[start-1] == '"' || text[start-1] == '\'') || inString(start) {
			i = start + 1
			continue
		}
		var marks []mark
		j := start
		for {
			for j < len(text) && (text[j] == ' ' || text[j] == '\t' || text[j] == '\n' || text[j] == '\r') {
				j++
			}
			if j >= len(text) || text[j] != '@' {
				break
			}
			loc := atMark.FindStringSubmatchIndex(text[j:])
			if loc == nil || loc[0] != 0 {
				break
			}
			full := strings.Join(strings.Fields(text[j+loc[2]:j+loc[3]]), "")
			if k := strings.IndexByte(full, ':'); k >= 0 {
				full = full[k+1:]
			}
			m := mark{Full: full, Name: full[strings.LastIndexByte(full, '.')+1:], Line: s.line(j)}
			j += loc[1]
			if j < len(text) && text[j] == '(' {
				inner, end := balanced(text, j)
				m.Args = splitArgs(inner)
				j = end
			}
			marks = append(marks, m)
		}
		if len(marks) == 0 {
			i = start + 1
			continue
		}
		d := decl{Marks: marks, Line: s.line(j)}
		d.Text = s.lineText(d.Line)
		rest := d.Text[min(len(d.Text), j-s.lines[d.Line-1]):]
		d.Class, d.Name = s.classify(rest, d.Line)
		out = append(out, d)
		i = max(j, start+1)
	}
	return out
}

// bracketDecls reads attribute sections: C#'s `[HttpGet("x"), Authorize]`
// and PHP's `#[Route('/x')]`, one or more before a declaration. PHP's
// docblock annotations (`@Route("/x")`, `@ORM\Entity`) are read from the
// docblock above the declaration.
func (s *source) bracketDecls() []decl {
	text := s.code
	var out []decl
	opener := "["
	if s.lang == langPHP {
		opener = "#["
	}
	var pending []mark
	pendingFrom := -1
	for line := 1; line <= len(s.lines); line++ {
		t := strings.TrimSpace(s.lineText(line))
		if t == "" {
			continue
		}
		if strings.HasPrefix(t, opener) {
			from := s.lines[line-1] + strings.Index(s.lineText(line), opener)
			inner, end := balanced(text, from+len(opener)-1)
			for _, a := range splitArgs(inner) {
				name, args := a, ""
				if k := strings.IndexByte(a, '('); k > 0 {
					name = a[:k]
					argText, _ := balanced(a, k)
					args = argText
				}
				name = strings.TrimSpace(name)
				if k := strings.IndexByte(name, ':'); k > 0 && !strings.Contains(name, "::") {
					name = strings.TrimSpace(name[k+1:])
				}
				if !isQualified(name) {
					continue
				}
				m := mark{Full: name, Name: name[strings.LastIndexAny(name, `.\`)+1:], Line: line}
				if args != "" {
					m.Args = splitArgs(args)
				}
				if s.lang == langCSharp {
					m.Name = strings.TrimSuffix(m.Name, "Attribute")
				}
				pending = append(pending, m)
			}
			if pendingFrom < 0 {
				pendingFrom = line
			}
			// The section may close on a later line, and a declaration may
			// follow it on the same line.
			endLine := s.line(end - 1)
			after := strings.TrimSpace(text[end:min(len(text), s.lineEnd(endLine))])
			line = endLine
			if after == "" {
				continue
			}
			t = after
		}
		if len(pending) == 0 {
			if s.lang == langPHP {
				if doc := s.docblockMarks(line); len(doc) > 0 {
					pending = doc
				}
			}
			if len(pending) == 0 {
				continue
			}
		}
		d := decl{Marks: pending, Line: line, Text: s.lineText(line)}
		d.Class, d.Name = s.classify(t, line)
		out = append(out, d)
		pending, pendingFrom = nil, -1
	}
	return out
}

func (s *source) lineEnd(line int) int {
	if line < len(s.lines) {
		return s.lines[line] - 1
	}
	return len(s.code)
}

var docMark = regexp.MustCompile(`@([A-Z][\w]*(?:\\[A-Z]\w*)*)(\()?`)

// docblockMarks are the annotations in the /** */ block that ends just above
// a line, as Doctrine and older Symfony write them.
func (s *source) docblockMarks(line int) []mark {
	if line < 2 {
		return nil
	}
	end := s.lines[line-1]
	before := s.raw[:end]
	close := strings.LastIndex(before, "*/")
	if close < 0 || strings.TrimSpace(before[close+2:]) != "" {
		return nil
	}
	open := strings.LastIndex(before[:close], "/**")
	if open < 0 {
		return nil
	}
	block := before[open:close]
	var out []mark
	for _, loc := range docMark.FindAllStringSubmatchIndex(block, -1) {
		full := block[loc[2]:loc[3]]
		m := mark{Full: full, Name: full[strings.LastIndexByte(full, '\\')+1:], Line: s.line(open + loc[0])}
		if loc[4] >= 0 {
			inner, _ := balanced(block, loc[4])
			m.Args = splitArgs(inner)
		}
		out = append(out, m)
	}
	return out
}

// classify says whether the text a declaration starts with is a type, and
// its name or the member's.
func (s *source) classify(rest string, line int) (bool, string) {
	t := rest
	if strings.TrimSpace(t) == "" {
		// The marks ran to the end of the line: the declaration is on the next
		// line that has code.
		for l := line + 1; l <= len(s.lines) && l < line+4; l++ {
			if x := strings.TrimSpace(s.lineText(l)); x != "" {
				t = x
				break
			}
		}
	}
	if s.lang == langPython {
		if m := pyClass.FindStringSubmatch(t); m != nil {
			return true, m[1]
		}
		if m := pyDef.FindStringSubmatch(t); m != nil {
			return false, m[1]
		}
		return false, ""
	}
	if m := classDecl.FindStringSubmatch(t); m != nil {
		return true, m[2]
	}
	for _, m := range memberName.FindAllStringSubmatch(t, -1) {
		if !keywordsBeforeName[m[1]] {
			return false, m[1]
		}
	}
	return false, ""
}

func isWordByte(c byte) bool {
	return c == '_' || c == '$' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9'
}

func isQualified(name string) bool {
	if name == "" {
		return false
	}
	for _, part := range strings.FieldsFunc(name, func(r rune) bool { return r == '.' || r == '\\' }) {
		if !isIdentifier(part) {
			return false
		}
	}
	return true
}
