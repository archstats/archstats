package kotlin

import (
	"sort"
	"strings"

	"github.com/archstats/archstats/core/file"
	"github.com/archstats/archstats/core/unit"
	"github.com/archstats/archstats/extensions/treesitter/common"
)

// Units for Kotlin, where a function can belong to a type it does not live in.
//
// Exposed declares 225 extension functions. `fun Table.selectAll()` is
// Table's, written in another file and usually another module, and that is
// exactly the case the unit grain keeps apart: where a unit lives and what
// it belongs to are different questions.
//
// An interface is a class_declaration here like any other, and an object is
// its own kind of declaration, so both are read as types.

const (
	captureType       = "kt__type__declaration"
	captureObject     = "kt__object__declaration"
	captureFunc       = "kt__function__declaration"
	captureReceiver   = "kt__function__receiver"
	captureSupertype  = "kt__supertype"
	captureAnnotation = "kt__annotation"
	// A keyword on the declaration, kept as a "keyword" marker. `expect` or
	// `actual`: Kotlin Multiplatform's declaration in common code and its
	// implementation per platform, which have the same qualified name and
	// fold into one unit whose files span the source sets. `data`, `sealed`
	// and the other class modifiers: what kind of class it is.
	capturePlatform = "kt__platform"
	captureDeclSpan = "kt__declaration__span"
	captureTypeSpan = "kt__type__span"
	capturePackage  = "kt__package"
	captureImport   = "kt__import"
	// What a declaration mentions, for references to its own package and to
	// star imports, which no import line names; and the headers whose names
	// are not uses.
	captureRefName     = "kt__ref__name"
	captureRefWildcard = "kt__ref__wildcard"
	captureRefSkip     = "kt__ref__skip"
	// A function's own type parameters: `fun <T : Table> T.deleteWhere()`
	// extends whatever T is, not a type called T.
	captureTypeParam = "kt__function__type_param"
)

func unitQueries() []string {
	return []string{
		`(class_declaration (type_identifier) @` + captureType + `)`,
		`(object_declaration (type_identifier) @` + captureObject + `)`,
		// A supertype is written three ways: `: Store`, `: Table()` -- a
		// constructor call, which is how every Exposed table extends Table
		// and which was never read -- and `: Store by delegate`.
		`(delegation_specifier (user_type (type_identifier) @` + captureSupertype + `))`,
		`(delegation_specifier (constructor_invocation (user_type (type_identifier) @` + captureSupertype + `)))`,
		`(delegation_specifier (explicit_delegation (user_type (type_identifier) @` + captureSupertype + `)))`,

		// An extension function's receiver is a field of its declaration;
		// the name is the declaration's only direct identifier, wherever
		// type parameters put it.
		`(function_declaration receiver: (receiver_type (user_type (type_identifier) @` + captureReceiver + `)))`,
		`(function_declaration receiver: (receiver_type (nullable_type (user_type (type_identifier) @` + captureReceiver + `))))`,
		`(function_declaration (simple_identifier) @` + captureFunc + `)`,
		`(function_declaration (type_parameters (type_parameter (type_identifier) @` + captureTypeParam + `)))`,

		// An annotation is read from the modifiers of the declaration it is
		// written on, and belongs to that declaration. Captured bare, it was
		// handed to the next class in the file: nowinandroid's composables
		// carried nothing, and NiaButtonDefaults, the object after five of
		// them, carried Composable five times. A parameter's annotation
		// (`@param:Named`) marks no declaration at all.
		`(class_declaration (modifiers (annotation) @` + captureAnnotation + `))`,
		`(class_declaration (primary_constructor (modifiers (annotation) @` + captureAnnotation + `)))`,
		`(object_declaration (modifiers (annotation) @` + captureAnnotation + `))`,
		`(function_declaration (modifiers (annotation) @` + captureAnnotation + `))`,
		`(class_declaration (modifiers (platform_modifier) @` + capturePlatform + `))`,
		`(object_declaration (modifiers (platform_modifier) @` + capturePlatform + `))`,
		`(function_declaration (modifiers (platform_modifier) @` + capturePlatform + `))`,
		// `data`, `sealed`, `enum`, `value`, `annotation`: what kind of class
		// it is. A data class is the shape Kotlin gives a model, and no
		// annotation or supertype says so.
		`(class_declaration (modifiers (class_modifier) @` + capturePlatform + `))`,
		`(class_declaration) @` + captureTypeSpan,
		`(object_declaration) @` + captureTypeSpan,
		`(function_declaration) @` + captureDeclSpan,
		`(package_header (identifier) @` + capturePackage + `)`,
		`(import_header (identifier) @` + captureImport + `)`,
		`(import_header (identifier) @` + captureRefWildcard + ` (wildcard_import))`,
		`(type_identifier) @` + captureRefName,
		`(simple_identifier) @` + captureRefName,
		`(import_header) @` + captureRefSkip,
		`(package_header) @` + captureRefSkip,
	}
}

