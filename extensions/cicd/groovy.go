package cicd

import (
	"strings"
)

// Jenkinsfiles are Groovy, and Groovy is a programming language: nothing
// short of running one says for certain what it does. What is read here is
// the shape nearly every Jenkinsfile has -- calls with arguments and a
// closure, nested -- from a tokenizer that knows Groovy's strings, comments
// and brackets, and nothing else. No evaluation, no Groovy module.
//
//	pipeline { stages { stage('Build') { steps { sh 'mvn package' } } } }
//	node('linux') { stage('Test') { sh "./gradlew test" } }
//	parallel(unit: { … }, e2e: { … })
//
// all become trees of gNode. What the tree cannot say (a stage built in a
// loop, a parallel map filled at runtime) the Jenkins reader reports as such.

type gTokKind int

const (
	gIdent gTokKind = iota
	gString
	gNumber
	gPunct
	gNewline
	gEOF
)

type gTok struct {
	Kind gTokKind
	Text string // identifier, punctuation, or a string's value
	Raw  string // a string as written, quotes included
	Line int
	// For strings: true when the value holds ${…} or $name.
	Interp bool
}

// lexGroovy tokenizes Groovy. It never fails: an unterminated string or
// comment runs to the end of the file.
func lexGroovy(src string) []gTok {
	var toks []gTok
	line := 1
	n := len(src)
	i := 0
	// Groovy decides whether / starts a regex from what came before it.
	slashyOK := func() bool {
		if len(toks) == 0 {
			return true
		}
		t := toks[len(toks)-1]
		switch t.Kind {
		case gIdent, gString, gNumber:
			return t.Kind == gIdent && (t.Text == "return" || t.Text == "case")
		case gPunct:
			return t.Text != ")" && t.Text != "]" && t.Text != "}"
		}
		return true
	}
	for i < n {
		c := src[i]
		switch {
		case c == '\n':
			toks = append(toks, gTok{Kind: gNewline, Line: line})
			line++
			i++
		case c == ' ' || c == '\t' || c == '\r' || c == '\f':
			i++
		case c == '\\' && i+1 < n && src[i+1] == '\n':
			line++
			i += 2
		case c == '/' && i+1 < n && src[i+1] == '/':
			for i < n && src[i] != '\n' {
				i++
			}
		case c == '/' && i+1 < n && src[i+1] == '*':
			i += 2
			for i < n && !(src[i] == '*' && i+1 < n && src[i+1] == '/') {
				if src[i] == '\n' {
					line++
				}
				i++
			}
			i += 2
		case c == '#' && i == 0 && i+1 < n && src[i+1] == '!':
			for i < n && src[i] != '\n' {
				i++
			}
		case c == '\'' || c == '"':
			start, startLine := i, line
			quote := string(c)
			if i+2 < n && src[i+1] == c && src[i+2] == c {
				quote = strings.Repeat(string(c), 3)
			}
			i += len(quote)
			var val strings.Builder
			interp := false
			depth := 0
			for i < n {
				if depth == 0 && strings.HasPrefix(src[i:], quote) {
					i += len(quote)
					break
				}
				ch := src[i]
				if ch == '\\' && i+1 < n {
					if src[i+1] == '\n' {
						line++
					}
					val.WriteByte(src[i+1])
					i += 2
					continue
				}
				if ch == '\n' {
					line++
					if len(quote) == 1 && depth == 0 {
						// A single-quoted string never spans lines; this
						// one was not a string.
						break
					}
				}
				if c == '"' && ch == '$' && i+1 < n {
					interp = true
					if src[i+1] == '{' {
						depth++
						val.WriteString("${")
						i += 2
						continue
					}
				}
				if depth > 0 {
					if ch == '{' {
						depth++
					} else if ch == '}' {
						depth--
					}
				}
				val.WriteByte(ch)
				i++
			}
			toks = append(toks, gTok{Kind: gString, Text: val.String(), Raw: src[start:min(i, n)], Line: startLine, Interp: interp})
		case c == '$' && i+1 < n && src[i+1] == '/':
			// Dollar-slashy string: $/ … /$
			start, startLine := i, line
			i += 2
			for i < n && !(src[i] == '/' && i+1 < n && src[i+1] == '$') {
				if src[i] == '\n' {
					line++
				}
				i++
			}
			end := min(i, n)
			i += 2
			toks = append(toks, gTok{Kind: gString, Text: src[start+2 : end], Raw: src[start:min(i, n)], Line: startLine})
		case c == '/' && slashyOK() && i+1 < n && src[i+1] != ' ' && src[i+1] != '=':
			// Slashy string (a regex): / … /
			start, startLine := i, line
			i++
			for i < n && src[i] != '/' && src[i] != '\n' {
				if src[i] == '\\' {
					i++
				}
				i++
			}
			if i < n && src[i] == '/' {
				i++
				toks = append(toks, gTok{Kind: gString, Text: src[start+1 : i-1], Raw: src[start:i], Line: startLine})
			} else {
				// Not a regex after all: a division at the end of a line.
				i = start + 1
				toks = append(toks, gTok{Kind: gPunct, Text: "/", Line: startLine})
			}
		case isIdentStart(c):
			start := i
			for i < n && isIdentPart(src[i]) {
				i++
			}
			toks = append(toks, gTok{Kind: gIdent, Text: src[start:i], Line: line})
		case c >= '0' && c <= '9':
			start := i
			for i < n && (isIdentPart(src[i]) || src[i] == '.') {
				i++
			}
			toks = append(toks, gTok{Kind: gNumber, Text: src[start:i], Line: line})
		default:
			// Two-character operators the parser cares about: -> and ?.
			if i+1 < n && (src[i:i+2] == "->" || src[i:i+2] == "?." || src[i:i+2] == "*.") {
				toks = append(toks, gTok{Kind: gPunct, Text: src[i : i+2], Line: line})
				i += 2
				continue
			}
			toks = append(toks, gTok{Kind: gPunct, Text: string(c), Line: line})
			i++
		}
	}
	toks = append(toks, gTok{Kind: gEOF, Line: line})
	return toks
}

