package recogniser

import (
	"regexp"
	"sort"
	"strings"
)

// A Mark is one annotation, attribute or decorator as written:
// `@GetMapping("/orders")`, `[HttpPost("place")]`, `#[Route('/x')]`,
// `@app.get("/items")`.
type Mark struct {
	// Name is the last part of the name: GetMapping, HttpPost, get.
	Name string
	// Full is the name as written, qualifier included: app.get, ORM\Entity.
	Full string
	Args []string
	Line int
}

// Arg is a mark's value: a named argument, or the first positional one. A
// value built from constants (`"/" + SECTION_KEY`) is kept with each
// constant in braces, so a route reads `/{SECTION_KEY}` rather than losing
// the part only the compiler knows.
func (m Mark) Arg(names ...string) string {
	if v := Named(m.Args, names...); v != "" {
		return v
	}
	pos := Positional(m.Args)
	if v := FirstString(pos); v != "" {
		if len(pos) > 0 && strings.Contains(pos[0], "+") {
			return Expression(pos[0])
		}
		return v
	}
	if len(pos) > 0 && pos[0] != "" && !strings.ContainsAny(pos[0][:1], "{[") {
		if strings.Contains(pos[0], "+") || IsQualified(pos[0]) && strings.ToUpper(LastName(pos[0])) == LastName(pos[0]) {
			return Expression(pos[0])
		}
	}
	return ""
}

// Text renders a mark for the auth column: its name and, when short, its
// arguments.
func (m Mark) Text() string {
	if len(m.Args) == 0 {
		return m.Name
	}
	a := strings.Join(strings.Fields(strings.Join(m.Args, ", ")), " ")
	if len(a) > 80 {
		a = a[:77] + "…"
	}
	return m.Name + "(" + a + ")"
}

// A Decl is what a run of marks is written on: a type or a member, its name,
// and the line its own text starts on.
type Decl struct {
	Marks []Mark
	// Class is true for a type declaration; Name is then the type's name.
	Class bool
	Name  string
	Line  int
	// Text is the declaration's first line, comments blanked.
	Text string
}

// Has is the first of the decl's marks with one of the names.
func (d Decl) Has(names ...string) *Mark {
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
	keywordsBeforeName = map[string]bool{"if": true, "for": true, "while": true, "switch": true, "catch": true, "return": true, "new": true, "function": true, "fun": true, "def": true, "func": true, "sizeof": true, "typeof": true}
)

// Decls reads every annotated declaration in a file.
func (s *Source) Decls() []Decl {
	if s.declsDone {
		return s.declsCache
	}
	s.declsDone = true
	switch s.syntax().Marks {
	case BracketMarks:
		s.declsCache = s.bracketDecls()
	case AtMarks:
		s.declsCache = s.atDecls()
	}
	return s.declsCache
}

