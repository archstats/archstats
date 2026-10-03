package php

import (
	"regexp"
	"sort"
	"strings"

	"github.com/archstats/archstats/core/file"
	"github.com/archstats/archstats/core/unit"
	"github.com/archstats/archstats/extensions/treesitter/common"
)

const (
	captureType       = "php__type__declaration"
	captureFunction   = "php__function__declaration"
	captureSpan       = "php__declaration__span"
	captureSupertype  = "php__supertype"
	captureAttribute  = "php__attribute"
	captureDoc        = "php__doc"
	captureUse        = "php__type__use"
	captureCall       = "php__function__call"
	captureImportDecl = "php__use__declaration"
	// Which of the types is an interface, so a consumer can tell a contract
	// from a class the way every other pack lets it.
	captureInterface = "php__interface__name"
)

func unitQueries() []string {
	names := `[(name) (qualified_name)]`
	return []string{
		`(class_declaration name: (name) @` + captureType + `)`,
		`(interface_declaration name: (name) @` + captureType + `)`,
		`(trait_declaration name: (name) @` + captureType + `)`,
		`(enum_declaration name: (name) @` + captureType + `)`,
		`(function_definition name: (name) @` + captureFunction + `)`,

		`(class_declaration) @` + captureSpan,
		`(interface_declaration) @` + captureSpan,
		`(trait_declaration) @` + captureSpan,
		`(enum_declaration) @` + captureSpan,
		`(function_definition) @` + captureSpan,

		`(base_clause ` + names + ` @` + captureSupertype + `)`,
		`(class_interface_clause ` + names + ` @` + captureSupertype + `)`,
		// An attribute wherever it sits: on the class, or on a method or
		// property inside it. Symfony's #[Route] is usually on the action,
		// and Doctrine's #[ORM\Column] on the property; read at the class
		// only, a controller whose routes were all on its methods carried
		// nothing.
		`(attribute_list (attribute_group (attribute ` + names + ` @` + captureAttribute + `)))`,
		`(interface_declaration name: (name) @` + captureInterface + `)`,
		`(comment) @` + captureDoc,

		// Everywhere a class is named rather than declared: extends and
		// implements, a type, `new`, a static call or constant, instanceof,
		// a trait, an attribute. Names that are not a class of this
		// codebase in the namespace they resolve to resolve to nothing.
		`(base_clause ` + names + ` @` + captureUse + `)`,
		`(class_interface_clause ` + names + ` @` + captureUse + `)`,
		`(named_type ` + names + ` @` + captureUse + `)`,
		`(object_creation_expression ` + names + ` @` + captureUse + `)`,
		`(scoped_call_expression scope: ` + names + ` @` + captureUse + `)`,
		`(scoped_property_access_expression scope: ` + names + ` @` + captureUse + `)`,
		`(class_constant_access_expression . ` + names + ` @` + captureUse + `)`,
		`(binary_expression operator: "instanceof" right: ` + names + ` @` + captureUse + `)`,
		`(use_declaration ` + names + ` @` + captureUse + `)`,
		`(attribute ` + names + ` @` + captureUse + `)`,
		`(function_call_expression function: ` + names + ` @` + captureCall + `)`,

		`(namespace_use_declaration) @` + captureImportDecl,
	}
}

type phpAnalyzer struct {
	lp *common.LanguagePack
}

func (a *phpAnalyzer) ClaimsFile(path string) bool { return a.lp.ClaimsFile(path) }

func (a *phpAnalyzer) AnalyzeFile(f file.File) *file.Results {
	res := a.lp.AnalyzeFile(f)
	if res == nil {
		return nil
	}
	res.Units = unitsFrom(f.Path(), res)
	// The helper captures build units and references; every name in every
	// expression is one, and they are not kept.
	kept := res.Snippets[:0]
	for _, sn := range res.Snippets {
		switch sn.Type {
		case captureSpan, captureDoc, captureUse, captureCall, captureImportDecl, captureInterface:
			continue
		}
		kept = append(kept, sn)
	}
	res.Snippets = kept
	return res
}

