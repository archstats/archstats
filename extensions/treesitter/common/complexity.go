package common

import (
	"strings"

	"github.com/archstats/archstats/core/file"
	"github.com/archstats/archstats/core/stats"
	sitter "github.com/tree-sitter/go-tree-sitter"
)

// The file stats a language pack's Complexity adds. Code health reads them;
// tasks/code-health/DECISION.md says why these and not others.
const (
	// CodeLines are the lines that hold code: not blank, not only comment.
	CodeLines = "complexity__lines__code"
	// Functions counts every function, method and closure the pack found.
	Functions = "complexity__functions"
	// ComplexFunctions counts the functions over ComplexThreshold.
	ComplexFunctions = "complexity__functions__complex"
	// ComplexLines are the lines inside functions over ComplexThreshold:
	// the code whose edits turned out to be bug fixes most often.
	ComplexLines = "complexity__lines__complex"
	// CognitiveMax is the cognitive complexity of the file's worst function.
	CognitiveMax = "complexity__cognitive__max"
	// Imports counts the file's import statements, the standard library's
	// and other projects' included: how much the file has to know about.
	// It predicted fixes where the resolved in-project fan-out did not.
	Imports = "modularity__imports__count"

	// ComplexThreshold is SonarSource's default for a function's cognitive
	// complexity, raised from 10 because 10 flagged too much.
	ComplexThreshold = 15
)

// Complexity names the node kinds that make a function hard to follow in one
// grammar. The walk is the same for every language; only the names differ.
type Complexity struct {
	// Functions are functions, methods, constructors and closures. Outside a
	// function one starts a new function; inside one it nests a level.
	Functions []string
	// FunctionBodies are bodies that stand beside their signature rather
	// than inside it, as in Dart: the body is the function, and the
	// signature before it names it.
	FunctionBodies []string
	// Ifs cost one plus their nesting, unless they are an else-if.
	Ifs []string
	// Structures are loops, switches, catches and ternaries: one plus their
	// nesting, and they nest what is inside them.
	Structures []string
	// Elses are else clauses, for grammars that have a node for one;
	// ElseIfs are else-if clauses (elif, elseif) that are not an if inside
	// an else. Grammars that put the else branch in the if's "alternative"
	// field need neither.
	Elses, ElseIfs []string
	// ElseTokens are an else written as a bare token among the if's
	// children, as in Swift: what follows is the else branch.
	ElseTokens []string
	// ElseIfWrappers hold an if's alternative; one that holds nothing but an
	// if is an else-if, as Kotlin's control_structure_body does.
	ElseIfWrappers []string
	// Logical are binary expressions; one whose operator is &&, ||, ??, and
	// or or costs one, and a run of the same operator costs one in all.
	// LogicalRuns are node kinds that are themselves an && or an ||.
	Logical, LogicalRuns []string
	// Comments are comment nodes, for counting the lines that hold code.
	Comments []string
	// Owners are the declarations whose name a function's name is read
	// under: classes, interfaces, objects, extensions.
	Owners []string
	// Imports are import statements, one per module imported; ImportCalls
	// are functions that import when called, CommonJS's require.
	Imports, ImportCalls []string
	// NameKinds name a declaration that has no name field: its first child
	// of one of these kinds is its name.
	NameKinds []string
	// ParamKinds count parameters where a function has no parameters
	// field: its children of these kinds, and their children's.
	ParamKinds []string
	// NameOf names a function where the rules above cannot; "" leaves it to
	// them.
	NameOf func(declaration *sitter.Node, src []byte) string
}

type complexityKinds struct {
	spec *Complexity
	functions, functionBodies, ifs, structures, elses, elseIfs, elseTokens, elseIfWrappers,
	logical, logicalRuns, comments, owners, imports, importCalls, nameKinds, paramKinds map[string]bool
}

func kindSet(kinds []string) map[string]bool {
	s := make(map[string]bool, len(kinds))
	for _, k := range kinds {
		s[k] = true
	}
	return s
}

