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
	res.Units = unitsFrom(f.Path(), res)
	return res
}

func unitsFrom(filePath string, res *file.Results) []*unit.Unit {
	module := common.ModuleOf(filePath)
	base := strings.TrimSuffix(path.Base(path.Clean(filePath)), ".py")

	var classes, functions, decorators []*file.Snippet
	for _, s := range res.Snippets {
		switch s.Type {
		case captureClass:
			classes = append(classes, s)
		case captureFunction:
			functions = append(functions, s)
		case captureDecorator:
			decorators = append(decorators, s)
		}
	}
	sortByOffset(classes)
	sortByOffset(functions)

	// A file whose name Django reads marks everything it declares.
	var fileMarkers []unit.Marker
	if roleFilenames[base] {
		fileMarkers = append(fileMarkers, unit.Marker{Source: unit.SourceFilename, Key: base})
	}

	var out []*unit.Unit
	for i, c := range classes {
		u := &unit.Unit{
			ID:      module + "#" + c.Value,
			Kind:    unit.KindType,
			Name:    c.Value,
			Files:   []string{filePath},
			Markers: append([]unit.Marker(nil), fileMarkers...),
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
		// Python nests a method inside its class, so the class it belongs to
		// is the last one declared before it -- but only if the function is
		// indented inside it rather than following it at module level.
		if idx := precedingIndexOf(f, classes); idx >= 0 && f.Begin.CharInLine > classes[idx].Begin.CharInLine {
			owner = module + "#" + classes[idx].Value
			name = classes[idx].Value + "." + f.Value
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