func unitsFrom(path string, res *file.Results) []*unit.Unit {
	namespace := ""
	var types, functions, spans, supertypes, attributes, docs, uses, calls, imports []*file.Snippet
	interfaces := map[int]bool{}
	for _, s := range res.Snippets {
		switch s.Type {
		case file.ComponentDeclaration:
			if namespace == "" {
				namespace = strings.TrimPrefix(s.Value, `\`)
			}
		case captureType:
			types = append(types, s)
		case captureFunction:
			functions = append(functions, s)
		case captureSpan:
			spans = append(spans, s)
		case captureSupertype:
			supertypes = append(supertypes, s)
		case captureAttribute:
			attributes = append(attributes, s)
		case captureDoc:
			docs = append(docs, s)
		case captureUse:
			uses = append(uses, s)
		case captureCall:
			calls = append(calls, s)
		case captureImportDecl:
			imports = append(imports, s)
		case captureInterface:
			interfaces[s.Begin.Offset] = true
		}
	}
	names := newNames(namespace, imports)

	declared := append(append([]*file.Snippet{}, types...), functions...)
	sort.SliceStable(declared, func(i, j int) bool { return declared[i].Begin.Offset < declared[j].Begin.Offset })

	// Each declaration span belongs to the first name declared inside it,
	// which is the name of what it declares.
	type owned struct {
		begin, end int
		unit       int
	}
	var out []*unit.Unit
	unitOf := map[*file.Snippet]int{}
	var bySpan []owned
	for _, d := range declared {
		kind := unit.KindType
		if d.Type == captureFunction {
			kind = unit.KindFunction
		}
		unitOf[d] = len(out)
		u := &unit.Unit{
			ID:    qualify(namespace, d.Value),
			Kind:  kind,
			Name:  d.Value,
			Files: []string{path},
		}
		if interfaces[d.Begin.Offset] {
			u.Markers = append(u.Markers, unit.Marker{Source: unit.SourceSupertype, Key: "interface"})
		}
		out = append(out, u)
	}
	for _, sp := range spans {
		for _, d := range declared {
			if d.Begin.Offset >= sp.Begin.Offset && d.Begin.Offset <= sp.End.Offset {
				bySpan = append(bySpan, owned{sp.Begin.Offset, sp.End.Offset, unitOf[d]})
				break
			}
		}
	}
	sort.SliceStable(bySpan, func(i, j int) bool {
		return bySpan[i].end-bySpan[i].begin < bySpan[j].end-bySpan[j].begin
	})
	ownerAt := func(offset int) int {
		for _, o := range bySpan {
			if offset >= o.begin && offset <= o.end {
				return o.unit
			}
		}
		return -1
	}
	spanOf := func(u int) (int, int) {
		for i := len(bySpan) - 1; i >= 0; i-- {
			if bySpan[i].unit == u {
				return bySpan[i].begin, bySpan[i].end
			}
		}
		return -1, -1
	}

	// Markers: what a type extends or implements, its attributes, and the
	// annotations of the docblock right above it (Doctrine's @ORM\Entity).
	// Keyed by the name the `use` clause resolves to, so `extends Eloquent`
	// under `use Illuminate\Database\Eloquent\Model as Eloquent` is a Model.
	resolved := func(written string) string {
		if r, ok := names.class(written); ok {
			return r.Name
		}
		return lastSegment(written)
	}
	for _, s := range supertypes {
		if u := ownerAt(s.Begin.Offset); u >= 0 {
			out[u].Markers = appendMarker(out[u].Markers, unit.Marker{Source: unit.SourceSupertype, Key: resolved(s.Value)})
		}
	}
	for _, s := range attributes {
		if u := ownerAt(s.Begin.Offset); u >= 0 {
			out[u].Markers = appendMarker(out[u].Markers, unit.Marker{Source: unit.SourceAnnotation, Key: resolved(s.Value)})
		}
	}
	for _, t := range types {
		u := unitOf[t]
		begin, _ := spanOf(u)
		if begin < 0 {
			continue
		}
		for _, d := range docs {
			if d.End.Offset <= begin && begin-d.End.Offset <= 2 && strings.HasPrefix(d.Value, "/**") {
				for _, m := range docTag.FindAllStringSubmatch(d.Value, -1) {
					out[u].Markers = append(out[u].Markers, unit.Marker{Source: unit.SourceAnnotation, Key: lastSegment(m[1])})
				}
			}
		}
	}

	// References, resolved the way PHP resolves a class name: fully
	// qualified, through a `use` alias, or in the file's own namespace.
	var moduleRefs []unit.Ref
	add := func(at int, refs ...unit.Ref) {
		u := ownerAt(at)
		for _, r := range refs {
			if u >= 0 {
				if out[u].Name == r.Name && r.Module == namespace {
					continue // a type naming itself
				}
				out[u].Refs = appendRef(out[u].Refs, r)
			} else {
				moduleRefs = appendRef(moduleRefs, r)
			}
		}
	}
	for _, s := range uses {
		if r, ok := names.class(s.Value); ok {
			add(s.Begin.Offset, r)
		}
	}
	for _, s := range calls {
		add(s.Begin.Offset, names.function(s.Value)...)
	}

	if mu := common.ModuleUnit(strings.TrimSuffix(path, ".php"), path, moduleRefs); mu != nil {
		out = append(out, mu)
	}
	return out
}

var docTag = regexp.MustCompile(`@([A-Z][A-Za-z0-9_]*(?:\\[A-Za-z0-9_]+)*)`)

func qualify(namespace, name string) string {
	if namespace == "" {
		return name
	}
	return namespace + `\` + name
}

func lastSegment(name string) string {
	name = strings.TrimPrefix(name, `\`)
	if i := strings.LastIndex(name, `\`); i >= 0 {
		return name[i+1:]
	}
	return name
}

// appendMarker keeps one marker per claim: twenty #[ORM\Column] properties
// are one fact about the entity.
func appendMarker(list []unit.Marker, m unit.Marker) []unit.Marker {
	for _, e := range list {
		if e == m {
			return list
		}
	}
	return append(list, m)
}

func appendRef(list []unit.Ref, r unit.Ref) []unit.Ref {
	for _, e := range list {
		if e == r {
			return list
		}
	}
	return append(list, r)
}

// names resolves what a file writes to what it means.
type names struct {
	namespace string
	classes   map[string]string // alias -> fully qualified name
	functions map[string]string
}

var useClause = regexp.MustCompile(`^\s*\\?([A-Za-z0-9_\\]+?)(?:\s+as\s+([A-Za-z0-9_]+))?\s*$`)

func newNames(namespace string, declarations []*file.Snippet) *names {
	n := &names{namespace: namespace, classes: map[string]string{}, functions: map[string]string{}}
	for _, d := range declarations {
		text := strings.TrimSpace(d.Value)
		text = strings.TrimPrefix(text, "use")
		text = strings.TrimSuffix(strings.TrimSpace(text), ";")
		into := n.classes
		switch {
		case strings.HasPrefix(strings.TrimSpace(text), "function "):
			into, text = n.functions, strings.TrimPrefix(strings.TrimSpace(text), "function ")
		case strings.HasPrefix(strings.TrimSpace(text), "const "):
			continue
		}
		prefix := ""
		if open := strings.Index(text, "{"); open >= 0 {
			prefix = strings.TrimSpace(text[:open])
			text = strings.TrimSuffix(strings.TrimSpace(text[open+1:]), "}")
		}
		for _, clause := range strings.Split(text, ",") {
			m := useClause.FindStringSubmatch(clause)
			if m == nil {
				continue
			}
			full := strings.TrimPrefix(prefix+m[1], `\`)
			alias := m[2]
			if alias == "" {
				alias = lastSegment(full)
			}
			into[strings.ToLower(alias)] = full
		}
	}
	return n
}

// class resolves a class name as written. self, static and parent name the
// class they are written in or its parent, which the declaration already
// says.
func (n *names) class(written string) (unit.Ref, bool) {
	written = strings.TrimSpace(written)
	switch strings.ToLower(written) {
	case "", "self", "static", "parent":
		return unit.Ref{}, false
	}
	full := n.expand(written, n.classes)
	module, name := split(full)
	return unit.Ref{Module: module, Name: name, Exact: true}, true
}

// function resolves a function call. An unqualified one that no `use`
// names falls back to the global namespace, as PHP does.
func (n *names) function(written string) []unit.Ref {
	written = strings.TrimSpace(written)
	if written == "" {
		return nil
	}
	full := n.expand(written, n.functions)
	module, name := split(full)
	refs := []unit.Ref{{Module: module, Name: name, Exact: true}}
	if !strings.Contains(written, `\`) && module != "" {
		if _, aliased := n.functions[strings.ToLower(written)]; !aliased {
			refs = append(refs, unit.Ref{Module: "", Name: name, Exact: true})
		}
	}
	return refs
}

func (n *names) expand(written string, aliases map[string]string) string {
	if strings.HasPrefix(written, `\`) {
		return written[1:]
	}
	first, rest, qualified := strings.Cut(written, `\`)
	if target, ok := aliases[strings.ToLower(first)]; ok {
		if qualified {
			return target + `\` + rest
		}
		return target
	}
	// A relative name continues the namespace through an aliased prefix
	// only; otherwise it is inside the file's own namespace.
	if qualified {
		if target, ok := n.classes[strings.ToLower(first)]; ok {
			return target + `\` + rest
		}
	}
	return qualify(n.namespace, written)
}

func split(full string) (module, name string) {
	if i := strings.LastIndex(full, `\`); i >= 0 {
		return full[:i], full[i+1:]
	}
	return "", full
}