func (c *Complexity) compile() *complexityKinds {
	if c == nil {
		return nil
	}
	return &complexityKinds{
		spec:      c,
		functions: kindSet(c.Functions), functionBodies: kindSet(c.FunctionBodies), ifs: kindSet(c.Ifs),
		structures: kindSet(c.Structures), elses: kindSet(c.Elses), elseIfs: kindSet(c.ElseIfs),
		elseTokens: kindSet(c.ElseTokens), elseIfWrappers: kindSet(c.ElseIfWrappers),
		logical: kindSet(c.Logical), logicalRuns: kindSet(c.LogicalRuns), comments: kindSet(c.Comments),
		owners: kindSet(c.Owners), imports: kindSet(c.Imports), importCalls: kindSet(c.ImportCalls),
		nameKinds: kindSet(c.NameKinds), paramKinds: kindSet(c.ParamKinds),
	}
}

var logicalOperators = kindSet([]string{"&&", "||", "??", "and", "or"})

// measure reads a parsed file's code lines and functions, and returns them
// as stats along with the functions themselves.
func (k *complexityKinds) measure(root *sitter.Node, content []byte) ([]*stats.Record, []*file.Function) {
	w := &complexityWalker{kinds: k, src: content}
	w.visit(root, 0)
	codeLines := k.codeLines(root, content)

	var complexFunctions, complexLines, worst int
	for _, f := range w.functions {
		if f.Cognitive > ComplexThreshold {
			complexFunctions++
			complexLines += f.Lines()
		}
		worst = max(worst, f.Cognitive)
	}
	records := []*stats.Record{
		{StatType: CodeLines, Value: codeLines},
		{StatType: Functions, Value: len(w.functions)},
		{StatType: ComplexFunctions, Value: complexFunctions},
		{StatType: ComplexLines, Value: complexLines},
		{StatType: CognitiveMax, Value: worst},
		{StatType: Imports, Value: w.imports},
	}
	return records, w.functions
}

type complexityWalker struct {
	kinds     *complexityKinds
	src       []byte
	functions []*file.Function
	current   *file.Function
	imports   int
}

// visit walks n at the given nesting and returns the deepest nesting reached
// beneath it.
func (w *complexityWalker) visit(n *sitter.Node, nesting int) int {
	k := w.kinds
	kind := n.Kind()
	if k.imports[kind] || (kind == "call_expression" && len(k.importCalls) > 0 && w.isImportCall(n)) {
		w.imports++
	}
	if k.functions[kind] || k.functionBodies[kind] {
		if w.current == nil {
			declaration := n
			if k.functionBodies[kind] && n.PrevNamedSibling() != nil {
				declaration = n.PrevNamedSibling()
			}
			f := &file.Function{
				Name:      w.nameOf(declaration),
				Begin:     int(declaration.StartPosition().Row) + 1,
				End:       int(n.EndPosition().Row) + 1,
				Params:    w.params(declaration),
				Signature: w.signature(declaration, n),
			}
			w.current = f
			f.Nesting = w.children(n, 0)
			w.current = nil
			w.functions = append(w.functions, f)
			return 0
		}
		// A closure inside a function: no cost of its own, but what is in it
		// is a level deeper.
		return w.children(n, nesting+1)
	}
	if w.current == nil {
		return w.children(n, nesting)
	}
	switch {
	case k.ifs[kind]:
		return w.visitIf(n, nesting, false)
	case k.structures[kind]:
		w.add(n, 1+nesting, w.keyword(n), nesting)
		return w.children(n, nesting+1)
	case k.logical[kind]:
		if op := n.ChildByFieldName("operator"); op != nil && logicalOperators[op.Kind()] && !continuesRun(n, op.Kind()) {
			w.add(op, 1, op.Kind(), nesting)
		}
	case k.logicalRuns[kind]:
		if p := n.Parent(); p == nil || p.Kind() != kind {
			w.add(n, 1, runOperator(n), nesting)
		}
	}
	return w.children(n, nesting)
}

// isImportCall reports whether a call is require("module").
func (w *complexityWalker) isImportCall(n *sitter.Node) bool {
	fn := n.ChildByFieldName("function")
	return fn != nil && w.kinds.importCalls[fn.Utf8Text(w.src)]
}

// continuesRun reports whether n's parent applies the same logical operator,
// so that a && b && c costs once.
func continuesRun(n *sitter.Node, operator string) bool {
	p := n.Parent()
	if p == nil || p.Kind() != n.Kind() {
		return false
	}
	op := p.ChildByFieldName("operator")
	return op != nil && op.Kind() == operator
}

