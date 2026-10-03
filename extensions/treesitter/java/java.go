package java

import (
	"embed"
	"fmt"
	"github.com/archstats/archstats/core"
	"github.com/archstats/archstats/core/definitions"
	"github.com/archstats/archstats/core/file"
	"github.com/archstats/archstats/core/stats"
	"github.com/archstats/archstats/core/unit"
	"github.com/archstats/archstats/extensions/treesitter/common"
	"github.com/samber/lo"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	java "github.com/tree-sitter/tree-sitter-java/bindings/go"
	"path/filepath"
	"sort"
	"strings"
)

type Extension struct {
	IgnoreImportsFor        []string
	IgnoreCommonJavaImports bool
	ExtraQueries            []string
	// ClassIndirect writes java_class_connections_indirect: every class
	// reachable from every class. It was the largest table of a Java
	// snapshot and nothing in the app reads it, so it is opt-in.
	ClassIndirect bool
}

func (e *Extension) typeAssert() core.Extension {
	return e
}

//go:embed definitions/**
var javaDefs embed.FS

type javaAnalyzer struct {
	lp *common.LanguagePack
}

func (a *javaAnalyzer) ClaimsFile(path string) bool { return a.lp.ClaimsFile(path) }

func (ja *javaAnalyzer) AnalyzeFile(f file.File) *file.Results {
	res := ja.lp.AnalyzeFile(f)
	if res == nil {
		return nil
	}
	// Post-process to find the package name and class name
	var javaClass string
	var javaPackage string

	for _, snippet := range res.Snippets {
		if snippet.Type == "modularity__component__declarations" {
			javaPackage = snippet.Value
		}
		if snippet.Type == "java__type__declaration" {
			if javaClass == "" {
				javaClass = snippet.Value
			}
			base := filepath.Base(f.Path())
			baseWithoutExt := strings.TrimSuffix(base, ".java")
			if snippet.Value == baseWithoutExt {
				javaClass = snippet.Value
			}
		}
	}

	if javaClass != "" {
		res.Stats = append(res.Stats, &stats.Record{
			StatType: "java_class",
			Value:    javaClass,
		})

		javaFullClass := javaClass
		if javaPackage != "" {
			javaFullClass = javaPackage + "." + javaClass
		}
		res.Stats = append(res.Stats, &stats.Record{
			StatType: "java_full_class",
			Value:    javaFullClass,
		})

		// The types, as units. The stats above stay as the compatibility
		// path for snapshots taken before units existed. Java is the one
		// language that imports the type itself, so a reference needs no
		// resolving beyond splitting the name: an import of com.acme.Order
		// is a reference to Order in com.acme. The file's references go to
		// its public type, which is the one that made nearly all of them.
		res.Units = javaUnits(f.Path(), javaClass, javaPackage, res.Snippets)
	}
	res.Snippets = withoutReferenceCaptures(res.Snippets)
	return res
}