func isIdentStart(c byte) bool {
	return c == '_' || c == '$' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= 0x80
}

func isIdentPart(c byte) bool { return isIdentStart(c) || c >= '0' && c <= '9' }

// A gNode is one statement.
type gNode struct {
	// call: name(args) { block } in any of Groovy's spellings.
	// def: a method definition; Name is the method, Block its body.
	// assign: name = expression; Raw is the expression.
	// if, loop, try: control flow; Block holds every branch's statements.
	// other: anything else; Raw is its text.
	Kind string
	Name string
	Args []*gArg
	// A closure passed last: stage('x') { … }.
	Block []*gNode
	// For a call chained after the first (`docker.image('x').inside { … }`),
	// every name in the chain.
	Chain []string
	Raw   string
	Line  int
	End   int
	// For a loop: what it iterates, as written.
	Over string
}

// A gArg is one argument: positional or named, with its value as a string,
// a closure, a list or map, or an expression kept as text.
type gArg struct {
	Key string
	// The string's value, when the argument is a string literal.
	Str    string
	IsStr  bool
	Interp bool
	Raw    string
	// A closure argument's statements, or a list/map's entries.
	Block []*gNode
	Items []*gArg
	Line  int
}

type gParser struct {
	toks []gTok
	i    int
	src  string
}

// parseGroovy reads a file into statements.
func parseGroovy(src string) []*gNode {
	p := &gParser{toks: lexGroovy(src), src: src}
	return p.block(false)
}

func (p *gParser) peek() gTok {
	if p.i < len(p.toks) {
		return p.toks[p.i]
	}
	return gTok{Kind: gEOF}
}

func (p *gParser) peekAt(k int) gTok {
	if p.i+k < len(p.toks) {
		return p.toks[p.i+k]
	}
	return gTok{Kind: gEOF}
}

