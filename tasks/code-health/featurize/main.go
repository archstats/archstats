// Featurize reads a source tree and writes the raw measurements behind the
// code-health experiments: one row per file (size, whitespace nesting as the
// engine reads it today, duplication) and one per function (length, nesting,
// cognitive and cyclomatic complexity, parameters). Scratch only: the scoring
// itself lives in score.py so formulas can change without a rebuild.
//
//	go run ./tasks/code-health/featurize <root> <out-dir>
package main

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"hash/fnv"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/archstats/archstats/core/file"
	"github.com/archstats/archstats/extensions/indentations"
	sitter "github.com/tree-sitter/go-tree-sitter"
	csharp "github.com/tree-sitter/tree-sitter-c-sharp/bindings/go"
	golang "github.com/tree-sitter/tree-sitter-go/bindings/go"
	java "github.com/tree-sitter/tree-sitter-java/bindings/go"
	javascript "github.com/tree-sitter/tree-sitter-javascript/bindings/go"
	tsphp "github.com/tree-sitter/tree-sitter-php/bindings/go"
	python "github.com/tree-sitter/tree-sitter-python/bindings/go"
	typescript "github.com/tree-sitter/tree-sitter-typescript/bindings/go"
)

// spec names the node kinds that make complexity in one grammar.
type spec struct {
	name       string
	lang       *sitter.Language
	funcs      set // a function: a root outside one, a nesting level inside one
	ifs        set // an if: +1, +nesting unless it is an else-if
	structures set // loops, switches, catches, ternaries: +1 +nesting
	elses      set // an else clause node (grammars that have one)
	elifs      set // an elif / else-if clause node
	cases      set // a case arm: cyclomatic only
	logical    set // binary kinds whose && || (and/or, ??) count
	comments   set
	imports    set // one per imported module: the file's fan-out
}

type set map[string]bool

func of(kinds ...string) set {
	s := set{}
	for _, k := range kinds {
		s[k] = true
	}
	return s
}

var (
	javaSpec = &spec{name: "java", lang: sitter.NewLanguage(java.Language()),
		funcs:      of("method_declaration", "constructor_declaration", "compact_constructor_declaration", "lambda_expression"),
		ifs:        of("if_statement"),
		structures: of("for_statement", "enhanced_for_statement", "while_statement", "do_statement", "switch_expression", "catch_clause", "ternary_expression"),
		cases:      of("switch_label", "switch_rule"),
		logical:    of("binary_expression"),
		comments:   of("line_comment", "block_comment"),
		imports:    of("import_declaration")}
	goSpec = &spec{name: "go", lang: sitter.NewLanguage(golang.Language()),
		funcs:      of("function_declaration", "method_declaration", "func_literal"),
		ifs:        of("if_statement"),
		structures: of("for_statement", "expression_switch_statement", "type_switch_statement", "select_statement"),
		cases:      of("expression_case", "type_case", "communication_case"),
		logical:    of("binary_expression"),
		comments:   of("comment"),
		imports:    of("import_spec")}
	pySpec = &spec{name: "python", lang: sitter.NewLanguage(python.Language()),
		funcs:      of("function_definition", "lambda"),
		ifs:        of("if_statement"),
		structures: of("for_statement", "while_statement", "except_clause", "conditional_expression", "match_statement"),
		elses:      of("else_clause"),
		elifs:      of("elif_clause"),
		cases:      of("case_clause"),
		logical:    of("boolean_operator"),
		comments:   of("comment"),
		imports:    of("import_statement", "import_from_statement")}
	jsKinds = func(name string, lang *sitter.Language) *spec {
		return &spec{name: name, lang: lang,
			funcs:      of("function_declaration", "generator_function_declaration", "method_definition", "arrow_function", "function_expression", "function"),
			ifs:        of("if_statement"),
			structures: of("for_statement", "for_in_statement", "while_statement", "do_statement", "switch_statement", "catch_clause", "ternary_expression"),
			elses:      of("else_clause"),
			cases:      of("switch_case"),
			logical:    of("binary_expression"),
			comments:   of("comment"),
			imports:    of("import_statement")}
	}
	jsSpec  = jsKinds("javascript", sitter.NewLanguage(javascript.Language()))
	tsSpec  = jsKinds("typescript", sitter.NewLanguage(typescript.LanguageTypescript()))
	tsxSpec = jsKinds("typescript", sitter.NewLanguage(typescript.LanguageTSX()))
	phpSpec = &spec{name: "php", lang: sitter.NewLanguage(tsphp.LanguagePHP()),
		funcs:      of("function_definition", "method_declaration", "anonymous_function", "anonymous_function_creation_expression", "arrow_function"),
		ifs:        of("if_statement"),
		structures: of("for_statement", "foreach_statement", "while_statement", "do_statement", "switch_statement", "match_expression", "catch_clause", "conditional_expression"),
		elses:      of("else_clause"),
		elifs:      of("else_if_clause"),
		cases:      of("case_statement", "match_conditional_expression"),
		logical:    of("binary_expression"),
		comments:   of("comment"),
		imports:    of("namespace_use_clause")}
	csSpec = &spec{name: "csharp", lang: sitter.NewLanguage(csharp.Language()),
		funcs:      of("method_declaration", "constructor_declaration", "destructor_declaration", "operator_declaration", "conversion_operator_declaration", "accessor_declaration", "local_function_statement", "lambda_expression", "anonymous_method_expression"),
		ifs:        of("if_statement"),
		structures: of("for_statement", "foreach_statement", "while_statement", "do_statement", "switch_statement", "switch_expression", "catch_clause", "conditional_expression"),
		cases:      of("switch_section", "switch_expression_arm"),
		logical:    of("binary_expression"),
		comments:   of("comment"),
		imports:    of("using_directive")}
)