// Every type declared in the file, as a unit: the public one named for the
// file, any other top-level type beside it, and the nested ones, which
// belong to the type around them the way a Kotlin nested class does.
//
// There used to be one unit per file carrying every annotation and supertype
// found anywhere in it. fineract writes its JDBC row mappers as nested
// classes of the service that uses them, so 43 of its @Service classes
// "implemented RowMapper"; a test class's nested @Configuration made the
// test a configuration. A marker belongs to the innermost declaration
// around it, and nothing else.
func javaUnits(path, primary, pkg string, snippets []*file.Snippet) []*unit.Unit {
	var names, spans, markers, interfaces []*file.Snippet
	for _, s := range snippets {
		switch s.Type {
		case "java__type__declaration":
			names = append(names, s)
		case "java__type__span":
			spans = append(spans, s)
		case "java__class__annotation", "java__class__extends", "java__class__implements":
			markers = append(markers, s)
		case "java__interface__declaration":
			interfaces = append(interfaces, s)
		}
	}
	nested := common.NestedNames(names, spans)
	qualify := func(name string) string {
		if pkg == "" {
			return name
		}
		return pkg + "." + name
	}

	// Innermost first, so the first span holding an offset is the
	// declaration the marker was written on.
	sort.SliceStable(spans, func(i, j int) bool {
		return spans[i].End.Offset-spans[i].Begin.Offset < spans[j].End.Offset-spans[j].Begin.Offset
	})
	declaredIn := func(sp *file.Snippet) *file.Snippet {
		var first *file.Snippet
		for _, n := range names {
			if n.Begin.Offset >= sp.Begin.Offset && n.Begin.Offset <= sp.End.Offset && (first == nil || n.Begin.Offset < first.Begin.Offset) {
				first = n
			}
		}
		return first
	}
	ownerOf := func(offset int) *file.Snippet {
		for _, sp := range spans {
			if offset >= sp.Begin.Offset && offset <= sp.End.Offset {
				return declaredIn(sp)
			}
		}
		return nil
	}

	byName := map[*file.Snippet]*unit.Unit{}
	var out []*unit.Unit
	for _, n := range names {
		u := &unit.Unit{ID: qualify(nested[n]), Kind: unit.KindType, Name: n.Value, Files: []string{path}}
		if i := strings.LastIndex(nested[n], "."); i > 0 {
			u.Owner = qualify(nested[n][:i])
		}
		byName[n] = u
		out = append(out, u)
	}
	for _, s := range markers {
		owner := ownerOf(s.Begin.Offset)
		if owner == nil {
			continue
		}
		source := unit.SourceSupertype
		if s.Type == "java__class__annotation" {
			source = unit.SourceAnnotation
		}
		byName[owner].Markers = append(byName[owner].Markers, unit.Marker{Source: source, Key: s.Value})
	}
	// An interface is one, whatever it extends; the other packs say so the
	// same way, and a consumer deciding whether a type is a data shape
	// reads it.
	for _, in := range interfaces {
		for _, n := range names {
			if n.Begin.Offset == in.Begin.Offset && n.Value == in.Value {
				byName[n].Markers = append(byName[n].Markers, unit.Marker{Source: unit.SourceSupertype, Key: "interface"})
			}
		}
	}
	for _, u := range out {
		if u.Name == primary && u.Owner == "" {
			u.Refs = javaRefs(snippets, pkg)
			break
		}
	}
	return out
}

// Every type this file uses, as a reference to the unit it names, resolved
// the way the compiler resolves a simple name: a single-type import first,
// then the file's own package, then its on-demand (wildcard) imports.
//
// Only imports used to count. A class never imports the types in its own
// package, so every same-package reference was missing -- all 928 of
// Broadleaf's `implements`/`extends` of a same-package type among them, the
// most common relationship in Java -- along with everything reached through
// `import a.b.*`. Names that resolve to nothing in the codebase (generic
// parameters, JDK types, locals) are dropped by the resolver.
func javaRefs(snippets []*file.Snippet, pkg string) []unit.Ref {
	var refs []unit.Ref
	seen := map[string]bool{}
	add := func(module, name string) {
		key := module + "\x00" + name
		if module == "" || name == "" || seen[key] {
			return
		}
		seen[key] = true
		refs = append(refs, unit.Ref{Module: module, Name: name})
	}
	// A simple name looked up in a package the language names exactly.
	addExact := func(module, name string) {
		key := module + "\x00" + name + "\x00exact"
		if module == "" || name == "" || seen[key] {
			return
		}
		seen[key] = true
		refs = append(refs, unit.Ref{Module: module, Name: name, Exact: true})
	}
	split := func(v string) (string, string) {
		idx := strings.LastIndex(v, ".")
		if idx <= 0 || idx == len(v)-1 {
			return "", ""
		}
		return v[:idx], v[idx+1:]
	}

	wildcard, static := map[string]bool{}, map[string]bool{}
	for _, s := range snippets {
		switch s.Type {
		case "java__ref__wildcard":
			wildcard[s.Value] = true
		case "java__ref__static":
			static[s.Value] = true
		}
	}

	imported := map[string]bool{}
	var onDemand []string
	for _, s := range snippets {
		if s.Type != "java__import__declaration" {
			continue
		}
		v := s.Value
		switch {
		case static[v] && wildcard[v]:
			// import static a.b.C.*; -- a reference to C.
			add(split(v))
		case static[v]:
			// import static a.b.C.member; -- a reference to C, not to member.
			typ, _ := split(v)
			module, name := split(typ)
			add(module, name)
			imported[name] = true
		case wildcard[v]:
			// import a.b.*; -- a package to look names up in, not a type.
			onDemand = append(onDemand, v)
		default:
			module, name := split(v)
			add(module, name)
			imported[name] = true
		}
	}

	declared := map[string]bool{}
	for _, s := range snippets {
		if s.Type == "java__type__declaration" {
			declared[s.Value] = true
		}
	}
	for _, s := range snippets {
		if s.Type != "java__ref__type" {
			continue
		}
		name := s.Value
		if name == "" || imported[name] || declared[name] || !isUpper(name[0]) {
			continue
		}
		addExact(pkg, name)
		for _, p := range onDemand {
			addExact(p, name)
		}
	}
	return refs
}

