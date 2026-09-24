package python

import (
	"path"
	"sort"
	"strings"

	"github.com/archstats/archstats/core/file"
	"github.com/archstats/archstats/core/unit"
	"github.com/archstats/archstats/extensions/treesitter/common"
)

// Units for Python, where the role is in the filename.
//
// django-oscar has 29 `apps.py`, 25 `views.py`, 24 `models.py` and 20
// `forms.py`, against 39 architectural decorators in the whole repository.
// Nothing in a Django class says what it is; where the file sits does. That
// is why a marker carries the kind of evidence it came from rather than
// pretending everything is an annotation: `filename:models` is a different
// claim from `annotation:Entity`, and a reader should not have to guess
// which they are looking at.

const (
	captureClass     = "py__class__definition"
	captureFunction  = "py__function__definition"
	captureDecorator = "py__decorator"
	// `from oscar.apps.basket import models` names both the module and what
	// was taken out of it, which is how a dependency between two units is
	// recoverable at all: an import that named only the module would say
	// which app was depended on and never which piece of it.
	captureBinding    = "py__import__binding"
	captureImportSpan = "py__import__span"
	captureDeclSpan   = "py__declaration__span"
)

func unitQueries() []string {
	return []string{
		`(class_definition name: (identifier) @` + captureClass + `)`,
		`(function_definition name: (identifier) @` + captureFunction + `)`,
		`(decorator [
			(identifier) @` + captureDecorator + `
			(call function: (identifier) @` + captureDecorator + `)
			(attribute attribute: (identifier) @` + captureDecorator + `)
			(call function: (attribute attribute: (identifier) @` + captureDecorator + `))
		])`,

		`(import_from_statement name: (dotted_name) @` + captureBinding + `)`,
		`(import_from_statement name: (aliased_import alias: (identifier) @` + captureBinding + `))`,
		`(import_from_statement) @` + captureImportSpan,
		`(import_statement) @` + captureImportSpan,

		`(function_definition) @` + captureDeclSpan,
		`(class_definition) @` + captureDeclSpan,
	}
}

// The filenames Django gives a meaning. A file called anything else says
// nothing, and marking it would be noise rather than evidence.
var roleFilenames = map[string]bool{
	"models": true, "views": true, "forms": true, "admin": true, "apps": true,
	"urls": true, "serializers": true, "signals": true, "receivers": true,
	"managers": true, "tasks": true, "middleware": true, "settings": true,
	"abstract_models": true,
}

type pythonAnalyzer struct {
	lp *common.LanguagePack
}

func (a *pythonAnalyzer) AnalyzeFile(f file.File) *file.Results {
	res := a.lp.AnalyzeFile(f)
	if res == nil {
		return nil
	}
	res.Units = unitsFrom(f.Path(), f.Content(), res)
	return res
}