func (p *gParser) next() gTok {
	t := p.peek()
	if p.i < len(p.toks) {
		p.i++
	}
	return t
}

func (p *gParser) is(text string) bool {
	t := p.peek()
	return t.Kind == gPunct && t.Text == text
}

func (p *gParser) skipNewlines() {
	for {
		t := p.peek()
		if t.Kind == gNewline || t.Kind == gPunct && t.Text == ";" {
			p.i++
			continue
		}
		return
	}
}

// block reads statements until the closing brace (inBraces) or the end.
func (p *gParser) block(inBraces bool) []*gNode {
	var out []*gNode
	if inBraces {
		p.skipClosureParams()
	}
	for {
		p.skipNewlines()
		t := p.peek()
		if t.Kind == gEOF {
			return out
		}
		if t.Kind == gPunct && t.Text == "}" {
			if inBraces {
				p.i++
				return out
			}
			p.i++ // a stray brace at the top level
			continue
		}
		before := p.i
		if n := p.statement(); n != nil {
			out = append(out, n)
		}
		if p.i == before {
			p.i++ // never stall
		}
	}
}

var groovyTypes = map[string]bool{"void": true, "def": true, "String": true, "boolean": true, "int": true, "Boolean": true, "Integer": true, "Map": true, "List": true, "Object": true, "Closure": true, "static": true, "private": true, "public": true, "final": true, "long": true}

func (p *gParser) statement() *gNode {
	t := p.peek()
	line := t.Line
	switch {
	case t.Kind == gPunct && t.Text == "@":
		// An annotation: @Library('x') _ is a library load.
		p.next()
		name := p.next().Text
		n := &gNode{Kind: "call", Name: "@" + name, Chain: []string{"@" + name}, Line: line}
		if p.is("(") {
			n.Args = p.args("(", ")")
		}
		if name != "Library" {
			// @Field, @NonCPS, @Grab: the declaration that follows is what
			// counts.
			return nil
		}
		if t := p.peek(); t.Kind == gIdent && t.Text == "_" {
			p.next()
		}
		n.End = p.peek().Line
		return n
	case t.Kind == gPunct && t.Text == "{":
		// A bare block.
		p.next()
		return &gNode{Kind: "other", Block: p.block(true), Line: line, End: p.prevLine()}
	case t.Kind != gIdent:
		return p.expressionStatement(line)
	}
	switch t.Text {
	case "import", "package":
		p.skipToEOL()
		return nil
	case "class", "interface", "enum", "trait":
		// A class body is code the pipeline may call, not pipeline.
		p.next()
		for !p.is("{") && p.peek().Kind != gEOF {
			p.next()
		}
		n := &gNode{Kind: "def", Name: "class", Line: line}
		if p.is("{") {
			p.next()
			n.Block = p.block(true)
		}
		n.End = p.prevLine()
		return n
	case "new", "assert":
		return p.expressionStatement(line)
	case "if":
		return p.ifStatement()
	case "for", "while":
		p.next()
		over := ""
		if p.is("(") {
			over = p.rawGroup("(", ")")
		}
		n := &gNode{Kind: "loop", Name: t.Text, Over: strings.Trim(over, "()"), Line: line}
		n.Block = p.body()
		n.End = p.prevLine()
		return n
	case "try":
		p.next()
		n := &gNode{Kind: "try", Line: line}
		n.Block = p.body()
		for {
			p.skipNewlinesIfFollowedBy("catch", "finally")
			if t := p.peek(); t.Kind == gIdent && (t.Text == "catch" || t.Text == "finally") {
				p.next()
				if p.is("(") {
					p.rawGroup("(", ")")
				}
				n.Block = append(n.Block, p.body()...)
				continue
			}
			break
		}
		n.End = p.prevLine()
		return n
	case "switch":
		p.next()
		if p.is("(") {
			p.rawGroup("(", ")")
		}
		n := &gNode{Kind: "if", Line: line}
		n.Block = p.body()
		n.End = p.prevLine()
		return n
	case "return", "throw", "break", "continue", "case", "default":
		p.next()
		if t.Text == "case" || t.Text == "default" {
			for !p.is(":") && p.peek().Kind != gEOF && p.peek().Kind != gNewline {
				p.next()
			}
			if p.is(":") {
				p.next()
			}
			return nil
		}
		return p.expressionStatement(line)
	}
	// A method definition: [modifiers] [type] name(params) { body }, or a
	// variable: def x = …, String x = ….
	if groovyTypes[t.Text] || isTypeName(t.Text) && p.peekAt(1).Kind == gIdent {
		j := p.i
		for j < len(p.toks) && p.toks[j].Kind == gIdent && (groovyTypes[p.toks[j].Text] || isTypeName(p.toks[j].Text)) {
			j++
			// generic types: Map<String, Closure>
			if j < len(p.toks) && p.toks[j].Kind == gPunct && p.toks[j].Text == "<" {
				depth := 0
				for j < len(p.toks) {
					if p.toks[j].Text == "<" {
						depth++
					} else if p.toks[j].Text == ">" {
						depth--
						if depth == 0 {
							j++
							break
						}
					}
					j++
				}
			}
		}
		if j < len(p.toks) && p.toks[j].Kind == gIdent && j+1 < len(p.toks) {
			name := p.toks[j].Text
			after := p.toks[j+1]
			if after.Kind == gPunct && after.Text == "(" {
				// Is the group followed by a body?
				save := p.i
				p.i = j + 1
				p.rawGroup("(", ")")
				p.skipNewlines()
				if p.is("{") {
					n := &gNode{Kind: "def", Name: name, Line: line}
					n.Block = p.body()
					n.End = p.prevLine()
					return n
				}
				p.i = save
			} else if after.Kind == gPunct && after.Text == "=" || after.Kind == gNewline || after.Kind == gPunct && after.Text == ";" {
				p.i = j + 1
				n := &gNode{Kind: "assign", Name: name, Line: line}
				if p.is("=") {
					p.next()
					n.Raw, n.Args = p.expressionValue()
				}
				n.End = p.prevLine()
				return n
			} else if j > p.i && groovyTypes[p.toks[p.i].Text] {
				// def with a destructuring or something unusual: skip.
				p.i = j
			}
		}
	}
	return p.callOrExpression(line)
}