func specFor(path string) *spec {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".java":
		return javaSpec
	case ".go":
		return goSpec
	case ".py":
		return pySpec
	case ".js", ".jsx", ".mjs", ".cjs":
		return jsSpec
	case ".ts", ".mts", ".cts":
		return tsSpec
	case ".tsx":
		return tsxSpec
	case ".php":
		return phpSpec
	case ".cs":
		return csSpec
	}
	return nil
}

var logicalOps = of("&&", "||", "??", "and", "or")

type fn struct {
	key               string // Type.name/params, "" for an anonymous function
	start, end        int
	params            int
	cognitive, cyclo  int
	maxNesting, bumps int
}

type walker struct {
	sp      *spec
	src     []byte
	funcs   []*fn
	cur     *fn
	imports int
}

func (w *walker) visit(n *sitter.Node, nesting int) int {
	kind := n.Kind()
	if w.sp.imports[kind] {
		w.imports++
	}
	if w.sp.funcs[kind] {
		if w.cur == nil {
			f := &fn{start: int(n.StartPosition().Row), end: int(n.EndPosition().Row), params: w.params(n), cyclo: 1}
			if name := w.nameOf(n); name != "" {
				f.key = w.owners(n) + name + "/" + itoa(f.params)
			}
			w.cur = f
			w.children(n, 0)
			w.cur = nil
			w.funcs = append(w.funcs, f)
			return 0
		}
		return w.children(n, nesting+1)
	}
	if w.cur == nil {
		return w.children(n, nesting)
	}
	f := w.cur
	switch {
	case w.sp.ifs[kind]:
		return w.visitIf(n, nesting, false)
	case w.sp.structures[kind]:
		f.cognitive += 1 + nesting
		f.cyclo++
		depth := w.children(n, nesting+1)
		w.mark(nesting, depth)
		return depth
	case w.sp.cases[kind]:
		f.cyclo++
	case w.sp.logical[kind]:
		if op := n.ChildByFieldName("operator"); op != nil && logicalOps[op.Kind()] {
			f.cyclo++
			// A run of one operator is one increment: a && b && c counts once.
			if p := n.Parent(); p == nil || p.Kind() != kind || p.ChildByFieldName("operator") == nil || p.ChildByFieldName("operator").Kind() != op.Kind() {
				f.cognitive++
			}
		}
	}
	return w.children(n, nesting)
}

// mark counts a bump: a top-level block of the function that nests at least
// twice deep. Two or more make a bumpy road.
func (w *walker) mark(nesting, depth int) {
	if nesting == 0 && depth >= 2 {
		w.cur.bumps++
	}
	if depth > w.cur.maxNesting {
		w.cur.maxNesting = depth
	}
}

