package deployables

import (
	"encoding/json"
	"path"
	"regexp"
	"strings"
)

// A Dockerfile, read for what an image is made of rather than how to build
// it: its stages, what each copies in and from where, and what it starts from.
//
// Hand-written rather than BuildKit's parser. BuildKit's parser package pulls
// in the protobuf runtime, about 3.6 MB of binary for a desktop app that reads
// perhaps a dozen Dockerfiles per workspace, and it needs a newer Go than this
// module builds with. What is read here is small and well specified: line
// continuations, the escape directive, comments, heredocs, and the shell and
// JSON forms of five instructions.
type Dockerfile struct {
	Stages []*Stage
	// ARG defaults declared before the first FROM, which FROM may use.
	GlobalArgs map[string]string
}

// A Stage is one FROM and what follows it.
type Stage struct {
	Index int
	// The name after AS, lower-cased as Docker compares it; "" when unnamed.
	Name string
	// What it starts from, with ARG defaults substituted: an image reference
	// or the name of an earlier stage.
	From     string
	FromLine int
	Copies   []*Copy
	Args     map[string]string
	Expose   []string
	// The last ENTRYPOINT or CMD, as written.
	Entrypoint string
	Workdir    string
}

// A Copy is one COPY or ADD.
type Copy struct {
	Line int
	// Sources as written, relative to the build context unless From is set.
	Sources []string
	Dest    string
	// --from: a stage name, a stage index or an image reference.
	From string
	// ADD of a URL or a git repository: nothing from the context.
	Remote bool
}

var (
	escapeDirective = regexp.MustCompile(`(?i)^#\s*escape\s*=\s*([\\` + "`" + `])\s*$`)
	directiveLine   = regexp.MustCompile(`(?i)^#\s*[a-z]+\s*=`)
	heredocMarker   = regexp.MustCompile(`<<-?\s*["']?([A-Za-z_][A-Za-z0-9_]*)["']?`)
	argRef          = regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)(?:(:?[-+])([^}]*))?\}|\$([A-Za-z_][A-Za-z0-9_]*)`)
)

// ParseDockerfile reads a Dockerfile. It never fails: an instruction it does
// not understand is skipped, and a file with no FROM yields no stages.
func ParseDockerfile(content []byte) *Dockerfile {
	df := &Dockerfile{GlobalArgs: map[string]string{}}
	var current *Stage
	for _, ins := range logicalLines(string(content)) {
		keyword, rest := splitInstruction(ins.text)
		switch keyword {
		case "ARG":
			for name, value := range parseArgs(rest) {
				if current == nil {
					df.GlobalArgs[name] = value
				} else {
					// An ARG redeclared inside a stage without a value takes
					// the global default.
					if value == "" {
						value = df.GlobalArgs[name]
					}
					current.Args[name] = value
				}
			}
		case "FROM":
			flags, args := splitFlags(fields(rest))
			_ = flags
			if len(args) == 0 {
				continue
			}
			st := &Stage{Index: len(df.Stages), FromLine: ins.line, Args: map[string]string{}}
			st.From = substitute(args[0], df.GlobalArgs)
			if len(args) >= 3 && strings.EqualFold(args[1], "AS") {
				st.Name = strings.ToLower(args[2])
			}
			df.Stages = append(df.Stages, st)
			current = st
		case "COPY", "ADD":
			if current == nil {
				continue
			}
			c := parseCopy(rest, keyword == "ADD", ins.line, current, df.GlobalArgs)
			if c != nil {
				current.Copies = append(current.Copies, c)
			}
		case "EXPOSE":
			if current != nil {
				for _, p := range fields(rest) {
					current.Expose = append(current.Expose, substitute(p, mergeArgs(df.GlobalArgs, current.Args)))
				}
			}
		case "ENTRYPOINT", "CMD":
			if current != nil {
				current.Entrypoint = strings.TrimSpace(rest)
			}
		case "WORKDIR":
			if current != nil {
				current.Workdir = strings.TrimSpace(rest)
			}
		}
	}
	return df
}

type logicalLine struct {
	line int // where the instruction starts, 1-based
	text string
}

// logicalLines joins continued lines, drops comments and blank lines, and
// skips heredoc bodies, reporting each instruction at the line it starts on.
func logicalLines(content string) []logicalLine {
	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	escape := byte('\\')
	// Parser directives are only honoured before the first instruction,
	// comment or blank line.
	for _, l := range lines {
		t := strings.TrimSpace(l)
		if m := escapeDirective.FindStringSubmatch(t); m != nil {
			escape = m[1][0]
			continue
		}
		if directiveLine.MatchString(t) {
			continue
		}
		break
	}
	var out []logicalLine
	var buf strings.Builder
	start := 0
	var heredocs []string
	for i := 0; i < len(lines); i++ {
		raw := lines[i]
		if len(heredocs) > 0 {
			// Inside a heredoc body: skip until its terminator.
			if strings.TrimSpace(raw) == heredocs[0] {
				heredocs = heredocs[1:]
			}
			continue
		}
		t := strings.TrimSpace(raw)
		if buf.Len() == 0 {
			if t == "" || strings.HasPrefix(t, "#") {
				continue
			}
			start = i + 1
		} else if strings.HasPrefix(t, "#") {
			// Comment lines inside a continuation are removed.
			continue
		} else if t == "" {
			// An empty line inside a continuation is ignored too.
			continue
		}
		trimmed := strings.TrimRight(raw, " \t")
		if len(trimmed) > 0 && trimmed[len(trimmed)-1] == escape {
			buf.WriteString(trimmed[:len(trimmed)-1])
			buf.WriteString(" ")
			continue
		}
		buf.WriteString(trimmed)
		text := strings.TrimSpace(buf.String())
		buf.Reset()
		out = append(out, logicalLine{line: start, text: text})
		keyword, rest := splitInstruction(text)
		if keyword == "RUN" || keyword == "COPY" || keyword == "ADD" {
			for _, m := range heredocMarker.FindAllStringSubmatch(rest, -1) {
				heredocs = append(heredocs, m[1])
			}
		}
	}
	if buf.Len() > 0 {
		out = append(out, logicalLine{line: start, text: strings.TrimSpace(buf.String())})
	}
	return out
}

func splitInstruction(text string) (string, string) {
	i := strings.IndexAny(text, " \t")
	if i < 0 {
		return strings.ToUpper(text), ""
	}
	return strings.ToUpper(text[:i]), strings.TrimSpace(text[i+1:])
}

func fields(s string) []string { return strings.Fields(s) }

// splitFlags separates leading --flags from the arguments.
func splitFlags(all []string) (map[string]string, []string) {
	flags := map[string]string{}
	i := 0
	for ; i < len(all); i++ {
		if !strings.HasPrefix(all[i], "--") {
			break
		}
		kv := strings.TrimPrefix(all[i], "--")
		if eq := strings.Index(kv, "="); eq >= 0 {
			flags[strings.ToLower(kv[:eq])] = kv[eq+1:]
		} else {
			flags[strings.ToLower(kv)] = ""
		}
	}
	return flags, all[i:]
}

// parseArgs reads `ARG A=1 B="two" C`.
func parseArgs(rest string) map[string]string {
	out := map[string]string{}
	for _, f := range fields(rest) {
		name, value, _ := strings.Cut(f, "=")
		out[name] = strings.Trim(value, `"'`)
	}
	return out
}