func (w *complexityWalker) visitIf(n *sitter.Node, nesting int, elseIf bool) int {
	k := w.kinds
	if elseIf {
		w.add(n, 1, "else if", nesting)
	} else {
		w.add(n, 1+nesting, w.keyword(n), nesting)
	}
	deepest := nesting + 1
	afterElse := false
	for i := uint(0); i < n.ChildCount(); i++ {
		c := n.Child(i)
		switch {
		case k.elseTokens[c.Kind()]:
			// Swift: `else` then either the else-if's if or the else block.
			afterElse = true
			if next := c.NextSibling(); next == nil || !k.ifs[next.Kind()] {
				w.add(c, 1, "else", nesting)
			}
		case afterElse && k.ifs[c.Kind()]:
			deepest = max(deepest, w.visitIf(c, nesting, true))
		case n.FieldNameForChild(uint32(i)) == "alternative" || k.elses[c.Kind()] || k.elseIfs[c.Kind()]:
			deepest = max(deepest, w.visitElse(c, nesting))
		default:
			deepest = max(deepest, w.visit(c, nesting+1))
		}
	}
	return deepest
}

// visitElse costs one for an else or else-if branch, which sits at the
// nesting of the if it belongs to.
func (w *complexityWalker) visitElse(n *sitter.Node, nesting int) int {
	k := w.kinds
	kind := n.Kind()
	switch {
	case k.ifs[kind]:
		return w.visitIf(n, nesting, true)
	case k.elses[kind] || k.elseIfWrappers[kind]:
		// JavaScript writes else-if as an else clause holding an if, Kotlin
		// as a body holding nothing but an if.
		if c := onlyNamedChild(n); c != nil && k.ifs[c.Kind()] {
			return w.visitIf(c, nesting, true)
		}
		if k.elses[kind] {
			for i := uint(0); i < n.NamedChildCount(); i++ {
				if c := n.NamedChild(i); k.ifs[c.Kind()] {
					return w.visitIf(c, nesting, true)
				}
			}
		}
	case k.elseIfs[kind]:
		w.add(n, 1, w.keyword(n), nesting)
		return w.children(n, nesting+1)
	}
	// A plain else, counted where its keyword is.
	at := n
	if p := n.PrevSibling(); p != nil && p.Kind() == "else" {
		at = p
	}
	w.add(at, 1, "else", nesting)
	return w.visit(n, nesting+1)
}

// add costs the current function points for the construct at n, and
// records where, so a reader can see what the score is made of.
func (w *complexityWalker) add(n *sitter.Node, points int, construct string, nesting int) {
	w.current.Cognitive += points
	w.current.Increments = append(w.current.Increments, file.Increment{
		Line: int(n.StartPosition().Row) + 1, Points: points, Construct: construct, Nesting: nesting,
	})
}

// keyword is the word the code spells a construct with -- for, while, catch,
// when, guard, ? -- read from the tree, so no grammar needs a table of them:
// the first word token among its children, or a named *_keyword node (Swift's
// catch_keyword).
func (w *complexityWalker) keyword(n *sitter.Node) string {
	for i := uint(0); i < n.ChildCount(); i++ {
		c := n.Child(i)
		if !c.IsNamed() && (c.Kind() == "?" || isWord(c.Kind())) {
			return c.Kind()
		}
		if c.IsNamed() && strings.HasSuffix(c.Kind(), "_keyword") {
			return c.Utf8Text(w.src)
		}
	}
	return strings.ReplaceAll(strings.TrimSuffix(strings.TrimSuffix(n.Kind(), "_statement"), "_expression"), "_", " ")
}

func isWord(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && r != '@' {
			return false
		}
	}
	return true
}

// runOperator is the operator of a grammar that makes && and || node kinds
// of their own (Kotlin, Swift, Dart).
func runOperator(n *sitter.Node) string {
	for i := uint(0); i < n.ChildCount(); i++ {
		if c := n.Child(i); logicalOperators[c.Kind()] {
			return c.Kind()
		}
	}
	if kind := n.Kind(); strings.Contains(kind, "and") || strings.Contains(kind, "conjunction") {
		return "&&"
	}
	return "||"
}

func onlyNamedChild(n *sitter.Node) *sitter.Node {
	if n.NamedChildCount() != 1 {
		return nil
	}
	return n.NamedChild(0)
}