func (w *walker) visitIf(n *sitter.Node, nesting int, elseIf bool) int {
	f := w.cur
	f.cyclo++
	if elseIf {
		f.cognitive++
	} else {
		f.cognitive += 1 + nesting
	}
	depth := nesting + 1
	for i := uint(0); i < n.ChildCount(); i++ {
		c := n.Child(i)
		var d int
		if n.FieldNameForChild(uint32(i)) == "alternative" || w.sp.elses[c.Kind()] || w.sp.elifs[c.Kind()] {
			d = w.visitElse(c, nesting)
		} else {
			d = w.visit(c, nesting+1)
		}
		depth = max(depth, d)
	}
	if !elseIf {
		w.mark(nesting, depth)
	}
	return depth
}

func (w *walker) visitElse(n *sitter.Node, nesting int) int {
	f := w.cur
	kind := n.Kind()
	switch {
	case w.sp.ifs[kind]:
		return w.visitIf(n, nesting, true)
	case w.sp.elses[kind]:
		// JavaScript writes else-if as an else clause holding an if.
		for i := uint(0); i < n.NamedChildCount(); i++ {
			if c := n.NamedChild(i); w.sp.ifs[c.Kind()] {
				return w.visitIf(c, nesting, true)
			}
		}
		f.cognitive++
		return w.children(n, nesting+1)
	case w.sp.elifs[kind]:
		f.cognitive++
		f.cyclo++
		return w.children(n, nesting+1)
	}
	f.cognitive++ // a plain else block
	return w.visit(n, nesting+1)
}

func (w *walker) children(n *sitter.Node, nesting int) int {
	depth := nesting
	for i := uint(0); i < n.ChildCount(); i++ {
		depth = max(depth, w.visit(n.Child(i), nesting))
	}
	return depth
}

func (w *walker) params(n *sitter.Node) int {
	p := n.ChildByFieldName("parameters")
	if p == nil {
		if n.ChildByFieldName("parameter") != nil {
			return 1
		}
		return 0
	}
	count := 0
	for i := uint(0); i < p.NamedChildCount(); i++ {
		c := p.NamedChild(i)
		k := c.Kind()
		if w.sp.comments[k] || k == "receiver_parameter" {
			continue
		}
		if w.sp.name == "python" {
			if t := c.Utf8Text(w.src); (t == "self" || t == "cls") && i == 0 {
				continue
			}
		}
		if w.sp.name == "go" {
			names := 0
			for j := uint(0); j < c.ChildCount(); j++ {
				if c.FieldNameForChild(uint32(j)) == "name" {
					names++
				}
			}
			count += max(names, 1)
			continue
		}
		count++
	}
	return count
}

// commentRows are the rows that hold nothing but comment.
func commentRows(root *sitter.Node, sp *spec, lines []string) map[int]bool {
	rows := map[int]bool{}
	var walk func(n *sitter.Node)
	walk = func(n *sitter.Node) {
		if sp.comments[n.Kind()] {
			s, e := n.StartPosition(), n.EndPosition()
			for r := int(s.Row); r <= int(e.Row) && r < len(lines); r++ {
				line := lines[r]
				first := len(line) - len(strings.TrimLeft(line, " \t"))
				last := len(strings.TrimRight(line, " \t\r"))
				if (r > int(s.Row) || int(s.Column) <= first) && (r < int(e.Row) || int(e.Column) >= last) {
					rows[r] = true
				}
			}
			return
		}
		for i := uint(0); i < n.ChildCount(); i++ {
			walk(n.Child(i))
		}
	}
	walk(root)
	return rows
}

type fileRow struct {
	path, lang           string
	test                 bool
	lines, code, comment int
	wsMax, wsTotal       int
	deep                 [maxDeep + 1]int // code lines at least this many levels in
	wsAvg                float64
	wsVol                int
	parseError           bool
	codeLines            []string // normalized, for duplication
	codeRowIdx           []int
	dupMark              []bool
	imports              int
}

type osFile struct {
	fs.FileInfo
	path    string
	content []byte
}

func (f *osFile) Path() string    { return f.path }
func (f *osFile) Content() []byte { return f.content }

var skipDirs = of(".git", "node_modules", "vendor", "dist", "build", "target", "bower_components", ".venv", "venv", "__pycache__")