type kotlinAnalyzer struct {
	lp *common.LanguagePack
}

func (a *kotlinAnalyzer) ClaimsFile(path string) bool { return a.lp.ClaimsFile(path) }

func (a *kotlinAnalyzer) AnalyzeFile(f file.File) *file.Results {
	res := a.lp.AnalyzeFile(f)
	if res == nil {
		return nil
	}
	// Types the parser lost, recovered from the text; see
	// recoverDeclarations. They take part in building the units and are not
	// kept as snippets of their own.
	names, spans := recoverDeclarations(f.Content(), res.Snippets)
	parsed := res.Snippets
	if len(names) > 0 {
		res.Snippets = append(append(append([]*file.Snippet{}, parsed...), names...), spans...)
	}
	res.Units = unitsFrom(f.Path(), res)
	res.Snippets = parsed
	// Helper captures: every name in the file, and whole declarations as
	// text. They build the references and are not kept.
	kept := res.Snippets[:0]
	for _, sn := range res.Snippets {
		switch sn.Type {
		case captureRefName, captureRefWildcard, captureRefSkip, captureDeclSpan, captureTypeSpan, captureTypeParam:
			continue
		}
		kept = append(kept, sn)
	}
	res.Snippets = kept
	return res
}

func unitsFrom(path string, res *file.Results) []*unit.Unit {
	var types, objects, funcs, receivers, supertypes, annotations, platforms, imports, typeSpans []*file.Snippet
	var declSpans, names, wildcards, skips, typeParams []*file.Snippet
	pkg := ""
	for _, s := range res.Snippets {
		switch s.Type {
		case captureType:
			types = append(types, s)
		case captureObject:
			objects = append(objects, s)
		case captureFunc:
			funcs = append(funcs, s)
		case captureReceiver:
			receivers = append(receivers, s)
		case captureSupertype:
			supertypes = append(supertypes, s)
		case captureAnnotation:
			annotations = append(annotations, s)
		case capturePlatform:
			platforms = append(platforms, s)
		case captureImport:
			imports = append(imports, s)
		case captureTypeSpan:
			typeSpans = append(typeSpans, s)
		case captureDeclSpan:
			declSpans = append(declSpans, s)
		case captureRefName:
			names = append(names, s)
		case captureRefWildcard:
			wildcards = append(wildcards, s)
		case captureRefSkip:
			skips = append(skips, s)
		case captureTypeParam:
			typeParams = append(typeParams, s)
		case capturePackage:
			if pkg == "" {
				pkg = s.Value
			}
		}
	}
	all := append(append([]*file.Snippet{}, types...), objects...)
	byOffset(all)
	byOffset(funcs)
	byOffset(receivers)

	qualify := func(name string) string {
		if pkg == "" {
			return name
		}
		return pkg + "." + name
	}

	// A class inside another is named through it: six test classes of one
	// package each declaring a `Town` are six types, not one.
	nested := common.NestedNames(all, typeSpans)

	var out []*unit.Unit
	var declaredBy []*file.Snippet
	for i, t := range all {
		u := &unit.Unit{
			ID:    qualify(nested[t]),
			Kind:  unit.KindType,
			Name:  t.Value,
			Files: []string{path},
		}
		// A nested type is a member of the one around it, as a method is.
		if i := strings.LastIndex(nested[t], "."); i > 0 {
			u.Owner = qualify(nested[t][:i])
		}
		for _, sup := range supertypes {
			if ownerIndex(sup, all) == i {
				u.Markers = append(u.Markers, unit.Marker{Source: unit.SourceSupertype, Key: sup.Value})
			}
		}
		out = append(out, u)
		declaredBy = append(declaredBy, t)
	}

	for _, f := range funcs {
		// The receiver of an extension function sits immediately before the
		// name. A function declared inside a class body belongs to that
		// class instead, which is the enclosing declaration.
		owner, name := "", f.Value
		var markers []unit.Marker
		if recv := receiverOf(f, receivers, declSpans, typeParams); recv != "" {
			owner = qualify(recv)
			name = recv + "." + f.Value
			// The receiver is evidence of the function's role the way a
			// supertype is of a class's: `fun Route.users()` is a Ktor
			// route and `fun Application.module()` a Ktor module, and
			// nothing else about either says so.
			markers = append(markers, unit.Marker{Source: "receiver", Key: recv})
		} else if enclosing := enclosingType(f, typeSpans, all, nested); enclosing != "" {
			// Inside the type's body, not merely declared after it. A
			// top-level function written below a class is not its method,
			// and attributing it there put `topLevel` inside `Store`.
			owner = qualify(enclosing)
			name = enclosing + "." + f.Value
		}
		out = append(out, &unit.Unit{
			ID:      qualify(name),
			Kind:    unit.KindFunction,
			Name:    f.Value,
			Files:   []string{path},
			Owner:   owner,
			Markers: markers,
		})
		declaredBy = append(declaredBy, f)
	}

	// Each annotation goes to the innermost declaration around it: the one
	// whose modifiers it was written in.
	owned := ownedSpans(out, declaredBy, append(append([]*file.Snippet{}, typeSpans...), declSpans...))
	for _, a := range annotations {
		for _, sp := range owned {
			if a.Begin.Offset >= sp.begin && a.Begin.Offset <= sp.end {
				if key := annotationKey(a.Value); key != "" {
					out[sp.unit].Markers = append(out[sp.unit].Markers, unit.Marker{Source: unit.SourceAnnotation, Key: key})
				}
				break
			}
		}
	}

	for _, p := range platforms {
		for _, sp := range owned {
			if p.Begin.Offset >= sp.begin && p.Begin.Offset <= sp.end {
				out[sp.unit].Markers = append(out[sp.unit].Markers, unit.Marker{Source: "keyword", Key: strings.TrimSpace(p.Value)})
				break
			}
		}
	}

	moduleRefs := attachKotlinRefs(out, declaredBy, pkg, imports, wildcards, names, skips, append(typeSpans, declSpans...))
	if len(moduleRefs) > 0 {
		base := strings.TrimSuffix(filepathBase(path), ".kt")
		out = append(out, &unit.Unit{
			ID:    qualify(base) + "#",
			Kind:  unit.KindModule,
			Name:  base,
			Files: []string{path},
			Refs:  moduleRefs,
		})
	}
	return out
}