// atDecls reads `@Name(args)` runs, the way Java, Kotlin, TypeScript and
// Python write them.
func (s *Source) atDecls() []Decl {
	text := s.Code
	var out []Decl
	strs := s.Literals()
	inString := func(offset int) bool {
		k := sort.Search(len(strs), func(k int) bool { return strs[k].End > offset })
		return k < len(strs) && strs[k].Offset < offset
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
		if start > 0 && (IsWordByte(text[start-1]) || text[start-1] == '"' || text[start-1] == '\'') || inString(start) {
			i = start + 1
			continue
		}
		var marks []Mark
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
			m := Mark{Full: full, Name: full[strings.LastIndexByte(full, '.')+1:], Line: s.Line(j)}
			j += loc[1]
			if j < len(text) && text[j] == '(' {
				inner, end := Balanced(text, j)
				m.Args = SplitArgs(inner)
				j = end
			}
			marks = append(marks, m)
		}
		if len(marks) == 0 {
			i = start + 1
			continue
		}
		d := Decl{Marks: marks, Line: s.Line(j)}
		d.Text = s.LineText(d.Line)
		rest := d.Text[min(len(d.Text), j-s.Lines[d.Line-1]):]
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
func (s *Source) bracketDecls() []Decl {
	text := s.Code
	x := s.syntax()
	var out []Decl
	opener := x.MarkOpen
	var pending []Mark
	pendingFrom := -1
	for line := 1; line <= len(s.Lines); line++ {
		t := strings.TrimSpace(s.LineText(line))
		if t == "" {
			continue
		}
		if strings.HasPrefix(t, opener) {
			from := s.Lines[line-1] + strings.Index(s.LineText(line), opener)
			inner, end := Balanced(text, from+len(opener)-1)
			for _, a := range SplitArgs(inner) {
				name, args := a, ""
				if k := strings.IndexByte(a, '('); k > 0 {
					name = a[:k]
					argText, _ := Balanced(a, k)
					args = argText
				}
				name = strings.TrimSpace(name)
				if k := strings.IndexByte(name, ':'); k > 0 && !strings.Contains(name, "::") {
					name = strings.TrimSpace(name[k+1:])
				}
				if !IsQualified(name) {
					continue
				}
				m := Mark{Full: name, Name: name[strings.LastIndexAny(name, `.\`)+1:], Line: line}
				if args != "" {
					m.Args = SplitArgs(args)
				}
				if x.MarkSuffix != "" {
					m.Name = strings.TrimSuffix(m.Name, x.MarkSuffix)
				}
				pending = append(pending, m)
			}
			if pendingFrom < 0 {
				pendingFrom = line
			}
			// The section may close on a later line, and a declaration may
			// follow it on the same line.
			endLine := s.Line(end - 1)
			after := strings.TrimSpace(text[end:min(len(text), s.LineEnd(endLine))])
			line = endLine
			if after == "" {
				continue
			}
			t = after
		}
		if len(pending) == 0 {
			if x.DocblockMarks {
				if doc := s.docblockMarks(line); len(doc) > 0 {
					pending = doc
				}
			}
			if len(pending) == 0 {
				continue
			}
		}
		d := Decl{Marks: pending, Line: line, Text: s.LineText(line)}
		d.Class, d.Name = s.classify(t, line)
		out = append(out, d)
		pending, pendingFrom = nil, -1
	}
	return out
}

var docMark = regexp.MustCompile(`@([A-Z][\w]*(?:\\[A-Z]\w*)*)(\()?`)

// docblockMarks are the annotations in the /** */ block that ends just above
// a line, as Doctrine and older Symfony write them.
func (s *Source) docblockMarks(line int) []Mark {
	if line < 2 {
		return nil
	}
	end := s.Lines[line-1]
	before := s.Raw[:end]
	close := strings.LastIndex(before, "*/")
	if close < 0 || strings.TrimSpace(before[close+2:]) != "" {
		return nil
	}
	open := strings.LastIndex(before[:close], "/**")
	if open < 0 {
		return nil
	}
	block := before[open:close]
	var out []Mark
	for _, loc := range docMark.FindAllStringSubmatchIndex(block, -1) {
		full := block[loc[2]:loc[3]]
		m := Mark{Full: full, Name: full[strings.LastIndexByte(full, '\\')+1:], Line: s.Line(open + loc[0])}
		if loc[4] >= 0 {
			inner, _ := Balanced(block, loc[4])
			m.Args = SplitArgs(inner)
		}
		out = append(out, m)
	}
	return out
}

// classify says whether the text a declaration starts with is a type, and
// its name or the member's.
func (s *Source) classify(rest string, line int) (bool, string) {
	t := rest
	if strings.TrimSpace(t) == "" {
		// The marks ran to the end of the line: the declaration is on the next
		// line that has code.
		for l := line + 1; l <= len(s.Lines) && l < line+4; l++ {
			if x := strings.TrimSpace(s.LineText(l)); x != "" {
				t = x
				break
			}
		}
	}
	if c := s.syntax().Classify; c != nil {
		return c(t)
	}
	return ClassifyCLike(t)
}

// ClassifyCLike reads a declaration written the way the C family writes
// one: a type keyword and a name, or a member's name before its parameters.
func ClassifyCLike(t string) (bool, string) {
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

// A TypeDecl is a type declared in a file, where, and its declaration line:
// what places a type unit on a line and gives it a signature, and what tells
// a member which type it sits in.
type TypeDecl struct {
	Name      string
	Line      int
	Signature string
}

var leadingMarks = regexp.MustCompile(`^\s*(?:(?:@[\w.]+(?:\([^()]*\))?|\[[\w.]+(?:\([^()]*\))?\]|#\[[^\]]*\])\s*)+`)

// CLikeTypeName is the type a line declares the way the C family declares
// one, marks before it dropped.
func CLikeTypeName(text string) string {
	t := leadingMarks.ReplaceAllString(text, "")
	if m := classDecl.FindStringSubmatch(t); m != nil {
		return m[2]
	}
	return ""
}

// StripMarks drops the marks a line starts with.
func StripMarks(text string) string { return leadingMarks.ReplaceAllString(text, "") }

// TypeDecls are the types the file declares, in order.
func (s *Source) TypeDecls() []TypeDecl {
	if s.typesDone {
		return s.types
	}
	s.typesDone = true
	typeName := s.syntax().TypeName
	if typeName == nil {
		return nil
	}
	for line := 1; line <= len(s.Lines); line++ {
		text := s.LineText(line)
		if strings.TrimSpace(text) == "" {
			continue
		}
		name := typeName(text)
		if name == "" {
			continue
		}
		s.types = append(s.types, TypeDecl{Name: name, Line: line, Signature: typeSignature(leadingMarks.ReplaceAllString(text, ""))})
	}
	return s.types
}

// typeSignature is a type's declaration line up to its body.
func typeSignature(text string) string {
	t := strings.TrimSpace(text)
	if i := strings.IndexByte(t, '{'); i >= 0 {
		t = t[:i]
	}
	t = strings.Join(strings.Fields(t), " ")
	if r := []rune(t); len(r) > 200 {
		t = string(r[:199]) + "…"
	}
	return strings.TrimSpace(t)
}

// TypeNear is the type declared on or just below a line: the class an
// annotation above it marks.
func (s *Source) TypeNear(line int) string {
	for _, t := range s.TypeDecls() {
		if t.Line >= line && t.Line <= line+4 {
			return t.Name
		}
	}
	return ""
}

// TypeAt is the type declared nearest above a line.
func (s *Source) TypeAt(line int) string {
	name := ""
	for _, t := range s.TypeDecls() {
		if t.Line <= line {
			name = t.Name
		}
	}
	return name
}