func isTypeName(s string) bool {
	return len(s) > 1 && s[0] >= 'A' && s[0] <= 'Z' && !strings.Contains(s, "_") && strings.ToUpper(s) != s
}

func (p *gParser) skipNewlinesIfFollowedBy(words ...string) {
	j := p.i
	for j < len(p.toks) && p.toks[j].Kind == gNewline {
		j++
	}
	if j < len(p.toks) && p.toks[j].Kind == gIdent {
		for _, w := range words {
			if p.toks[j].Text == w {
				p.i = j
				return
			}
		}
	}
}

func (p *gParser) ifStatement() *gNode {
	line := p.next().Line
	n := &gNode{Kind: "if", Line: line}
	if p.is("(") {
		n.Over = strings.Trim(p.rawGroup("(", ")"), "()")
	}
	n.Block = p.body()
	for {
		p.skipNewlinesIfFollowedBy("else")
		if t := p.peek(); t.Kind == gIdent && t.Text == "else" {
			p.next()
			if t := p.peek(); t.Kind == gIdent && t.Text == "if" {
				n.Block = append(n.Block, p.ifStatement().Block...)
				continue
			}
			n.Block = append(n.Block, p.body()...)
			continue
		}
		break
	}
	n.End = p.prevLine()
	return n
}

// body reads a braced block, or a single statement when there are no braces.
func (p *gParser) body() []*gNode {
	p.skipNewlines()
	if p.is("{") {
		p.next()
		return p.block(true)
	}
	if s := p.statement(); s != nil {
		return []*gNode{s}
	}
	return nil
}