func parseCopy(rest string, isAdd bool, line int, st *Stage, global map[string]string) *Copy {
	// The heredoc form (`COPY <<EOF /x`) copies inline text, nothing from the
	// context.
	if strings.Contains(rest, "<<") {
		return nil
	}
	var parts []string
	trimmed := strings.TrimSpace(rest)
	flagPart := ""
	// Flags come first in both forms.
	for strings.HasPrefix(trimmed, "--") {
		i := strings.IndexAny(trimmed, " \t")
		if i < 0 {
			break
		}
		flagPart += " " + trimmed[:i]
		trimmed = strings.TrimSpace(trimmed[i:])
	}
	flags, _ := splitFlags(fields(flagPart))
	if strings.HasPrefix(trimmed, "[") {
		var arr []string
		if json.Unmarshal([]byte(trimmed), &arr) == nil {
			parts = arr
		}
	} else {
		parts = fields(trimmed)
	}
	if len(parts) < 2 {
		return nil
	}
	args := mergeArgs(global, st.Args)
	c := &Copy{Line: line, Dest: substitute(parts[len(parts)-1], args), From: strings.ToLower(substitute(flags["from"], args))}
	for _, s := range parts[:len(parts)-1] {
		s = substitute(s, args)
		if isAdd && (strings.Contains(s, "://") || strings.HasPrefix(s, "git@")) {
			c.Remote = true
			continue
		}
		c.Sources = append(c.Sources, s)
	}
	if len(c.Sources) == 0 && !c.Remote {
		return nil
	}
	return c
}

func mergeArgs(a, b map[string]string) map[string]string {
	out := make(map[string]string, len(a)+len(b))
	for k, v := range a {
		out[k] = v
	}
	for k, v := range b {
		if v != "" || out[k] == "" {
			out[k] = v
		}
	}
	return out
}

// substitute replaces $X and ${X} with their defaults where one is known, and
// ${X:-d} with d where X has none. Unknown references are left as written, so
// a caller can tell an interpolated value from a literal.
func substitute(s string, args map[string]string) string {
	if !strings.Contains(s, "$") {
		return s
	}
	return argRef.ReplaceAllStringFunc(s, func(m string) string {
		sub := argRef.FindStringSubmatch(m)
		name, op, word := sub[1], sub[2], sub[3]
		if name == "" {
			name = sub[4]
		}
		v, set := args[name]
		switch op {
		case ":-":
			if set && v != "" {
				return v
			}
			return word
		case "-":
			if set {
				return v
			}
			return word
		case ":+", "+":
			if set && (v != "" || op == "+") {
				return word
			}
			return ""
		}
		if set && v != "" {
			return v
		}
		return m
	})
}