func isUpper(b byte) bool { return b >= 'A' && b <= 'Z' }

// The captures javaRefs reads. They are not snippets anyone reads afterwards,
// so they are removed before the results leave the analyser rather than
// stored once per type mention in every file.
func javaReferenceQueries() []string {
	return []string{`
(type_identifier) @java__ref__type
(marker_annotation name: (identifier) @java__ref__type)
(annotation name: (identifier) @java__ref__type)
(method_invocation object: (identifier) @java__ref__type)
(field_access object: (identifier) @java__ref__type)
(import_declaration (scoped_identifier) @java__ref__wildcard (asterisk))
(import_declaration "static" (scoped_identifier) @java__ref__static)
`}
}

func withoutReferenceCaptures(snippets []*file.Snippet) []*file.Snippet {
	out := snippets[:0]
	for _, s := range snippets {
		if !strings.HasPrefix(s.Type, "java__ref__") && s.Type != "java__type__span" {
			out = append(out, s)
		}
	}
	return out
}

func (e *Extension) Init(settings core.Analyzer) error {
	loadedDefs, err := definitions.LoadYamlFiles(javaDefs)
	if err == nil {
		for _, def := range loadedDefs {
			settings.AddDefinition(def)
		}
	}
	// A class name describes a file, not a group of them. The last-record
	// merger stamped one arbitrary class on every component, directory and on
	// the codebase summary; a group now carries a name only when all of its
	// files agree on it.
	settings.RegisterStatAccumulator("java_class", stats.SameOrNothingStatMerger)
	settings.RegisterStatAccumulator("java_full_class", stats.SameOrNothingStatMerger)

	settings.RegisterFileAnalyzer(&javaAnalyzer{lp: e.createJavaLanguagePack()})

	settings.RegisterView(&core.ViewFactory{
		Name:           "java_class_connections_direct",
		CreateViewFunc: ClassConnectionsDirectView,
	})

	if e.ClassIndirect {
		settings.RegisterView(&core.ViewFactory{
			Name:           "java_class_connections_indirect",
			CreateViewFunc: ClassConnectionsIndirectView,
		})
	}

	return nil
}

//go:embed common_java_packages.txt
var commonJavaImports string

func (e *Extension) createJavaLanguagePack() *common.LanguagePack {

	language := tree_sitter.NewLanguage(java.Language())
	ignoreList := e.getIgnoreList()
	var allQueriesForStats []string
	allQueriesForStats = append(allQueriesForStats, springQueriesForStats()...)
	allQueriesForStats = append(allQueriesForStats, modularityQueries(ignoreList)...)

	// Counts whose every match is also a snippet of the singular name.
	var allQueriesForCounts []string
	allQueriesForCounts = append(allQueriesForCounts, springQueriesForCounts()...)
	allQueriesForCounts = append(allQueriesForCounts, jpaQueriesForStats()...)
	allQueriesForCounts = append(allQueriesForCounts, javaQueriesForStats()...)

	var allQueriesForSnippets []string
	allQueriesForSnippets = append(allQueriesForSnippets, springQueriesForSnippets()...)
	allQueriesForSnippets = append(allQueriesForSnippets, jpaQueriesForSnippets()...)
	allQueriesForSnippets = append(allQueriesForSnippets, javaQueriesForSnippets(ignoreList)...)
	allQueriesForSnippets = append(allQueriesForSnippets, javaFrameworkFactQueries()...)
	allQueriesForSnippets = append(allQueriesForSnippets, javaReferenceQueries()...)

	allQueriesForSnippets = append(allQueriesForSnippets, e.ExtraQueries...)

	lp := &common.LanguagePackTemplate{
		FileGlob:           "**.java",
		Complexity:         complexity,
		Language:           language,
		QueriesForStats:    allQueriesForStats,
		QueriesForSnippets: allQueriesForSnippets,
		QueriesForCounts:   allQueriesForCounts,
	}
	template, err := common.PackFromTemplate(lp)
	if err != nil {
		panic(err)
	}
	return template
}