// skipClosureParams skips `a, b ->` at the start of a closure.
func (p *gParser) skipClosureParams() {
	j := p.i
	for j < len(p.toks) && p.toks[j].Kind == gNewline {
		j++
	}
	k := j
	for k < len(p.toks) {
		t := p.toks[k]
		switch {
		case t.Kind == gPunct && t.Text == "->":
			if k > j || true {
				p.i = k + 1
			}
			return
		case t.Kind == gIdent, t.Kind == gPunct && (t.Text == "," || t.Text == "<" || t.Text == ">" || t.Text == "="), t.Kind == gString, t.Kind == gNumber:
			k++
		default:
			return
		}
	}
}

func (p *gParser) prevLine() int {
	if p.i > 0 && p.i-1 < len(p.toks) {
		return p.toks[p.i-1].Line
	}
	return 0
}

func (p *gParser) skipToEOL() {
	for {
		t := p.peek()
		if t.Kind == gNewline || t.Kind == gEOF {
			return
		}
		p.next()
	}
}

// callOrExpression reads a statement that starts with a name:
//
//	sh 'make'                      command syntax
//	sh(script: 'make')             parenthesised
//	stage('x') { … }               with a closure
//	steps { … }                    closure only
//	docker.image('x').inside { … } chained
//	env.X = 'y'                    assignment
//	foo.each { x -> … }            a loop
func (p *gParser) callOrExpression(line int) *gNode {
	start := p.i
	name := p.dottedName()
	if name == "" {
		p.i = start
		return p.expressionStatement(line)
	}
	n := &gNode{Kind: "call", Name: name, Chain: []string{name}, Line: line}
	switch {
	case p.is("="):
		p.next()
		n.Kind = "assign"
		n.Raw, n.Args = p.expressionValue()
		n.End = p.prevLine()
		return n
	case p.is("["):
		// executions["x"] = { … }: an assignment into a map; otherwise a
		// list passed in command syntax.
		save := p.i
		key := ""
		for p.is("[") {
			key += p.rawGroup("[", "]")
		}
		if p.is("=") || p.is("+") && p.peekAt(1).Text == "=" {
			if p.is("+") {
				p.next()
			}
			p.next()
			n.Kind = "assign"
			n.Name = name + key
			n.Raw, n.Args = p.expressionValue()
			n.End = p.prevLine()
			return n
		}
		p.i = save
	}
	// x += 1, x++, list << item: an expression, not a call.
	if t := p.peek(); t.Kind == gPunct && !strings.Contains("({.?*[=", t.Text) && t.Text != "->" && t.Text != "?." && t.Text != "*." && t.Text != "!" && t.Text != "-" {
		p.i = start
		return p.expressionStatement(line)
	}
loop:
	for {
		t := p.peek()
		switch {
		case p.is("("):
			n.Args = append(n.Args, p.args("(", ")")...)
		case p.is("{"):
			p.next()
			n.Block = append(n.Block, p.block(true)...)
		case p.is(".") || p.is("?.") || p.is("*."):
			p.next()
			nm := p.dottedName()
			if nm == "" {
				p.skipExpression()
				break loop
			}
			n.Chain = append(n.Chain, nm)
		case t.Kind == gNewline || t.Kind == gEOF || p.is("}") || p.is(";"):
			break loop
		case p.is("=") && len(n.Chain) > 1:
			// env.X = 'y'
			p.next()
			n.Kind = "assign"
			n.Name = strings.Join(n.Chain, ".")
			n.Raw, n.Args = p.expressionValue()
			n.End = p.prevLine()
			return n
		case len(n.Args) == 0 && n.Block == nil && p.looksLikeCommandArg():
			// Command syntax: sh 'make', echo "x", build job: 'y'
			before := p.i
			n.Args = p.commandArgs()
			if p.i == before {
				p.skipExpression()
				break loop
			}
		default:
			p.skipExpression()
			break loop
		}
	}
	n.Name = strings.Join(n.Chain, ".")
	if last := n.Chain[len(n.Chain)-1]; len(n.Chain) > 1 && loopMethods[last] {
		n.Kind = "loop"
		n.Over = strings.Join(n.Chain[:len(n.Chain)-1], ".")
	}
	n.End = p.prevLine()
	return n
}