func unitsFrom(filePath string, content []byte, res *file.Results) []*unit.Unit {
	module := common.ModuleOf(filePath)
	base := strings.TrimSuffix(path.Base(path.Clean(filePath)), ".py")

	var classes, functions, decorators []*file.Snippet
	var bindings, importSpans, declSpans, sources []*file.Snippet
	for _, s := range res.Snippets {
		switch s.Type {
		case captureClass:
			classes = append(classes, s)
		case captureFunction:
			functions = append(functions, s)
		case captureDecorator:
			decorators = append(decorators, s)
		case captureBinding:
			bindings = append(bindings, s)
		case captureImportSpan:
			importSpans = append(importSpans, s)
		case captureDeclSpan:
			declSpans = append(declSpans, s)
		case file.ImportRaw:
			sources = append(sources, s)
		}
	}
	sortByOffset(classes)
	sortByOffset(functions)

	// A file whose name Django reads marks everything it declares.
	var fileMarkers []unit.Marker
	if roleFilenames[base] {
		fileMarkers = append(fileMarkers, unit.Marker{Source: unit.SourceFilename, Key: base})
	}

	// Which class holds what. A class nested in another -- every Django
	// model's `class Meta` -- is named and owned through it, and a method
	// belongs to the innermost class whose body holds it. It used to belong
	// to the last class declared above it, so every method written after a
	// model's Meta belonged to no class: oscar's `__str__`, `clean` and
	// `generate_hash` were module-level functions, one per name per file.
	var classSpans []*file.Snippet
	for _, sp := range declSpans {
		if strings.HasPrefix(sp.Value, "class") {
			classSpans = append(classSpans, sp)
		}
	}
	nested := common.NestedNames(classes, classSpans)
	innermostClass := func(s *file.Snippet) int {
		best, bestSize := -1, 0
		for _, sp := range classSpans {
			if s.Begin.Offset <= sp.Begin.Offset || s.Begin.Offset > sp.End.Offset {
				continue
			}
			for i, c := range classes {
				if c.Begin.Offset >= sp.Begin.Offset && c.Begin.Offset <= sp.End.Offset {
					if c != s && (best == -1 || sp.End.Offset-sp.Begin.Offset < bestSize) {
						best, bestSize = i, sp.End.Offset-sp.Begin.Offset
					}
					break
				}
			}
		}
		return best
	}

	var out []*unit.Unit
	for i, c := range classes {
		u := &unit.Unit{
			ID:      module + "#" + nested[c],
			Kind:    unit.KindType,
			Name:    c.Value,
			Files:   []string{filePath},
			Markers: append([]unit.Marker(nil), fileMarkers...),
		}
		if outer := innermostClass(c); outer >= 0 {
			u.Owner = module + "#" + nested[classes[outer]]
		}
		for _, d := range decorators {
			if firstAtOrAfter(d, classes) == i && precedingIndexOf(d, classes) < i {
				u.Markers = append(u.Markers, unit.Marker{Source: unit.SourceAnnotation, Key: d.Value})
			}
		}
		out = append(out, u)
	}

	for _, f := range functions {
		owner := ""
		name := f.Value
		if idx := innermostClass(f); idx >= 0 {
			owner = module + "#" + nested[classes[idx]]
			name = nested[classes[idx]] + "." + f.Value
		}
		u := &unit.Unit{
			ID:      module + "#" + name,
			Kind:    unit.KindFunction,
			Name:    f.Value,
			Files:   []string{filePath},
			Owner:   owner,
			Markers: append([]unit.Marker(nil), fileMarkers...),
		}
		for _, d := range decorators {
			if between(d, f, functions) {
				u.Markers = append(u.Markers, unit.Marker{Source: unit.SourceAnnotation, Key: d.Value})
			}
		}
		out = append(out, u)
	}

	// Classes then functions, the order out was built in.
	var declaredBy []*file.Snippet
	declaredBy = append(declaredBy, classes...)
	declaredBy = append(declaredBy, functions...)
	moduleRefs := common.AttachRefs(out, declaredBy, content, bindings, pathLikeSources(sources), importSpans, declSpans)
	if mu := common.ModuleUnit(module, filePath, moduleRefs); mu != nil {
		out = append(out, mu)
	}
	return out
}

func sortByOffset(snippets []*file.Snippet) {
	sort.SliceStable(snippets, func(i, j int) bool {
		return snippets[i].Begin.Offset < snippets[j].Begin.Offset
	})
}

func precedingIndexOf(s *file.Snippet, declarations []*file.Snippet) int {
	last := -1
	for i, d := range declarations {
		if d.Begin.Offset < s.Begin.Offset {
			last = i
		}
	}
	return last
}

func firstAtOrAfter(s *file.Snippet, declarations []*file.Snippet) int {
	for i, d := range declarations {
		if d.Begin.Offset >= s.Begin.Offset {
			return i
		}
	}
	return -1
}

// between reports whether a decorator belongs to this function: it sits
// above it, and nothing else is declared in between.
func between(decorator, fn *file.Snippet, functions []*file.Snippet) bool {
	if decorator.Begin.Offset >= fn.Begin.Offset {
		return false
	}
	for _, other := range functions {
		if other.Begin.Offset > decorator.Begin.Offset && other.Begin.Offset < fn.Begin.Offset {
			return false
		}
	}
	return true
}

// pathLikeSources rewrites Python's relative module syntax into the path form
// the resolver reads: `.models` -> `./models`, `..core.utils` ->
// `../core/utils`. Every relative import in django-oscar went unresolved,
// because `.models` joined onto a directory is a file called ".models".
// Copies, so the stored import text stays what the file wrote.
func pathLikeSources(sources []*file.Snippet) []*file.Snippet {
	out := make([]*file.Snippet, len(sources))
	for i, s := range sources {
		c := *s
		c.Value = relativeToPath(s.Value)
		out[i] = &c
	}
	return out
}

func relativeToPath(spec string) string {
	if !strings.HasPrefix(spec, ".") {
		return spec
	}
	dots := len(spec) - len(strings.TrimLeft(spec, "."))
	rest := strings.ReplaceAll(spec[dots:], ".", "/")
	prefix := "./"
	if dots > 1 {
		prefix = strings.Repeat("../", dots-1)
	}
	if rest == "" {
		return strings.TrimSuffix(prefix, "/")
	}
	return prefix + rest
}