// StageByRef finds the stage a --from or FROM names: by name, or by index.
func (df *Dockerfile) StageByRef(ref string, before int) *Stage {
	ref = strings.ToLower(ref)
	for _, st := range df.Stages {
		if st.Index >= before {
			break
		}
		if st.Name != "" && st.Name == ref {
			return st
		}
	}
	for _, st := range df.Stages {
		if st.Index >= before {
			break
		}
		if ref == itoa(st.Index) {
			return st
		}
	}
	return nil
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b []byte
	for i > 0 {
		b = append([]byte{byte('0' + i%10)}, b...)
		i /= 10
	}
	return string(b)
}

// Target returns the stage a build of this Dockerfile produces: the named
// one, or the last.
func (df *Dockerfile) Target(name string) *Stage {
	if len(df.Stages) == 0 {
		return nil
	}
	if name != "" {
		for _, st := range df.Stages {
			if st.Name == strings.ToLower(name) {
				return st
			}
		}
		return nil
	}
	return df.Stages[len(df.Stages)-1]
}

// A ContextSource is something from the build context that ends up in the
// image, with the instruction that put it there.
type ContextSource struct {
	Path string // relative to the build context, cleaned; "." is all of it
	// A glob below Path, when the source had one: `COPY package*.json ./`
	// copies the root manifests, not the whole context.
	Pattern string
	Line    int
}

// ImageContents is everything from the build context that a target stage
// holds: what it copies itself, what it inherits from the stage it is built
// FROM, and what it copies out of other stages with --from, followed all the
// way down. LibreChat's api image copies only `api` and `config` directly,
// and four workspace packages through the stages that built them.
//
// Every stage a --from reaches counts in full. A stage that copies all of
// the context and hands on only a dist folder still counts as the whole
// context: which files a build step turned into that folder cannot be known
// without running it, and the source is what the architecture is made of.
func (df *Dockerfile) ImageContents(target *Stage) []ContextSource {
	if target == nil {
		return nil
	}
	seen := map[int]bool{}
	var out []ContextSource
	var visit func(st *Stage)
	visit = func(st *Stage) {
		if seen[st.Index] {
			return
		}
		seen[st.Index] = true
		if parent := df.StageByRef(st.From, st.Index); parent != nil {
			visit(parent)
		}
		for _, c := range st.Copies {
			if c.From != "" {
				if src := df.StageByRef(c.From, st.Index); src != nil {
					visit(src)
				}
				// --from=<image> copies from another image, not the context.
				continue
			}
			for _, s := range c.Sources {
				p, pattern := splitSource(s)
				out = append(out, ContextSource{Path: p, Pattern: pattern, Line: c.Line})
			}
		}
	}
	visit(target)
	return out
}

// BaseImage is the external image the target stage finally rests on,
// following FROM through earlier stages.
func (df *Dockerfile) BaseImage(target *Stage) (string, int) {
	st := target
	for i := 0; st != nil && i < len(df.Stages); i++ {
		parent := df.StageByRef(st.From, st.Index)
		if parent == nil {
			return st.From, st.FromLine
		}
		st = parent
	}
	return "", 0
}

// BuildImages are the external images of every stage the target depends on:
// a Go service built on golang:1.22 and shipped on distroless says its
// runtime through the builder, not the final image.
func (df *Dockerfile) BuildImages(target *Stage) []string {
	if target == nil {
		return nil
	}
	seen := map[int]bool{}
	var out []string
	var visit func(st *Stage)
	visit = func(st *Stage) {
		if seen[st.Index] {
			return
		}
		seen[st.Index] = true
		if parent := df.StageByRef(st.From, st.Index); parent != nil {
			visit(parent)
		} else if st.From != "" && !strings.EqualFold(st.From, "scratch") {
			out = append(out, st.From)
		}
		for _, c := range st.Copies {
			if src := df.StageByRef(c.From, st.Index); src != nil {
				visit(src)
			}
		}
	}
	visit(target)
	return out
}

// splitSource turns a COPY source into a context-relative path and, when the
// source has a glob, the pattern below that path: `src/*.py` is src and
// `*.py`, `package*.json` is the context root and `package*.json`,
// `packages/*/package.json` is packages and `*/package.json`.
func splitSource(s string) (string, string) {
	s = strings.TrimPrefix(s, "./")
	if s == "" || s == "." || s == "/" {
		return ".", ""
	}
	pattern := ""
	if i := strings.IndexAny(s, "*?["); i >= 0 {
		if j := strings.LastIndex(s[:i], "/"); j >= 0 {
			pattern = strings.TrimSuffix(s[j+1:], "/")
			s = s[:j]
		} else {
			pattern = strings.TrimSuffix(s, "/")
			s = ""
		}
	}
	s = path.Clean("/" + s)[1:]
	if s == "" {
		s = "."
	}
	return s, pattern
}