func main() {
	if os.Args[1] == "touched" {
		touched(os.Args[2], os.Args[3], os.Args[4])
		return
	}
	root, out := os.Args[1], os.Args[2]
	_ = os.MkdirAll(out, 0o755)
	ind := indentations.Detected()
	parsers := map[*spec]*sitter.Parser{}

	var files []*fileRow
	ff, _ := os.Create(filepath.Join(out, "functions.csv"))
	fw := csv.NewWriter(ff)
	_ = fw.Write([]string{"path", "key", "start", "lines", "params", "cognitive", "cyclomatic", "max_nesting", "bumps"})

	began := time.Now()
	_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if p != root && (skipDirs[d.Name()] || strings.HasPrefix(d.Name(), ".")) {
				return filepath.SkipDir
			}
			return nil
		}
		sp := specFor(p)
		if sp == nil {
			return nil
		}
		rel, _ := filepath.Rel(root, p)
		rel = filepath.ToSlash(rel)
		content, err := os.ReadFile(p)
		if err != nil || len(content) == 0 || len(content) > 2<<20 {
			return nil
		}
		if file.IsGenerated(content) || file.IsThirdParty(rel, content) {
			return nil
		}
		info, _ := d.Info()
		res := ind.AnalyzeFile(&osFile{FileInfo: info, path: rel, content: content})
		row := &fileRow{path: rel, lang: sp.name, test: file.IsTestPath(rel)}
		for _, r := range res.Stats {
			switch r.StatType {
			case indentations.Max:
				row.wsMax = r.Value.(int)
			case indentations.Volatility:
				row.wsVol = r.Value.(int)
			case indentations.Count:
				row.wsTotal = r.Value.(int)
			}
		}

		parser, ok := parsers[sp]
		if !ok {
			parser = sitter.NewParser()
			_ = parser.SetLanguage(sp.lang)
			parsers[sp] = parser
		}
		tree := parser.Parse(content, nil)
		rootNode := tree.RootNode()
		row.parseError = rootNode.HasError()
		lines := strings.Split(string(content), "\n")
		if len(lines) > 0 && lines[len(lines)-1] == "" {
			lines = lines[:len(lines)-1]
		}
		row.lines = len(lines)
		comments := commentRows(rootNode, sp, lines)
		width, ok := detectWidth(content)
		if !ok {
			width = 4
		}
		for i, l := range lines {
			t := strings.TrimSpace(l)
			switch {
			case t == "":
			case comments[i]:
				row.comment++
			default:
				row.code++
				lv := levels(l, width)
				for k := 0; k <= maxDeep && k <= lv; k++ {
					row.deep[k]++
				}
				row.codeLines = append(row.codeLines, strings.Join(strings.Fields(t), " "))
				row.codeRowIdx = append(row.codeRowIdx, i)
			}
		}
		// The engine's average: total levels over non-blank lines.
		if nonBlank := row.code + row.comment; nonBlank > 0 {
			row.wsAvg = float64(row.wsTotal) / float64(nonBlank)
		}
		w := &walker{sp: sp, src: content}
		w.visit(rootNode, 0)
		w.dedupe()
		row.imports = w.imports
		tree.Close()
		for _, f := range w.funcs {
			_ = fw.Write([]string{rel, f.key, itoa(f.start), itoa(f.end - f.start + 1), itoa(f.params), itoa(f.cognitive), itoa(f.cyclo), itoa(f.maxNesting), itoa(f.bumps)})
		}
		files = append(files, row)
		return nil
	})
	fw.Flush()
	ff.Close()

	markDuplicates(files)

	out2, _ := os.Create(filepath.Join(out, "files.csv"))
	cw := csv.NewWriter(out2)
	_ = cw.Write([]string{"path", "lang", "test", "lines", "code_lines", "comment_lines", "ws_max", "ws_avg", "ws_vol", "dup_lines", "parse_error", "deep1", "deep2", "deep3", "deep4", "deep5", "deep6", "deep7", "deep8", "imports"})
	for _, f := range files {
		dup := 0
		for _, m := range f.dupMark {
			if m {
				dup++
			}
		}
		_ = cw.Write([]string{f.path, f.lang, strconv.FormatBool(f.test), itoa(f.lines), itoa(f.code), itoa(f.comment), itoa(f.wsMax), strconv.FormatFloat(f.wsAvg, 'f', 4, 64), itoa(f.wsVol), itoa(dup), strconv.FormatBool(f.parseError), itoa(f.deep[1]), itoa(f.deep[2]), itoa(f.deep[3]), itoa(f.deep[4]), itoa(f.deep[5]), itoa(f.deep[6]), itoa(f.deep[7]), itoa(f.deep[8]), itoa(f.imports)})
	}
	cw.Flush()
	out2.Close()
	fmt.Fprintf(os.Stderr, "%d files in %s\n", len(files), time.Since(began).Round(time.Millisecond))
}