func (w *complexityWalker) children(n *sitter.Node, nesting int) int {
	deepest := nesting
	for i := uint(0); i < n.ChildCount(); i++ {
		deepest = max(deepest, w.visit(n.Child(i), nesting))
	}
	return deepest
}

// params counts a function's declared parameters, leaving out Python's
// self and cls and Java's receiver parameter. Go's `a, b int` is two.
func (w *complexityWalker) params(n *sitter.Node) int {
	p := n.ChildByFieldName("parameters")
	if p == nil {
		if d := n.ChildByFieldName("declarator"); d != nil {
			p = d.ChildByFieldName("parameters") // C: the declarator holds them
		}
	}
	if p == nil {
		if n.ChildByFieldName("parameter") != nil {
			return 1
		}
		return w.paramsByKind(n, 3)
	}
	count := 0
	for i := uint(0); i < p.NamedChildCount(); i++ {
		c := p.NamedChild(i)
		switch c.Kind() {
		case "receiver_parameter":
			continue
		case "identifier":
			if t := c.Utf8Text(w.src); i == 0 && (t == "self" || t == "cls") {
				continue
			}
		case "parameter_declaration", "variadic_parameter_declaration":
			if c.Utf8Text(w.src) == "void" {
				continue
			}
			names := 0
			for j := uint(0); j < c.ChildCount(); j++ {
				if c.FieldNameForChild(uint32(j)) == "name" {
					names++
				}
			}
			count += max(names, 1)
			continue
		}
		if w.kinds.comments[c.Kind()] {
			continue
		}
		count++
	}
	return count
}

// paramsByKind counts the children of ParamKinds within depth levels.
func (w *complexityWalker) paramsByKind(n *sitter.Node, depth int) int {
	if len(w.kinds.paramKinds) == 0 || depth == 0 {
		return 0
	}
	count := 0
	for i := uint(0); i < n.NamedChildCount(); i++ {
		c := n.NamedChild(i)
		if w.kinds.paramKinds[c.Kind()] {
			count++
		} else if !w.kinds.functions[c.Kind()] && !w.kinds.functionBodies[c.Kind()] {
			count += w.paramsByKind(c, depth-1)
		}
	}
	return count
}

// nameOf is a function's name under the declarations it sits in, or the name
// it is assigned to: `const total = () => ...` is "total". A Go method is
// named under its receiver's type.
func (w *complexityWalker) nameOf(n *sitter.Node) string {
	name := ""
	if w.kinds.spec.NameOf != nil {
		name = w.kinds.spec.NameOf(n, w.src)
	}
	if name == "" {
		name = w.declaredName(n)
	}
	if name == "" {
		if p := n.Parent(); p != nil {
			for _, field := range []string{"name", "left", "key"} {
				if c := p.ChildByFieldName(field); c != nil && c.Id() != n.Id() {
					if t := c.Utf8Text(w.src); len(t) <= 80 && !strings.ContainsAny(t, "\n(") {
						name = t
					}
					break
				}
			}
		}
	}
	if name == "" {
		return ""
	}
	owners := ""
	if r := n.ChildByFieldName("receiver"); r != nil && n.Kind() == "method_declaration" {
		if f := strings.Fields(strings.Trim(r.Utf8Text(w.src), "()")); len(f) > 0 {
			owners = strings.TrimLeft(f[len(f)-1], "*") + "."
		}
	}
	for p := n.Parent(); p != nil; p = p.Parent() {
		if w.kinds.owners[p.Kind()] {
			if owner := w.declaredName(p); owner != "" {
				owners = owner + "." + owners
			}
		}
	}
	return owners + name
}

// signature is the declaration up to its body, on one line and at most 200
// characters, starting after the annotations, attributes and decorators
// written on it. A closure assigned to a name is read from the assignment, so
// `const total = (items) =>` keeps its name.
func (w *complexityWalker) signature(declaration, n *sitter.Node) string {
	start, end := headerStart(declaration), n.EndByte()
	if body := n.ChildByFieldName("body"); body != nil && body.StartByte() > start {
		end = body.StartByte()
	} else if declaration.Id() != n.Id() {
		end = n.StartByte()
	}
	if p := declaration.Parent(); p != nil && declaration.Id() == n.Id() {
		switch p.Kind() {
		case "variable_declarator", "assignment_expression", "pair", "public_field_definition", "field_definition", "assignment":
			start = p.StartByte()
		}
	}
	if end <= start || int(end) > len(w.src) {
		return ""
	}
	return oneLine(string(w.src[start:end]))
}