func (e *Extension) getIgnoreList() []string {
	var ignoreList []string
	if e.IgnoreCommonJavaImports {
		ignoreList = strings.Split(commonJavaImports, "\n")
	}
	ignoreList = append(ignoreList, e.IgnoreImportsFor...)
	return ignoreList
}

// springQueriesForCounts count the classes springQueriesForSnippets record,
// one for one: java__spring__beans matches exactly where java__spring__bean does.
func springQueriesForCounts() []string {
	return []string{
		createQueryForClassAnnotation("java__spring__controllers", "^(Controller|RestController)$"),
		createQueryForClassAnnotation("java__spring__services", "^Service$"),
		createQueryForClassAnnotation("java__spring__repositories", "^Repository$"),
		createQueryForClassAnnotation("java__spring__components", "^Component$"),
		createQueryForClassAnnotation("java__spring__configurations", "^Configuration$"),
		createQueryForClassAnnotation("java__spring__beans", "^(Component|Service|Repository|Controller|RestController|Configuration)$"),
	}
}

func springQueriesForStats() []string {
	return []string{
		createQueryForMethodAnnotation("java__spring__request_mappings__total", "^(Request|Get|Put|Post|Delete|Patch)Mapping$"),
		createQueryForMethodAnnotation("java__spring__request_mappings__get", "^GetMapping$"),
		createQueryForMethodAnnotation("java__spring__request_mappings__put", "^PutMapping$"),
		createQueryForMethodAnnotation("java__spring__request_mappings__post", "^PostMapping$"),
		createQueryForMethodAnnotation("java__spring__request_mappings__delete", "^DeleteMapping$"),
		createQueryForMethodAnnotation("java__spring__request_mappings__patch", "^PatchMapping$"),
		createQueryForRequestMapping("java__spring__request_mappings__get", "GET"),
		createQueryForRequestMapping("java__spring__request_mappings__put", "PUT"),
		createQueryForRequestMapping("java__spring__request_mappings__post", "POST"),
		createQueryForRequestMapping("java__spring__request_mappings__delete", "DELETE"),
		createQueryForRequestMapping("java__spring__request_mappings__patch", "PATCH"),
	}
}

func springQueriesForSnippets() []string {
	return []string{
		createQueryForClassAnnotationReferringBackToName("java__spring__controller", "^(Controller|RestController)$"),
		createQueryForClassAnnotationReferringBackToName("java__spring__service", "^Service$"),
		createQueryForClassAnnotationReferringBackToName("java__spring__repository", "^Repository$"),
		createQueryForClassAnnotationReferringBackToName("java__spring__component", "^Component$"),
		createQueryForClassAnnotationReferringBackToName("java__spring__configuration", "^Configuration$"),
		createQueryForClassAnnotationReferringBackToName("java__spring__bean", "^(Component|Service|Repository|Controller|RestController|Configuration)$"),
	}
}

