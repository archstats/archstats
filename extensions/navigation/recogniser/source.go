package recogniser

import (
	"sort"
	"strings"
)

// Syntax is what a lexical reader needs to know about a language: where its
// comments are, how its strings are quoted, how it writes marks and types,
// and how its strings interpolate.
type Syntax struct {
	// LineComments start a comment that runs to the end of the line: "//",
	// "#". NotComments are prefixes that look like one and are not: PHP's
	// "#[" opens an attribute.
	LineComments []string
	NotComments  []string
	// BlockComments open and close a comment: {"/*", "*/"}.
	BlockComments [][2]string
	// Strings says whether quotes delimit strings at all; in XML, YAML and
	// prose a quote is text.
	Strings bool
	// TripleQuotes are """ and ''' strings, which may span lines.
	TripleQuotes bool
	// CharLiterals are 'x': an apostrophe never opens more than a few
	// characters.
	CharLiterals bool
	Backticks    Backtick

	// Marks is how the language writes annotations; MarkOpen opens a
	// bracketed section ("[", "#["), MarkSuffix is dropped from a mark's
	// name ("Attribute"), and DocblockMarks reads annotations from the
	// /** */ block above a declaration.
	Marks         MarkStyle
	MarkOpen      string
	MarkSuffix    string
	DocblockMarks bool
	// Classify says whether the text a declaration starts with declares a
	// type, and its name or the member's; nil reads C-like declarations.
	Classify func(text string) (bool, string)
	// TypeName is the type declared on a line, or "". nil declares none.
	TypeName func(line string) string

	// BraceInterpolation: "{x}" in a string is an expression (Python's f"",
	// C#'s $"", PHP). DollarInterpolation: "$x" is one.
	BraceInterpolation  bool
	DollarInterpolation bool
	// DotConcat: strings are joined with ".".
	DotConcat bool
	// Statements is where a statement ends besides a semicolon.
	Statements StatementEnd
	// ExpressionBodies: a member's body may be `= expr` (Kotlin).
	ExpressionBodies bool
}

// Backtick is what a backquote opens.
type Backtick int

const (
	// NoBackticks: a backquote is a character.
	NoBackticks Backtick = iota
	// RawBackticks: a raw string that may span lines, no escapes (Go).
	RawBackticks
	// TemplateBackticks: a template string that may span lines, with
	// escapes (JavaScript).
	TemplateBackticks
)

// MarkStyle is how annotations are written.
type MarkStyle int

const (
	NoMarks MarkStyle = iota
	// AtMarks: `@Name(args)` runs (Java, Kotlin, TypeScript, Python).
	AtMarks
	// BracketMarks: attribute sections before a declaration (C#, PHP).
	BracketMarks
)

// StatementEnd is where a statement ends besides a semicolon or a blank
// line.
type StatementEnd int

const (
	SemicolonStatements StatementEnd = iota
	// NewlineStatements end at a line's end (Python).
	NewlineStatements
	// NewlineStatementsUnlessDot end at a line's end unless it ends with a
	// dot (Go's chained calls).
	NewlineStatementsUnlessDot
)

// exprSyntax reads an expression on its own: an argument list, a value. Its
// strings are the widest of any language's: three quotes, backquotes with
// escapes.
var exprSyntax = &Syntax{Strings: true, Backticks: TemplateBackticks}

// Source is a file as the recognisers read it: its text with every comment
// blanked to spaces (newlines kept, so offsets and lines still match the
// file), the raw text for the few things written inside comments (PHP
// docblock annotations), and the start of each line for turning an offset
// into a line number.
type Source struct {
	Path string
	// Lang is the language's name.
	Lang  string
	Raw   string
	Code  string
	Lines []int

	language   *Language
	declsCache []Decl
	declsDone  bool
	types      []TypeDecl
	typesDone  bool
}

// NewSource reads a file in a language.
func NewSource(path string, lang *Language, content []byte) *Source {
	raw := string(content)
	s := &Source{Path: path, Lang: lang.Name, Raw: raw, Code: lang.Syntax.BlankComments(raw), language: lang}
	s.Lines = append(s.Lines, 0)
	for i := 0; i < len(raw); i++ {
		if raw[i] == '\n' {
			s.Lines = append(s.Lines, i+1)
		}
	}
	return s
}

// Language is the language the file is read in.
func (s *Source) Language() *Language { return s.language }

func (s *Source) syntax() *Syntax { return &s.language.Syntax }

// Line is the 1-based line an offset falls on.
func (s *Source) Line(offset int) int {
	return sort.Search(len(s.Lines), func(i int) bool { return s.Lines[i] > offset })
}

// LineText is the code on a 1-based line, comments blanked.
func (s *Source) LineText(line int) string {
	if line < 1 || line > len(s.Lines) {
		return ""
	}
	start := s.Lines[line-1]
	end := len(s.Code)
	if line < len(s.Lines) {
		end = s.Lines[line] - 1
	}
	return s.Code[start:end]
}

// LineEnd is the offset a 1-based line ends at.
func (s *Source) LineEnd(line int) int {
	if line < len(s.Lines) {
		return s.Lines[line] - 1
	}
	return len(s.Code)
}