// isMarkerNode is an annotation, attribute or decorator: what a declaration
// is marked with, not what it is.
func isMarkerNode(kind string) bool {
	return strings.Contains(kind, "annotation") || strings.Contains(kind, "decorator") || strings.HasPrefix(kind, "attribute") || kind == "comment" || kind == "line_comment" || kind == "block_comment"
}

// headerStart is where a declaration's own text begins: past the markers in
// front of it, including those inside a modifiers list (`@Override public`).
func headerStart(declaration *sitter.Node) uint {
	for i := uint(0); i < declaration.ChildCount(); i++ {
		c := declaration.Child(i)
		if isMarkerNode(c.Kind()) {
			continue
		}
		if strings.HasSuffix(c.Kind(), "modifiers") || c.Kind() == "modifier_list" {
			for j := uint(0); j < c.ChildCount(); j++ {
				if m := c.Child(j); !isMarkerNode(m.Kind()) {
					return m.StartByte()
				}
			}
			continue
		}
		return c.StartByte()
	}
	return declaration.StartByte()
}

// oneLine collapses a declaration to a line: comment lines dropped,
// whitespace runs squeezed, a trailing `{` left off.
func oneLine(text string) string {
	var kept []string
	for _, line := range strings.Split(text, "\n") {
		t := strings.TrimSpace(line)
		if t == "" || strings.HasPrefix(t, "//") || strings.HasPrefix(t, "/*") || strings.HasPrefix(t, "*") {
			continue
		}
		kept = append(kept, t)
	}
	out := strings.TrimSpace(strings.TrimSuffix(strings.Join(strings.Fields(strings.Join(kept, " ")), " "), "{"))
	if r := []rune(out); len(r) > 200 {
		out = string(r[:199]) + "…"
	}
	return out
}

// declaredName is a declaration's name field, or its first child of a
// NameKind.
func (w *complexityWalker) declaredName(n *sitter.Node) string {
	if c := n.ChildByFieldName("name"); c != nil {
		return c.Utf8Text(w.src)
	}
	for i := uint(0); i < n.NamedChildCount(); i++ {
		if c := n.NamedChild(i); w.kinds.nameKinds[c.Kind()] {
			return c.Utf8Text(w.src)
		}
	}
	return ""
}

// codeLines counts the lines that hold something other than whitespace and
// comment.
func (k *complexityKinds) codeLines(root *sitter.Node, content []byte) int {
	lines := strings.Split(string(content), "\n")
	commentOnly := map[int]bool{}
	var walk func(n *sitter.Node)
	walk = func(n *sitter.Node) {
		if k.comments[n.Kind()] {
			s, e := n.StartPosition(), n.EndPosition()
			for r := int(s.Row); r <= int(e.Row) && r < len(lines); r++ {
				line := lines[r]
				first := len(line) - len(strings.TrimLeft(line, " \t"))
				last := len(strings.TrimRight(line, " \t\r"))
				if (r > int(s.Row) || int(s.Column) <= first) && (r < int(e.Row) || int(e.Column) >= last) {
					commentOnly[r] = true
				}
			}
			return
		}
		for i := uint(0); i < n.ChildCount(); i++ {
			walk(n.Child(i))
		}
	}
	walk(root)
	count := 0
	for r, line := range lines {
		if strings.TrimSpace(line) != "" && !commentOnly[r] {
			count++
		}
	}
	return count
}

// Measure parses content as language and measures it, for a caller that has
// no tree yet.
func (c *Complexity) Measure(language *sitter.Language, content []byte) ([]*stats.Record, []*file.Function) {
	tree, err := parse(content, language)
	if err != nil {
		return nil, nil
	}
	defer tree.Close()
	return c.compile().measure(tree.RootNode(), content)
}

// MeasureByName measures content and returns its stats and its functions by
// name, the shape a pack's complexity test reads.
func (c *Complexity) MeasureByName(language *sitter.Language, content string) (map[string]interface{}, map[string]*file.Function) {
	records, functions := c.Measure(language, []byte(content))
	stats := map[string]interface{}{}
	for _, r := range records {
		stats[r.StatType] = r.Value
	}
	byName := map[string]*file.Function{}
	for _, f := range functions {
		byName[f.Name] = f
	}
	return stats, byName
}