var loopMethods = map[string]bool{"each": true, "eachWithIndex": true, "collect": true, "times": true, "findAll": true, "collectEntries": true, "upto": true, "step": true}

// looksLikeCommandArg says whether the token after a bare name starts an
// argument: a string, a name, a number, a list, or key: value.
func (p *gParser) looksLikeCommandArg() bool {
	t := p.peek()
	switch t.Kind {
	case gString, gNumber:
		return true
	case gIdent:
		return t.Text != "in" && t.Text != "instanceof" && t.Text != "as"
	case gPunct:
		return t.Text == "[" || t.Text == "-" || t.Text == "!"
	}
	return false
}

func (p *gParser) dottedName() string {
	t := p.peek()
	if t.Kind != gIdent {
		return ""
	}
	p.next()
	name := t.Text
	return name
}

// args reads a parenthesised or bracketed argument list.
func (p *gParser) args(open, close string) []*gArg {
	if !p.is(open) {
		return nil
	}
	p.next()
	var out []*gArg
	for {
		p.skipNewlines()
		if p.is(close) {
			p.next()
			return out
		}
		if p.peek().Kind == gEOF {
			return out
		}
		before := p.i
		a := p.arg(close)
		if a != nil {
			out = append(out, a)
		}
		p.skipNewlines()
		if p.is(",") {
			p.next()
		}
		if p.i == before {
			p.i++
		}
	}
}

// arg reads one argument up to a comma or the closing bracket.
func (p *gParser) arg(close string) *gArg {
	a := &gArg{Line: p.peek().Line}
	// key: value
	if t, c := p.peek(), p.peekAt(1); (t.Kind == gIdent || t.Kind == gString) && c.Kind == gPunct && c.Text == ":" {
		a.Key = t.Text
		p.next()
		p.next()
		p.skipNewlines()
	}
	start := p.i
	t := p.peek()
	switch {
	case t.Kind == gString && (p.endsArg(1, close) || close == "\n" && p.endsLine(1)):
		p.next()
		a.Str, a.IsStr, a.Interp, a.Raw = t.Text, true, t.Interp, t.Raw
		return a
	case t.Kind == gPunct && t.Text == "{":
		p.next()
		a.Block = p.block(true)
		a.Raw = "{…}"
		if p.endsArg(0, close) {
			return a
		}
	case t.Kind == gPunct && t.Text == "[":
		a.Items = p.args("[", "]")
		a.Raw = "[…]"
		if p.endsArg(0, close) {
			return a
		}
	}
	p.i = start
	depth := 0
	defer func() { a.Raw = joinToks(p.toks[start:p.i]) }()
	var sb strings.Builder
	for {
		t := p.peek()
		if t.Kind == gEOF {
			break
		}
		if depth == 0 && t.Kind == gPunct && (t.Text == "," || t.Text == close) {
			break
		}
		if depth == 0 && close == "\n" && (t.Kind == gNewline || t.Kind == gPunct && (t.Text == "{" || t.Text == ";")) {
			break
		}
		if t.Kind == gPunct {
			switch t.Text {
			case "(", "[", "{":
				depth++
			case ")", "]", "}":
				depth--
				if depth < 0 {
					a.Raw = strings.TrimSpace(sb.String())
					return a
				}
			}
		}
		p.next()
		if t.Kind == gNewline {
			sb.WriteByte(' ')
			continue
		}
		if t.Kind == gString {
			sb.WriteString(t.Raw)
		} else {
			sb.WriteString(t.Text)
		}
		if t.Kind == gIdent || t.Kind == gNumber {
			sb.WriteByte(' ')
		}
	}
	a.Raw = strings.TrimSpace(sb.String())
	return a
}

func (p *gParser) endsLine(k int) bool {
	j := p.i + k
	if j >= len(p.toks) {
		return true
	}
	t := p.toks[j]
	return t.Kind == gNewline || t.Kind == gEOF || t.Kind == gPunct && (t.Text == "," || t.Text == "}" || t.Text == ";" || t.Text == "{")
}