// BlankComments replaces comments with spaces and leaves strings alone, so a
// pattern can look for `@GetMapping("/orders")` without matching one that
// was commented out, and still read the path inside the string.
func (x *Syntax) BlankComments(text string) string {
	b := []byte(text)
	blank := func(from, to int) {
		for i := from; i < to && i < len(b); i++ {
			if b[i] != '\n' {
				b[i] = ' '
			}
		}
	}
	i := 0
next:
	for i < len(b) {
		c := b[i]
		for _, bc := range x.BlockComments {
			if c == bc[0][0] && strings.HasPrefix(text[i:], bc[0]) {
				end := strings.Index(text[i+len(bc[0]):], bc[1])
				if end < 0 {
					end = len(text) - i - len(bc[0])
				}
				blank(i, i+len(bc[0])+end+len(bc[1]))
				i += len(bc[0]) + end + len(bc[1])
				continue next
			}
		}
		for _, lc := range x.LineComments {
			if c == lc[0] && strings.HasPrefix(text[i:], lc) && !x.notComment(text[i:]) {
				end := strings.IndexByte(text[i:], '\n')
				if end < 0 {
					end = len(text) - i
				}
				blank(i, i+end)
				i += end
				continue next
			}
		}
		if (c == '"' || c == '\'' || c == '`') && x.Strings {
			i = x.SkipString(text, i)
			continue
		}
		i++
	}
	return string(b)
}

func (x *Syntax) notComment(rest string) bool {
	for _, n := range x.NotComments {
		if strings.HasPrefix(rest, n) {
			return true
		}
	}
	return false
}

// SkipString returns the offset just past the string literal starting at i:
// triple-quoted and backquoted strings may span lines, others end at the
// line's end if they are not closed.
func (x *Syntax) SkipString(text string, i int) int {
	q := text[i]
	if (q == '"' || q == '\'') && strings.HasPrefix(text[i:], strings.Repeat(string(q), 3)) && x.TripleQuotes {
		end := strings.Index(text[i+3:], strings.Repeat(string(q), 3))
		if end < 0 {
			return len(text)
		}
		return i + 3 + end + 3
	}
	if q == '`' && x.Backticks == NoBackticks {
		return i + 1
	}
	if q == '\'' && x.CharLiterals {
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
			if q != '`' || x.Backticks == TemplateBackticks {
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

// SkipString returns the offset just past the string literal starting at i
// in an expression on its own.
func SkipString(text string, i int) int { return exprSyntax.SkipString(text, i) }

// BlankComments blanks the comments of an expression or a JSON-like text.
func BlankComments(text string) string {
	return (&Syntax{LineComments: []string{"//"}, BlockComments: [][2]string{{"/*", "*/"}}, Strings: true, Backticks: TemplateBackticks}).BlankComments(text)
}

// A Literal is a string in the code: its unquoted text and where it starts.
type Literal struct {
	Text   string
	Offset int
	End    int
}

// Literals are the file's string literals in order, read from the code with
// comments blanked. Raw, triple-quoted and template strings are included; an
// interpolation is left in the text as written.
func (s *Source) Literals() []Literal {
	var out []Literal
	text := s.Code
	x := s.syntax()
	if !x.Strings {
		return nil
	}
	i := 0
	for i < len(text) {
		c := text[i]
		if c != '"' && c != '\'' && c != '`' {
			i++
			continue
		}
		end := x.SkipString(text, i)
		q := 1
		if end-i >= 6 && (strings.HasPrefix(text[i:], `"""`) || strings.HasPrefix(text[i:], `'''`)) {
			q = 3
		}
		if end-q > i+q {
			out = append(out, Literal{Text: text[i+q : end-q], Offset: i, End: end})
		} else {
			out = append(out, Literal{Offset: i, End: end})
		}
		i = max(end, i+1)
	}
	return out
}

// Body is the code of the member declared on a line: its braces, or for an
// expression body (Kotlin's `fun x() = X()`) the rest of the line.
func (s *Source) Body(line int) string {
	start := s.Lines[line-1]
	rest := s.Code[start:]
	brace := strings.IndexByte(rest, '{')
	eq := strings.Index(rest[:min(len(rest), max(brace, 0)+1)], "=")
	if brace < 0 || eq >= 0 && eq < brace && s.syntax().ExpressionBodies {
		return s.LineText(line)
	}
	inner, _ := Balanced(s.Code, start+brace)
	return inner
}

// Statement is the text of the statement around an offset, for reading a
// call's options: from the line's start to the end of the statement.
func (s *Source) Statement(at int) string {
	line := s.Line(at)
	start := s.Lines[line-1]
	return s.StatementFrom(start)
}

// StatementFrom is the text from an offset to the end of its statement: the
// first `;` or blank line at depth zero, at most 1,500 bytes.
func (s *Source) StatementFrom(at int) string {
	code := s.Code
	x := s.syntax()
	depth := 0
	limit := min(len(code), at+1500)
	for i := at; i < limit; i++ {
		switch code[i] {
		case '"', '\'', '`':
			i = x.SkipString(code, i) - 1
		case '(', '{', '[':
			depth++
		case ')', '}', ']':
			depth--
			if depth < 0 {
				return code[at:i]
			}
		case ';':
			if depth == 0 {
				return code[at:i]
			}
		case '\n':
			if depth == 0 && i+1 < len(code) && (code[i+1] == '\n' || x.Statements == NewlineStatements || x.Statements == NewlineStatementsUnlessDot && !strings.HasSuffix(strings.TrimSpace(code[at:i]), ".")) {
				next := strings.TrimSpace(code[i+1 : min(len(code), i+80)])
				if !strings.HasPrefix(next, ".") && !strings.HasPrefix(next, "->") {
					return code[at:i]
				}
			}
		}
	}
	return code[at:limit]
}