// duplicationWindow is how many consecutive code lines make a clone.
const duplicationWindow = 6

// markDuplicates flags the code lines that sit in a run of six that appears
// somewhere else too, in this file or another. Runs of braces and imports
// are too common to mean anything and are skipped.
func markDuplicates(files []*fileRow) {
	type at struct {
		f   *fileRow
		idx int
	}
	seen := map[uint64][]at{}
	for _, f := range files {
		f.dupMark = make([]bool, len(f.codeLines))
		for i := 0; i+duplicationWindow <= len(f.codeLines); i++ {
			win := f.codeLines[i : i+duplicationWindow]
			if !substantial(win) {
				continue
			}
			h := fnv.New64a()
			for _, l := range win {
				h.Write([]byte(l))
				h.Write([]byte{'\n'})
			}
			k := h.Sum64()
			seen[k] = append(seen[k], at{f, i})
		}
	}
	for _, places := range seen {
		if len(places) < 2 {
			continue
		}
		for _, p := range places {
			for j := p.idx; j < p.idx+duplicationWindow; j++ {
				p.f.dupMark[j] = true
			}
		}
	}
}

func substantial(win []string) bool {
	chars := 0
	for _, l := range win {
		if strings.HasPrefix(l, "import ") || strings.HasPrefix(l, "using ") || strings.HasPrefix(l, "from ") || strings.HasPrefix(l, "package ") || strings.HasPrefix(l, "use ") || strings.HasPrefix(l, "@") {
			return false
		}
		chars += len(strings.Trim(l, "{}();,[] "))
	}
	return chars >= 120
}

func itoa(i int) string { return strconv.Itoa(i) }

const maxDeep = 8

// levels is a line's leading indentation in levels, as the engine counts it.
func levels(line string, width int) int {
	tabs, spaces := 0, 0
	for _, c := range line {
		switch c {
		case '\t':
			tabs++
		case ' ':
			spaces++
		default:
			return tabs + spaces/width
		}
	}
	return tabs + spaces/width
}

// detectWidth is the engine's (extensions/indentations/detect.go), copied
// because it is unexported.
func detectWidth(content []byte) (int, bool) {
	var tabbed, spaced int
	steps := map[int]int{}
	previous := 0
	for _, line := range strings.Split(string(content), "\n") {
		if len(strings.TrimSpace(line)) == 0 {
			continue
		}
		spaces := 0
		for spaces < len(line) && line[spaces] == ' ' {
			spaces++
		}
		if line[spaces] == '\t' {
			tabbed++
			continue
		}
		if spaces > 0 {
			spaced++
		}
		if step := spaces - previous; step >= 2 && step <= 8 {
			steps[step]++
		}
		previous = spaces
	}
	if tabbed > spaced {
		return 4, true
	}
	best, bestCount := 0, 0
	for step, count := range steps {
		if count > bestCount || (count == bestCount && step < best) {
			best, bestCount = step, count
		}
	}
	return best, bestCount > 0
}

// nameOf is a function's own name, or the name it is assigned to.
func (w *walker) nameOf(n *sitter.Node) string {
	if c := n.ChildByFieldName("name"); c != nil {
		return c.Utf8Text(w.src)
	}
	if p := n.Parent(); p != nil {
		for _, field := range []string{"name", "left", "key"} {
			if c := p.ChildByFieldName(field); c != nil && c.Id() != n.Id() {
				t := c.Utf8Text(w.src)
				if len(t) <= 80 && !strings.ContainsAny(t, "\n(") {
					return t
				}
			}
		}
	}
	return ""
}

var ownerKinds = of("class_declaration", "interface_declaration", "enum_declaration", "record_declaration",
	"class_definition", "class", "abstract_class_declaration", "trait_declaration", "struct_declaration",
	"object_declaration", "property_declaration", "impl_item")