// attachKotlinRefs gives each declaration the references it makes, resolved
// the way the compiler resolves a simple name: an explicit import, then the
// file's own package, then its star imports. Returns the references made
// outside every declaration, which belong to the file.
//
// Every top-level declaration used to receive every import of its file, used
// or not, and nothing reached through its own package or a star import at
// all -- the gap Java had.
func attachKotlinRefs(units []*unit.Unit, declaredBy []*file.Snippet, pkg string, imports, wildcards, names, skips, spans []*file.Snippet) []unit.Ref {
	wild := map[string]bool{}
	for _, w := range wildcards {
		wild[w.Value] = true
	}
	explicit := map[string]string{} // simple name -> package
	var onDemand []string
	for _, imp := range imports {
		if wild[imp.Value] {
			onDemand = append(onDemand, imp.Value)
			continue
		}
		idx := strings.LastIndex(imp.Value, ".")
		if idx <= 0 || idx == len(imp.Value)-1 {
			continue
		}
		explicit[imp.Value[idx+1:]] = imp.Value[:idx]
	}
	refsFor := func(name string) []unit.Ref {
		if p, ok := explicit[name]; ok {
			return []unit.Ref{{Module: p, Name: name}}
		}
		var out []unit.Ref
		if pkg != "" {
			out = append(out, unit.Ref{Module: pkg, Name: name, Exact: true})
		}
		for _, p := range onDemand {
			out = append(out, unit.Ref{Module: p, Name: name, Exact: true})
		}
		return out
	}

	spansOwned := ownedSpans(units, declaredBy, spans)

	var moduleRefs []unit.Ref
	add := func(list []unit.Ref, r unit.Ref) []unit.Ref {
		for _, e := range list {
			if e == r {
				return list
			}
		}
		return append(list, r)
	}
	for _, n := range names {
		at := n.Begin.Offset
		skipped := false
		for _, sk := range skips {
			if at >= sk.Begin.Offset && at <= sk.End.Offset {
				skipped = true
				break
			}
		}
		if skipped || n.Value == "" {
			continue
		}
		attributed := false
		for _, sp := range spansOwned {
			if at >= sp.begin && at <= sp.end {
				u := units[sp.unit]
				if n.Value != u.Name {
					for _, r := range refsFor(n.Value) {
						u.Refs = add(u.Refs, r)
					}
				}
				attributed = true
				break
			}
		}
		if !attributed {
			for _, r := range refsFor(n.Value) {
				moduleRefs = add(moduleRefs, r)
			}
		}
	}
	return moduleRefs
}

