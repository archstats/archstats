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
	// What a class extends, by its last segment: `models.Model` is `Model`.
	// The filename is Django's evidence; the base class is everyone else's
	// -- Pydantic's BaseModel, SQLAlchemy's Base, DRF's ModelViewSet -- and
	// Django's too once models live in a `models/` package.
	captureSuperclass = "py__superclass"
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
		`(class_definition superclasses: (argument_list [
			(identifier) @` + captureSuperclass + `
			(attribute attribute: (identifier) @` + captureSuperclass + `)
			(subscript value: (identifier) @` + captureSuperclass + `)
			(subscript value: (attribute attribute: (identifier) @` + captureSuperclass + `))
		]))`,

		`(import_from_statement name: (dotted_name) @` + captureBinding + `)`,
		`(import_from_statement name: (aliased_import alias: (identifier) @` + captureBinding + `))`,
		`(import_from_statement) @` + captureImportSpan,
		`(import_statement) @` + captureImportSpan,

		`(function_definition) @` + captureDeclSpan,
		`(class_definition) @` + captureDeclSpan,
		// The decorator is part of what it decorates. Without this span the
		// names in `@admin.register(Product)`, `@receiver(post_save,
		// sender=Product)` and `@router.get("/x", response_model=UserOut)`
		// sat outside every declaration and were the module's references,
		// so no route ever reached its schema and no admin its model.
		`(decorated_definition) @` + captureDeclSpan,
	}
}

// The filenames Django gives a meaning. A file called anything else says
// nothing, and marking it would be noise rather than evidence.
var roleFilenames = map[string]bool{
	"models": true, "views": true, "viewsets": true, "forms": true, "admin": true, "apps": true,
	"urls": true, "serializers": true, "signals": true, "receivers": true,
	"managers": true, "tasks": true, "middleware": true, "settings": true,
	"abstract_models": true,
}

// The same roles as a package: `models/product.py` is a models file as much
// as `models.py` is, and a big app splits its models, views and serializers
// that way. django-oscar has 34 units in `models/` packages that carried no
// marker at all. `migrations/` is a directory only, and `apps/` is left out
// because it holds the apps themselves, not their configs.
var roleDirectories = map[string]bool{
	"models": true, "views": true, "viewsets": true, "forms": true, "admin": true,
	"serializers": true, "signals": true, "receivers": true, "managers": true,
	"tasks": true, "middleware": true, "settings": true, "migrations": true,
}

type pythonAnalyzer struct {
	lp *common.LanguagePack
}

func (a *pythonAnalyzer) ClaimsFile(path string) bool { return a.lp.ClaimsFile(path) }

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
	clean := path.Clean(filePath)
	base := strings.TrimSuffix(path.Base(clean), ".py")
	dir := path.Base(path.Dir(clean))

	var classes, functions, decorators, superclasses []*file.Snippet
	var bindings, importSpans, declSpans, sources []*file.Snippet
	for _, s := range res.Snippets {
		switch s.Type {
		case captureClass:
			classes = append(classes, s)
		case captureFunction:
			functions = append(functions, s)
		case captureDecorator:
			decorators = append(decorators, s)
		case captureSuperclass:
			superclasses = append(superclasses, s)
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

	// A file whose name Django reads marks everything it declares, and so
	// does a package whose name it reads.
	var fileMarkers []unit.Marker
	if roleFilenames[base] {
		fileMarkers = append(fileMarkers, unit.Marker{Source: unit.SourceFilename, Key: base})
	}
	if roleDirectories[dir] {
		fileMarkers = append(fileMarkers, unit.Marker{Source: unit.SourcePath, Key: dir})
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
	unitOf := map[*file.Snippet]*unit.Unit{}
	for _, c := range classes {
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
		unitOf[c] = u
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
		unitOf[f] = u
		out = append(out, u)
	}

	// A base class follows the name of the class it belongs to.
	for _, s := range superclasses {
		if idx := precedingIndexOf(s, classes); idx >= 0 {
			u := unitOf[classes[idx]]
			u.Markers = append(u.Markers, unit.Marker{Source: unit.SourceSupertype, Key: s.Value})
		}
	}

	// A decorator belongs to the declaration directly below it, class or
	// function alike. Matched against classes and functions separately, a
	// method's `@property` also landed on the next class in the file -- 209
	// of oscar's types carried it -- and a route function's `@router.get`
	// landed on the Pydantic model declared after it.
	declarations := append(append([]*file.Snippet{}, classes...), functions...)
	sortByOffset(declarations)
	for _, d := range decorators {
		if idx := firstAtOrAfter(d, declarations); idx >= 0 {
			u := unitOf[declarations[idx]]
			u.Markers = append(u.Markers, unit.Marker{Source: unit.SourceAnnotation, Key: d.Value})
		}
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