// Framework-neutral facts about every type: which annotations sit on it, what
// it extends and what it implements. The UI maps these to roles per framework
// (Spring, Jakarta EE, Apache Beam, …) without the engine knowing any of them.
// The snippet content is the simple name of the annotation or type: a
// qualified `@org.springframework.stereotype.Service`, written where two
// frameworks both have a Service, is a Service like the imported one.
func javaFrameworkFactQueries() []string {
	annotationName := `[
	(annotation name: (identifier) @java__class__annotation)
	(marker_annotation name: (identifier) @java__class__annotation)
	(annotation name: (scoped_identifier name: (identifier) @java__class__annotation))
	(marker_annotation name: (scoped_identifier name: (identifier) @java__class__annotation))
]`
	return []string{
		`
(class_declaration (modifiers ` + annotationName + `))
(interface_declaration (modifiers ` + annotationName + `))
(record_declaration (modifiers ` + annotationName + `))
(enum_declaration (modifiers ` + annotationName + `))
`,
		// Whole declarations, so a marker can be given to the declaration it
		// was written on and a nested type named through the one around it.
		`
(class_declaration) @java__type__span
(interface_declaration) @java__type__span
(record_declaration) @java__type__span
(enum_declaration) @java__type__span
(annotation_type_declaration) @java__type__span
`,
		`
(class_declaration (superclass [
	(type_identifier) @java__class__extends
	(generic_type (type_identifier) @java__class__extends)
	(scoped_type_identifier (type_identifier) @java__class__extends)
	(generic_type (scoped_type_identifier (type_identifier) @java__class__extends))
]))
`,
		`
(class_declaration (super_interfaces (type_list [
	(type_identifier) @java__class__implements
	(generic_type (type_identifier) @java__class__implements)
	(scoped_type_identifier (type_identifier) @java__class__implements)
	(generic_type (scoped_type_identifier (type_identifier) @java__class__implements))
])))
(interface_declaration (extends_interfaces (type_list [
	(type_identifier) @java__class__implements
	(generic_type (type_identifier) @java__class__implements)
	(scoped_type_identifier (type_identifier) @java__class__implements)
	(generic_type (scoped_type_identifier (type_identifier) @java__class__implements))
])))
(record_declaration (super_interfaces (type_list [
	(type_identifier) @java__class__implements
	(generic_type (type_identifier) @java__class__implements)
])))
(enum_declaration (super_interfaces (type_list [
	(type_identifier) @java__class__implements
	(generic_type (type_identifier) @java__class__implements)
])))
`,
	}
}

func createQueryForClassAnnotationReferringBackToName(snippetType, annotationRegex string) string {
	return fmt.Sprintf(`
((class_declaration
	(modifiers [
    	(annotation name: ((identifier) @_annotation_name)) 
        (marker_annotation name: ((identifier)@_annotation_name))
        ]) 
     name: (identifier) @%s
)
(#match? @_annotation_name "%s")
)
((interface_declaration
	(modifiers [
    	(annotation name: ((identifier) @_annotation_name)) 
        (marker_annotation name: ((identifier)@_annotation_name))
        ]) 
     name: (identifier) @%s
)
(#match? @_annotation_name "%s")
)
`, snippetType, annotationRegex, snippetType, annotationRegex)
}

func jpaQueriesForStats() []string {
	return []string{
		createQueryForClassAnnotation("java__jpa__entities", "^Entity$"),
	}
}

func jpaQueriesForSnippets() []string {
	return []string{
		createQueryForClassAnnotationReferringBackToName("java__jpa__entity", "^Entity$"),
	}
}

func javaQueriesForSnippets(ignoreImportsFor []string) []string {
	ignoreListSplitted := lo.Map(ignoreImportsFor, func(imp string, _ int) string {
		return fmt.Sprintf("(#not-match? @java__import__declaration \"^%s\")", imp)
	})

	ignoreList := strings.Join(ignoreListSplitted, "\n")
	return []string{
		fmt.Sprintf(`
(class_declaration name: (identifier) @java__class__declaration)
(interface_declaration name: (identifier) @java__interface__declaration)
(record_declaration name: (identifier) @java__record__declaration)

((interface_declaration name: (identifier) @java__type__declaration))
((class_declaration name: (identifier) @java__type__declaration))
((record_declaration name: (identifier) @java__type__declaration))
((enum_declaration name: (identifier) @java__type__declaration))
((annotation_type_declaration name: (identifier) @java__type__declaration))

(field_declaration (variable_declarator name: (identifier) @java__field__declaration))
(method_declaration name: (identifier) @java__method__declaration)
(import_declaration (scoped_identifier) @java__import__declaration
%s
)
(import_declaration (scoped_identifier) @modularity__import__raw
%s
)
`, ignoreList, ignoreList),
	}
}
func javaQueriesForStats() []string {
	return []string{
		`
(class_declaration name: (identifier) @java__class__declarations)
(field_declaration (variable_declarator name: (identifier) @java__field__declarations))
(method_declaration name: (identifier) @java__method_declarations)
`}
}