func (p *gParser) endsArg(k int, close string) bool {
	j := p.i + k
	for j < len(p.toks) && p.toks[j].Kind == gNewline {
		j++
	}
	if j >= len(p.toks) {
		return true
	}
	t := p.toks[j]
	return t.Kind == gPunct && (t.Text == "," || t.Text == close)
}

// commandArgs reads command-syntax arguments up to the end of the line.
func (p *gParser) commandArgs() []*gArg {
	var out []*gArg
	for {
		t := p.peek()
		if t.Kind == gNewline || t.Kind == gEOF || t.Kind == gPunct && (t.Text == "}" || t.Text == ";" || t.Text == "{") {
			return out
		}
		before := p.i
		a := p.arg("\n")
		if a != nil {
			out = append(out, a)
		}
		if p.is(",") {
			p.next()
			p.skipNewlines()
		}
		if p.i == before {
			p.next()
		}
		// arg stops at "," or a newline token only through depth, so a
		// command ends at the line end.
		if t := p.peek(); t.Kind == gNewline {
			return out
		}
	}
}

// expressionValue reads the right side of an assignment: a closure keeps its
// statements, a list or map its entries, and anything else its text.
func (p *gParser) expressionValue() (string, []*gArg) {
	p.skipNewlines()
	switch {
	case p.is("{"):
		line := p.peek().Line
		p.next()
		return "{…}", []*gArg{{Block: p.block(true), Raw: "{…}", Line: line}}
	case p.is("["):
		items := p.args("[", "]")
		return "[…]", items
	}
	start := p.i
	p.skipExpression()
	return joinToks(p.toks[start:p.i]), nil
}

// skipExpression moves to the end of the statement: a newline or ; at depth
// zero, or the brace that closes the enclosing block. A closure inside is
// skipped whole.
func (p *gParser) skipExpression() {
	depth := 0
	for {
		t := p.peek()
		switch {
		case t.Kind == gEOF:
			return
		case depth == 0 && (t.Kind == gNewline || t.Kind == gPunct && (t.Text == ";" || t.Text == "}")):
			// A line ending in an operator continues.
			if t.Kind == gNewline && p.i > 0 {
				prev := p.toks[p.i-1]
				postfix := (prev.Text == "+" || prev.Text == "-") && p.i >= 2 && p.toks[p.i-2].Text == prev.Text
				if prev.Kind == gPunct && strings.Contains("+-*&|?:=,.(", prev.Text) && prev.Text != ")" && !postfix {
					p.next()
					continue
				}
				// .method() on the next line continues the chain.
				j := p.i
				for j < len(p.toks) && p.toks[j].Kind == gNewline {
					j++
				}
				if j < len(p.toks) && p.toks[j].Kind == gPunct && (p.toks[j].Text == "." || p.toks[j].Text == "?.") {
					p.i = j
					continue
				}
			}
			return
		case t.Kind == gPunct && (t.Text == "(" || t.Text == "[" || t.Text == "{"):
			depth++
		case t.Kind == gPunct && (t.Text == ")" || t.Text == "]" || t.Text == "}"):
			depth--
			if depth < 0 {
				return
			}
		}
		p.next()
	}
}

func (p *gParser) expressionStatement(line int) *gNode {
	start := p.i
	p.skipExpression()
	return &gNode{Kind: "other", Raw: joinToks(p.toks[start:p.i]), Line: line, End: p.prevLine()}
}

// rawGroup reads a balanced group and returns its text.
func (p *gParser) rawGroup(open, close string) string {
	if !p.is(open) {
		return ""
	}
	start := p.i
	depth := 0
	for {
		t := p.peek()
		if t.Kind == gEOF {
			return joinToks(p.toks[start:p.i])
		}
		p.next()
		if t.Kind == gPunct {
			if t.Text == open {
				depth++
			} else if t.Text == close {
				depth--
			}
		}
		if depth == 0 {
			return joinToks(p.toks[start:p.i])
		}
	}
}

