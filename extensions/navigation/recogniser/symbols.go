package recogniser

import (
	"regexp"
	"strings"
)

// The declaration shapes several languages share, for their recognisers'
// Declared, Modules and Constants.

var (
	// Type name: `OrderService orders`, `IRepository<Order> _repo`.
	typedName = regexp.MustCompile(`\b([A-Z][\w]*(?:\.[A-Z]\w*)*(?:\s*<[^;=(){}<>]*(?:<[^;=(){}<>]*>[^;=(){}<>]*)*>)?(?:\[\])?\??)\s+(_?[a-z][\w$]*)\s*(?:[;=,)]|$|:\s*[A-Z])`)
	// name: Type, as Kotlin, TypeScript, Swift, Scala and Python hints write it.
	nameColonType = regexp.MustCompile(`(?:^|[\s(,{])(?:(?:private|protected|public|readonly|override|internal|val|var|let|const|lateinit)\s+)*([a-z_$][\w$]*)\s*[?!]?\s*:\s*([A-Z][\w.$]*(?:\s*<[^;=(){}<>]*(?:<[^;=(){}<>]*>[^;=(){}<>]*)*>)?)`)
	// x = module.Client(  (Python, Go): the type keeps its module.
	moduleConstructed = regexp.MustCompile(`\b([a-z_][\w]*)\s*(?::=|=)\s*([a-z_]\w*\.(?:[A-Z]\w*|create|createClient|Session|Client|AsyncClient|New\w*))\s*\(`)
	// String constants.
	staticConst = regexp.MustCompile(`\b(?:static\s+final|final\s+static|const)\s+(?:String|string|var)?\s*([A-Za-z_]\w*)\s*=\s*("(?:[^"\\]|\\.)*"(?:\s*\+\s*(?:"(?:[^"\\]|\\.)*"|[A-Z_][A-Z0-9_]*))*)\s*;`)
	scriptConst = regexp.MustCompile(`(?m)^\s*(?:export\s+)?(?:const|let|var|val|const val)?\s*([A-Za-z_][\w]*)\s*(?::\s*\w+)?\s*=\s*(["'` + "`" + `][^"'` + "`" + `\n]*["'` + "`" + `])\s*;?\s*$`)
)

// TypedNames declares names written after their type: `OrderService orders`.
func TypedNames(s *Source, sy *Symbols) {
	for _, m := range typedName.FindAllStringSubmatch(s.Code, -1) {
		sy.SetVar(m[2], m[1])
	}
}

// ColonTypes declares names written before their type: `orders: OrderService`.
func ColonTypes(s *Source, sy *Symbols) {
	for _, m := range nameColonType.FindAllStringSubmatch(s.Code, -1) {
		sy.SetVar(m[1], m[2])
	}
}

// ModuleConstructed declares names bound to what a module constructs:
// `client = httpx.Client(`.
func ModuleConstructed(s *Source, sy *Symbols) {
	for _, m := range moduleConstructed.FindAllStringSubmatch(s.Code, -1) {
		sy.SetVar(m[1], m[2])
	}
}

// StaticConstants are `static final String X = "…" + Y;` and `const`
// constants, concatenations evaluated.
func StaticConstants(s *Source, sy *Symbols) {
	for _, m := range staticConst.FindAllStringSubmatch(s.Code, -1) {
		sy.SetConst(m[1], evalConcat(m[2], sy.Consts))
	}
}

// AssignedConstants are names assigned a string literal on a line of their
// own.
func AssignedConstants(s *Source, sy *Symbols) {
	for _, m := range scriptConst.FindAllStringSubmatch(s.Code, -1) {
		if u := Unquote(m[2]); u != "" {
			sy.SetConst(m[1], u)
		}
	}
}

// ConcatConstants are constants a pattern finds, its first group the name
// and its second the value, concatenations evaluated.
func ConcatConstants(s *Source, sy *Symbols, re *regexp.Regexp) {
	for _, m := range re.FindAllStringSubmatch(s.Code, -1) {
		sy.SetConst(m[1], evalConcat(m[2], sy.Consts))
	}
}

// evalConcat is a constant built from strings and other constants.
func evalConcat(expr string, consts map[string]string) string {
	var b strings.Builder
	for _, part := range strings.Split(expr, "+") {
		part = strings.TrimSpace(part)
		if u := Unquote(part); u != "" {
			b.WriteString(u)
		} else if v, ok := consts[part]; ok {
			b.WriteString(v)
		}
	}
	return b.String()
}