func createQueryForRequestMapping(statName, method string) string {
	return fmt.Sprintf(`
((method_declaration (modifiers [
    	(annotation 
        	name: ((identifier) @_annotation_name) 
            arguments: 
            	(annotation_argument_list 
            		(element_value_pair 
                    	key: (identifier) @_argument
                        value: ([(identifier) (field_access)]) @_value
         ))) 
        (marker_annotation name: ((identifier)@_annotation_name))
        ]) 
   name: (identifier) @%s
)
(#match? @_annotation_name "^RequestMapping$")
(#match? @_argument "^method$")
(#match? @_value "%s")
)
`, statName, method)
}

func createQueryForClassAnnotation(statName, annotationRegex string) string {
	return fmt.Sprintf(`
((class_declaration
	(modifiers [
    	(annotation name: ((identifier) @_annotation_name)) 
        (marker_annotation name: ((identifier)@_annotation_name))
        ]) 
	name: (identifier) @%s
)
(#match? @_annotation_name "%s")
)
((interface_declaration
	(modifiers [
    	(annotation name: ((identifier) @_annotation_name)) 
        (marker_annotation name: ((identifier)@_annotation_name))
        ] )
	name: (identifier) @%s
)
(#match? @_annotation_name "%s")
)
`, statName, annotationRegex, statName, annotationRegex)
}

func createQueryForMethodAnnotation(statName, annotationRegex string) string {
	return fmt.Sprintf(`
((method_declaration
	(modifiers [
		(annotation name: ((identifier) @_annotation_name)) 
		(marker_annotation name: ((identifier)@_annotation_name))
		]) 
	name: (identifier) @%s
)
(#match? @_annotation_name "%s")
)`, statName, annotationRegex)
}

func modularityQueries(ignoreImportsFor []string) []string {
	ignoreListSplitted := lo.Map(ignoreImportsFor, func(imp string, _ int) string {
		return fmt.Sprintf("(#not-match? @modularity__component__imports \"^%s\")", imp)
	})

	ignoreList := strings.Join(ignoreListSplitted, "\n")

	return []string{
		`(package_declaration  (scoped_identifier) @modularity__component__declarations)`,
		`
((interface_declaration name: (identifier) @modularity__types__total))
((class_declaration  name: (identifier) @modularity__types__total))
((record_declaration name: (identifier) @modularity__types__total))
;  An enum is a type; elepy has 13 that went uncounted, and abstractness
;  is abstract over total, so leaving them out overstates it.
((enum_declaration name: (identifier) @modularity__types__total))
`,
		`
((interface_declaration name: (identifier) @modularity__types__abstract))
((class_declaration ((modifiers) @_modifiers) name: (identifier) @modularity__types__abstract) (#match? @_modifiers "abstract"))
`,
		fmt.Sprintf(`
(
  ((import_declaration 
    ((scoped_identifier scope: (scoped_identifier) @modularity__component__imports))) @_import ) 
    (#not-match? @_import "(^import static)|[*]")
	%s
)
(
  ((import_declaration 
    ((scoped_identifier) @modularity__component__imports) (asterisk)) @_import)
    (#not-match? @_import "(^import static)")
    (#match? @_import "[*]")
	%s
)
(
  ((import_declaration
    ((scoped_identifier scope: (scoped_identifier) @modularity__component__imports))) @_import ) 
      (#match? @_import "(^import static)")
      (#not-match? @_import "[*]")
	  %s
)
(
  ((import_declaration
      ((scoped_identifier) @modularity__component__imports) (asterisk)) @_import)
      (#match? @_import "(^import static)")
      (#match? @_import "[*]")
	  %s
)
`, ignoreList, ignoreList, ignoreList, ignoreList),
	}
}