type ownedSpan struct{ begin, end, unit int }

// ownedSpans pairs each declaration's span with the unit it declares -- the
// first declared name inside it -- smallest first, so the first span that
// contains an offset is the innermost declaration around it.
func ownedSpans(units []*unit.Unit, declaredBy, spans []*file.Snippet) []ownedSpan {
	var out []ownedSpan
	for _, sp := range spans {
		best, bestOffset := -1, 0
		for i, d := range declaredBy {
			if d == nil || i >= len(units) {
				continue
			}
			if off := d.Begin.Offset; off >= sp.Begin.Offset && off <= sp.End.Offset && (best == -1 || off < bestOffset) {
				best, bestOffset = i, off
			}
		}
		if best >= 0 {
			out = append(out, ownedSpan{sp.Begin.Offset, sp.End.Offset, best})
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].end-out[i].begin < out[j].end-out[j].begin
	})
	return out
}

// annotationKey is an annotation's simple name: `@Preview(showBackground =
// true)` is Preview, `@androidx.compose.runtime.Composable` is Composable and
// `@get:JvmName("x")` is JvmName. The arguments used to stay in the key, so
// Hilt's `@InstallIn(SingletonComponent::class)` never matched InstallIn.
func annotationKey(v string) string {
	v = strings.TrimPrefix(strings.TrimSpace(v), "@")
	if i := strings.IndexAny(v, "(<"); i >= 0 {
		v = v[:i]
	}
	if i := strings.LastIndex(v, ":"); i >= 0 {
		v = v[i+1:]
	}
	v = strings.TrimSpace(v)
	if i := strings.LastIndex(v, "."); i >= 0 {
		v = v[i+1:]
	}
	return v
}

func filepathBase(p string) string {
	if i := strings.LastIndex(p, "/"); i >= 0 {
		return p[i+1:]
	}
	return p
}

func byOffset(snippets []*file.Snippet) {
	sort.SliceStable(snippets, func(i, j int) bool {
		return snippets[i].Begin.Offset < snippets[j].Begin.Offset
	})
}

func ownerIndex(s *file.Snippet, declarations []*file.Snippet) int {
	last := -1
	for i, d := range declarations {
		if d.Begin.Offset <= s.Begin.Offset {
			last = i
		}
	}
	return last
}

func precedingIndex(s *file.Snippet, declarations []*file.Snippet) int {
	last := -1
	for i, d := range declarations {
		if d.Begin.Offset < s.Begin.Offset {
			last = i
		}
	}
	return last
}

// enclosingType is the type whose body contains this function, if any. The
// smallest containing span wins, so a function in a nested class belongs to
// the inner one.
func enclosingType(f *file.Snippet, typeSpans, names []*file.Snippet, nested map[*file.Snippet]string) string {
	best := ""
	bestSize := 0
	for _, sp := range typeSpans {
		if f.Begin.Offset < sp.Begin.Offset || f.Begin.Offset > sp.End.Offset {
			continue
		}
		size := sp.End.Offset - sp.Begin.Offset
		if best != "" && size >= bestSize {
			continue
		}
		for _, n := range names {
			if n.Begin.Offset >= sp.Begin.Offset && n.Begin.Offset <= sp.End.Offset {
				best, bestSize = nested[n], size
				break
			}
		}
	}
	return best
}

// receiverOf is the receiver of the function a name declares, if it is an
// extension: the receiver written before the name inside the smallest
// declaration holding both. `fun <T> Column<T>.comment()` puts type
// arguments between the two, which reading only what sits directly in front
// of the name missed, so the extension became a plain function.
func receiverOf(name *file.Snippet, receivers, declSpans, typeParams []*file.Snippet) string {
	var own *file.Snippet
	for _, sp := range declSpans {
		if name.Begin.Offset < sp.Begin.Offset || name.Begin.Offset > sp.End.Offset {
			continue
		}
		if own == nil || sp.End.Offset-sp.Begin.Offset < own.End.Offset-own.Begin.Offset {
			own = sp
		}
	}
	if own == nil {
		return ""
	}
	last := ""
	for _, r := range receivers {
		if r.Begin.Offset >= own.Begin.Offset && r.End.Offset <= name.Begin.Offset {
			last = r.Value
		}
	}
	for _, tp := range typeParams {
		if tp.Value == last && tp.Begin.Offset >= own.Begin.Offset && tp.End.Offset <= name.Begin.Offset {
			return ""
		}
	}
	return last
}
