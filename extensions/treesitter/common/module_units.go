package common

import (
	"path"
	"sort"
	"strings"

	"github.com/archstats/archstats/core/file"
	"github.com/archstats/archstats/core/unit"
)

// ModuleOf is the module a file is, which is its path without the extension.
// `client/src/utils/rules.ts` is the module `client/src/utils/rules`.
//
// A language may name one file for its directory: `index.ts` is what
// `import "./components"` means, and Python's `__init__.py` is what `from
// oscar import get_version` means. dirFile is that file's base name without
// its extension, "" for a language with none.
func ModuleOf(filePath, dirFile string) string {
	p := path.Clean(strings.TrimPrefix(filePath, "./"))
	p = strings.TrimSuffix(p, path.Ext(p))
	if base := path.Base(p); dirFile != "" && base == dirFile {
		if dir := path.Dir(p); dir != "." && dir != "/" {
			return dir
		}
	}
	return p
}

// ModuleUnit is the file itself, as the unit that makes the references its
// declarations do not: top-level code, a barrel's re-exported names, a test
// file's calls. Without it a file declaring nothing could never be an edge's
// source, so every barrel, context file and spec lost all of its imports --
// 191 of them in LibreChat alone. Returns nil when the module itself uses
// nothing, so a file is not given a unit only to stand for nothing.
//
// Its id ends in "#" with no name, so it can be a reference's source but
// never a target: nothing imports "the module" by name.
func ModuleUnit(module, filePath string, refs []unit.Ref) *unit.Unit {
	if len(refs) == 0 {
		return nil
	}
	return &unit.Unit{
		ID:    module + "#",
		Kind:  unit.KindModule,
		Name:  path.Base(module),
		Files: []string{filePath},
		Refs:  refs,
	}
}

// AttachRefs works out which unit used which imported name, and returns the
// references the module itself makes: uses outside every declaration.
//
// An import clause says what was taken and from where; the usages say by
// whom. declaredBy is the snippet that declares each unit, index for index
// (nil for a unit with no span of its own); a declaration span belongs to the
// unit whose own declaring name sits inside it. It used to be matched by
// name, which gave every method called render the first render's span.
//
// The usages are found by scanning the file for the name, because nothing in
// a tree-sitter query can ask "where is this identifier referenced" and
// capturing every identifier would put millions of snippets in the database
// to throw almost all of them away. Textual rather than semantic: a name in
// a comment or a string counts, which overstates a little and is far better
// than the alternative of attributing every import to every unit in the file.
//
// refOf is the reference one use of an imported name makes, as the language
// spells it: the module the name came from, and the unit it takes.
func AttachRefs(units []*unit.Unit, declaredBy []*file.Snippet, content []byte, bindings, sources, importSpans, spans []*file.Snippet, refOf func(source, binding string) unit.Ref) []unit.Ref {
	if len(bindings) == 0 {
		return nil
	}
	type span struct {
		begin, end int
		unitIdx    int
	}
	var owned []span
	for _, sp := range spans {
		best, bestOffset := -1, 0
		for i, d := range declaredBy {
			if d == nil || i >= len(units) {
				continue
			}
			off := d.Begin.Offset
			if off < sp.Begin.Offset || off > sp.End.Offset {
				continue
			}
			if best == -1 || off < bestOffset {
				best, bestOffset = i, off
			}
		}
		if best >= 0 {
			owned = append(owned, span{begin: sp.Begin.Offset, end: sp.End.Offset, unitIdx: best})
		}
	}
	// Innermost first, so a method inside a class is preferred over the class.
	sort.SliceStable(owned, func(i, j int) bool {
		return (owned[i].end - owned[i].begin) < (owned[j].end - owned[j].begin)
	})

	var moduleRefs []unit.Ref
	text := string(content)
	for _, b := range bindings {
		source := FirstSourceAfter(b, sources, importSpans)
		if source == "" || b.Value == "" {
			continue
		}
		ref := refOf(source, b.Value)
		for _, at := range occurrencesOf(text, b.Value) {
			if insideAny(at, importSpans) {
				continue
			}
			attributed := false
			for _, sp := range owned {
				if at >= sp.begin && at <= sp.end {
					units[sp.unitIdx].Refs = appendRef(units[sp.unitIdx].Refs, ref)
					attributed = true
					break
				}
			}
			if !attributed {
				moduleRefs = appendRef(moduleRefs, ref)
			}
		}
	}
	return moduleRefs
}

func appendRef(list []unit.Ref, r unit.Ref) []unit.Ref {
	for _, existing := range list {
		if existing == r {
			return list
		}
	}
	return append(list, r)
}

// The module an import clause took its names from: the first source string
// inside the same import statement.
func FirstSourceAfter(binding *file.Snippet, sources, importSpans []*file.Snippet) string {
	for _, span := range importSpans {
		if binding.Begin.Offset < span.Begin.Offset || binding.Begin.Offset > span.End.Offset {
			continue
		}
		for _, src := range sources {
			if src.Begin.Offset >= span.Begin.Offset && src.End.Offset <= span.End.Offset {
				return src.Value
			}
		}
	}
	return ""
}

func insideAny(offset int, spans []*file.Snippet) bool {
	for _, s := range spans {
		if offset >= s.Begin.Offset && offset <= s.End.Offset {
			return true
		}
	}
	return false
}

// Whole-word occurrences of a name. Textual rather than semantic: a name in
// a comment or a string counts, which overstates a little and is far better
// than the alternative of attributing every import to every unit in the file.
func occurrencesOf(text, name string) []int {
	var out []int
	for from := 0; ; {
		idx := strings.Index(text[from:], name)
		if idx < 0 {
			return out
		}
		at := from + idx
		from = at + len(name)
		if at > 0 && isWordByte(text[at-1]) {
			continue
		}
		if end := at + len(name); end < len(text) && isWordByte(text[end]) {
			continue
		}
		out = append(out, at)
	}
}

func isWordByte(b byte) bool {
	return b == '_' || b == '$' || (b >= '0' && b <= '9') || (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}