// joinToks writes tokens back as text, a space only between words.
func joinToks(toks []gTok) string {
	var sb strings.Builder
	prevWord := false
	for _, t := range toks {
		word := t.Kind == gIdent || t.Kind == gNumber || t.Kind == gString
		switch t.Kind {
		case gNewline:
			if sb.Len() > 0 {
				sb.WriteByte(' ')
			}
			prevWord = false
			continue
		case gString:
			if prevWord {
				sb.WriteByte(' ')
			}
			sb.WriteString(t.Raw)
		default:
			if word && prevWord {
				sb.WriteByte(' ')
			}
			if t.Kind == gPunct && (t.Text == "," || t.Text == ":") {
				sb.WriteString(t.Text + " ")
				prevWord = false
				continue
			}
			sb.WriteString(t.Text)
		}
		prevWord = word
	}
	return strings.Join(strings.Fields(sb.String()), " ")
}

// argOf looks up a named argument, falling back to the n-th positional one
// (n < 0: no fallback).
func argOf(args []*gArg, key string, n int) *gArg {
	if key != "" {
		for _, a := range args {
			if a.Key == key {
				return a
			}
		}
	}
	pos := 0
	for _, a := range args {
		if a.Key == "" {
			if pos == n {
				return a
			}
			pos++
		}
	}
	return nil
}

// text is what an argument says: a string's value, or its text.
func (a *gArg) text() string {
	if a == nil {
		return ""
	}
	if a.IsStr {
		return a.Str
	}
	return a.Raw
}

// walkGroovy visits every node, depth first, closures and arguments included.
func walkGroovy(nodes []*gNode, f func(n *gNode, parents []*gNode)) {
	var walk func(ns []*gNode, parents []*gNode)
	walk = func(ns []*gNode, parents []*gNode) {
		for _, n := range ns {
			f(n, parents)
			next := append(append([]*gNode{}, parents...), n)
			walk(n.Block, next)
			for _, a := range n.Args {
				walkArg(a, next, walk)
			}
		}
	}
	walk(nodes, nil)
}

func walkArg(a *gArg, parents []*gNode, walk func([]*gNode, []*gNode)) {
	walk(a.Block, parents)
	for _, it := range a.Items {
		walkArg(it, parents, walk)
	}
}

// localNames lists the names a file defines for itself: variables, method
// and closure parameters, loop variables. A call on one of them is Groovy;
// a call on any other name is a library's global variable.
func localNames(src string) map[string]bool {
	toks := lexGroovy(src)
	out := map[string]bool{"this": true, "it": true, "super": true, "owner": true, "delegate": true}
	for i, t := range toks {
		if t.Kind != gIdent {
			continue
		}
		next := gTok{}
		if i+1 < len(toks) {
			next = toks[i+1]
		}
		prev := gTok{}
		if i > 0 {
			prev = toks[i-1]
		}
		switch {
		case next.Kind == gPunct && next.Text == "=" && !(i+2 < len(toks) && toks[i+2].Text == "="):
			out[t.Text] = true
		case next.Kind == gPunct && (next.Text == "->" || next.Text == ","):
			// closure parameters: a, b -> …
			for j := i + 1; j < len(toks) && j < i+12; j++ {
				if toks[j].Kind == gPunct && toks[j].Text == "->" {
					out[t.Text] = true
					break
				}
				if toks[j].Kind == gNewline || toks[j].Kind == gPunct && (toks[j].Text == "{" || toks[j].Text == "(" || toks[j].Text == ")") {
					break
				}
			}
		case prev.Kind == gIdent && (groovyTypes[prev.Text] || isTypeName(prev.Text)) && (next.Kind == gPunct && (next.Text == "=" || next.Text == ")" || next.Text == "," || next.Text == ";" || next.Text == ":") || next.Kind == gNewline):
			out[t.Text] = true
		case next.Kind == gIdent && next.Text == "in" && prev.Kind == gPunct && prev.Text == "(":
			out[t.Text] = true
		}
	}
	return out
}