// owners names the types around a function, outermost first: "Outer.Inner.".
// A Go method's receiver type stands in for its class.
func (w *walker) owners(n *sitter.Node) string {
	chain := ""
	if n.Kind() == "method_declaration" && w.sp.name == "go" {
		if r := n.ChildByFieldName("receiver"); r != nil {
			t := strings.Trim(r.Utf8Text(w.src), "()")
			if f := strings.Fields(t); len(f) > 0 {
				chain = strings.TrimLeft(f[len(f)-1], "*") + "."
			}
		}
	}
	for p := n.Parent(); p != nil; p = p.Parent() {
		if ownerKinds[p.Kind()] {
			if c := p.ChildByFieldName("name"); c != nil {
				chain = c.Utf8Text(w.src) + "." + chain
			}
		}
	}
	return chain
}

// dedupe numbers repeated keys (same name and arity) in file order.
func (w *walker) dedupe() {
	seen := map[string]int{}
	for _, f := range w.funcs {
		if f.key == "" {
			continue
		}
		seen[f.key]++
		if k := seen[f.key]; k > 1 {
			f.key += "#" + itoa(k)
		}
	}
}

func functionsOf(sp *spec, parser *sitter.Parser, content []byte) []*fn {
	tree := parser.Parse(content, nil)
	defer tree.Close()
	w := &walker{sp: sp, src: content}
	w.visit(tree.RootNode(), 0)
	w.dedupe()
	return w.funcs
}

var hunk = regexp.MustCompile(`^@@ -\d+(?:,\d+)? \+(\d+)(?:,(\d+))? @@`)

// touched maps every later change of a T0 file onto the functions it
// edited: the rows of the new side of each diff hunk (around a pure
// deletion, the rows either side), matched to functions by key.
func touched(repo, changesCSV, outCSV string) {
	in, _ := os.Open(changesCSV)
	rows, _ := csv.NewReader(in).ReadAll()
	in.Close()
	type job struct{ commit, path, origin, fix string }
	jobs := make(chan job)
	results := make(chan []string, 64)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			parsers := map[*spec]*sitter.Parser{}
			for j := range jobs {
				sp := specFor(j.path)
				diff, err := exec.Command("git", "-C", repo, "diff", "-U0", "--no-color", "--no-ext-diff", j.commit+"^", j.commit, "--", j.path).Output()
				if err != nil || bytes.Contains(diff, []byte("\nnew file mode")) {
					continue
				}
				content, err := exec.Command("git", "-C", repo, "show", j.commit+":"+j.path).Output()
				if err != nil {
					continue
				}
				changed := map[int]bool{}
				for _, line := range strings.Split(string(diff), "\n") {
					m := hunk.FindStringSubmatch(line)
					if m == nil {
						continue
					}
					start, _ := strconv.Atoi(m[1])
					count := 1
					if m[2] != "" {
						count, _ = strconv.Atoi(m[2])
					}
					if count == 0 {
						changed[start-1], changed[start] = true, true
					}
					for r := start - 1; r < start-1+count; r++ {
						changed[r] = true
					}
				}
				parser, ok := parsers[sp]
				if !ok {
					parser = sitter.NewParser()
					_ = parser.SetLanguage(sp.lang)
					parsers[sp] = parser
				}
				for _, f := range functionsOf(sp, parser, content) {
					if f.key == "" {
						continue
					}
					for r := f.start; r <= f.end; r++ {
						if changed[r] {
							results <- []string{j.origin, f.key, j.commit, j.fix}
							break
						}
					}
				}
			}
		}()
	}
	go func() {
		for _, r := range rows[1:] {
			if specFor(r[1]) != nil {
				jobs <- job{r[0], r[1], r[2], r[3]}
			}
		}
		close(jobs)
		wg.Wait()
		close(results)
	}()
	out, _ := os.Create(outCSV)
	cw := csv.NewWriter(out)
	_ = cw.Write([]string{"path", "key", "commit", "fix"})
	n := 0
	for r := range results {
		_ = cw.Write(r)
		n++
	}
	cw.Flush()
	out.Close()
	fmt.Fprintf(os.Stderr, "%d function edits from %d file changes\n", n, len(rows)-1)
}
