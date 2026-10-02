// Package navigation reads what an architect finds their way by: where the
// outside world gets in (entry points), what the code stores and who touches
// it (data), where implementations are bound to what they implement (wiring),
// and what the team wrote down about it (docs).
//
// These are not metrics. A metric says how much; these say where, and they
// are what a model reading one file at a time cannot see. Each is read
// lexically -- comments blanked, strings kept -- from how a framework spells
// it, and every row carries the file and line it was read from, so a reader
// can check it rather than trust it. Nothing here is a guess from a name.
//
// One pass reads each file and keeps only the small facts; linking them to
// units, functions and components waits until those are resolved, as the
// deployables extension does.
package navigation

import (
	"path"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/archstats/archstats/core"
	"github.com/archstats/archstats/core/file"
	"github.com/archstats/archstats/core/unit"
)

// The largest file read: a minified bundle or a generated client is neither
// an entry point nor a decision record, and a regex over it costs seconds.
const maxFileSize = 1 << 20

type facts struct {
	lang     string
	entries  []entry
	entities []entity
	accesses []access
	bindings []binding
	doc      *doc
	types    []typeDecl
	imports  []routeImport
}

type extension struct {
	mu    sync.Mutex
	files map[string]*facts
	// Filled by EditResults, read by the views.
	entryRows   []entryRow
	entityRows  []entityRow
	accessRows  []accessRow
	superRows   []superRow
	bindingRows []bindingRow
	docRows     []*doc
	linkRows    []linkRow
}

// Extension reads entry points, data, wiring and docs.
func Extension() core.Extension {
	return &extension{}
}

func (e *extension) Init(a core.Analyzer) error {
	*e = extension{files: map[string]*facts{}}
	a.RegisterFileAnalyzer(e)
	a.RegisterResultsEditor(e)
	for _, v := range []*core.ViewFactory{
		{Name: "entry_points", CreateViewFunc: e.entryPointsView},
		{Name: "data_entities", CreateViewFunc: e.entitiesView},
		{Name: "data_access", CreateViewFunc: e.accessView},
		{Name: "unit_supertypes", CreateViewFunc: e.supertypesView},
		{Name: "bindings", CreateViewFunc: e.bindingsView},
		{Name: "docs", CreateViewFunc: e.docsView},
		{Name: "doc_links", CreateViewFunc: e.docLinksView},
	} {
		a.RegisterView(v)
	}
	return nil
}

func (e *extension) AnalyzeFile(f file.File) *file.Results {
	p := clean(f.Path())
	lang := languageOf(p)
	if lang == langUnreadable {
		return nil
	}
	content := f.Content()
	if len(content) > maxFileSize {
		return nil
	}
	s := newSource(p, lang, content)
	fx := &facts{lang: lang}
	if lang == langMarkdown {
		fx.doc = s.document()
	} else {
		fx.entries = append(s.entries(), fileRoutes(p)...)
		routes, imports := s.yamlRoutes()
		fx.entries, fx.imports = append(fx.entries, routes...), imports
		fx.entities = s.entities()
		fx.accesses = s.accesses()
		fx.bindings = s.bindings()
		fx.types = s.typeDecls()
	}
	if fx.doc == nil && len(fx.imports) == 0 && len(fx.entries) == 0 && len(fx.entities) == 0 && len(fx.accesses) == 0 && len(fx.bindings) == 0 && len(fx.types) == 0 {
		return nil
	}
	e.mu.Lock()
	e.files[p] = fx
	e.mu.Unlock()
	return nil
}

func clean(p string) string { return strings.TrimPrefix(p, "./") }

// own is whether a file is the codebase's own production code, the only
// code whose routes and tables describe the system: a test's routes are
// fixtures, a vendored library's are someone else's.
func own(results *core.Results, f string) bool {
	switch roleOf(results, f) {
	case "test", "generated", "third_party":
		return false
	}
	return true
}

func roleOf(results *core.Results, f string) string {
	if r, ok := results.FileRoles[f]; ok {
		return r
	}
	return results.FileRoles["./"+f]
}

func componentOf(results *core.Results, f string) string {
	if c, ok := results.FileToComponent[f]; ok {
		return c
	}
	return results.FileToComponent["./"+f]
}

// linker holds the lookups every fact is linked through.
type linker struct {
	results   *core.Results
	functions map[string][]*file.Function
	units     map[string][]*unit.Unit
	byName    map[string][]*unit.Unit
	uses      map[string]map[string]bool
	types     map[string][]typeDecl
	from      map[string][]string
	// services are container ids bound to a class: "sylius.controller.x".
	services map[string]*unit.Unit
}

func newLinker(results *core.Results, files map[string]*facts) *linker {
	l := &linker{services: map[string]*unit.Unit{}, results: results, functions: map[string][]*file.Function{}, units: map[string][]*unit.Unit{}, byName: map[string][]*unit.Unit{}, uses: map[string]map[string]bool{}, types: map[string][]typeDecl{}}
	for f, fns := range results.FunctionsByFile {
		l.functions[clean(f)] = fns
	}
	for f, us := range results.UnitsByFile {
		l.units[clean(f)] = append(l.units[clean(f)], us...)
	}
	for _, u := range results.Units {
		l.byName[u.Name] = append(l.byName[u.Name], u)
	}
	for comp, conns := range results.ConnectionsFrom {
		set := map[string]bool{}
		for _, c := range conns {
			set[c.To] = true
		}
		l.uses[comp] = set
	}
	for f, fx := range files {
		l.types[f] = fx.types
	}
	return l
}

// functionAt is the innermost measured function holding a line.
func (l *linker) functionAt(f string, line int) *file.Function {
	var best *file.Function
	for _, fn := range l.functions[f] {
		if fn.Begin <= line && line <= fn.End && (best == nil || fn.Begin >= best.Begin) {
			best = fn
		}
	}
	return best
}

// typeAt is the type declared nearest above a line.
func (l *linker) typeAt(f string, line int) string {
	name := ""
	for _, t := range l.types[f] {
		if t.Line <= line {
			name = t.Name
		}
	}
	return name
}

// unitNamed is the unit called name declared in a file.
func (l *linker) unitNamed(f, name string) *unit.Unit {
	var fallback *unit.Unit
	for _, u := range l.units[f] {
		if u.Name == name {
			if u.Kind == unit.KindType {
				return u
			}
			if fallback == nil {
				fallback = u
			}
		}
	}
	return fallback
}

// unitOfFunction is the unit a measured function belongs to: its type, for a
// method; itself, for a function that is a unit.
func (l *linker) unitOfFunction(f string, fn *file.Function) *unit.Unit {
	if fn == nil {
		return nil
	}
	parts := strings.Split(fn.Name, ".")
	for i := len(parts) - 2; i >= 0; i-- {
		if u := l.unitNamed(f, parts[i]); u != nil && u.Kind == unit.KindType {
			return u
		}
	}
	if u := l.unitNamed(f, parts[len(parts)-1]); u != nil {
		return u
	}
	if len(l.units[f]) == 1 {
		return l.units[f][0]
	}
	return nil
}

// resolve finds the unit a name in a file means: one in the same file, then
// one in a component the file's component imports, then the only one of
// that name anywhere. A qualified name is looked up as an id first.
func (l *linker) resolve(name, from string) *unit.Unit {
	if name == "" {
		return nil
	}
	if u := l.results.UnitByID[name]; u != nil {
		return u
	}
	if strings.Contains(name, `\`) {
		if u := l.results.UnitByID[strings.ReplaceAll(name, `\`, ".")]; u != nil {
			return u
		}
	}
	simple := name[strings.LastIndexAny(name, `.\$`)+1:]
	if u := l.unitNamed(from, simple); u != nil {
		return u
	}
	candidates := l.byName[simple]
	if strings.ContainsAny(name, `.\`) {
		// A qualified name narrows the candidates to those whose id ends with it.
		var exact []*unit.Unit
		norm := strings.ReplaceAll(name, `\`, ".")
		for _, u := range candidates {
			if strings.HasSuffix(strings.ReplaceAll(u.ID, `\`, "."), norm) {
				exact = append(exact, u)
			}
		}
		if len(exact) == 1 {
			return exact[0]
		}
	}
	comp := componentOf(l.results, from)
	var near []*unit.Unit
	for _, u := range candidates {
		if u.Component == comp || l.uses[comp][u.Component] {
			near = append(near, u)
		}
	}
	if len(near) == 1 {
		return near[0]
	}
	var types []*unit.Unit
	for _, u := range candidates {
		if u.Kind == unit.KindType {
			types = append(types, u)
		}
	}
	if len(candidates) == 1 {
		return candidates[0]
	}
	if len(types) == 1 {
		return types[0]
	}
	return nil
}

// functionNamed is a member of a unit by its own name, found in the unit's
// files.
func (l *linker) functionNamed(u *unit.Unit, member string) *file.Function {
	if u == nil || member == "" {
		return nil
	}
	for _, f := range u.Files {
		for _, fn := range l.functions[clean(f)] {
			if fn.Name == member || strings.HasSuffix(fn.Name, "."+member) && (u.Kind != unit.KindType || strings.Contains(fn.Name, u.Name+".")) {
				return fn
			}
		}
	}
	return nil
}

func firstFile(u *unit.Unit) string {
	if u == nil || len(u.Files) == 0 {
		return ""
	}
	return clean(u.Files[0])
}

func idOf(u *unit.Unit) string {
	if u == nil {
		return ""
	}
	return u.ID
}

func (e *extension) EditResults(results *core.Results) {
	files := e.files
	e.files = nil
	paths := make([]string, 0, len(files))
	for p := range files {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	l := newLinker(results, files)

	e.placeUnits(l)
	e.linkWiring(l, files, paths)
	e.linkEntries(l, files, paths)
	e.linkData(l, files, paths)
	e.linkDocs(l, files, paths)
}

// placeUnits gives every unit the line it is declared on and its signature:
// a function's from its measurement, a type's from its declaration line.
func (e *extension) placeUnits(l *linker) {
	for _, u := range l.results.Units {
		if u.Line > 0 {
			continue
		}
		f := firstFile(u)
		if u.Kind == unit.KindType {
			for _, t := range l.types[f] {
				if t.Name == u.Name {
					u.Line, u.Signature = t.Line, t.Signature
					break
				}
			}
			continue
		}
		owner := ""
		if o := l.results.UnitByID[u.Owner]; o != nil {
			owner = o.Name
		}
		for _, fn := range l.functions[f] {
			last := fn.Name[strings.LastIndexByte(fn.Name, '.')+1:]
			if last != u.Name {
				continue
			}
			if owner != "" && !strings.Contains(fn.Name, owner+".") {
				continue
			}
			u.Line, u.Signature = fn.Begin, fn.Signature
			break
		}
	}
}

type entryRow struct {
	Kind, Method, Path, Framework, Handler, Unit, Function, Component, File string
	Line                                                                    int
}

// Frameworks that read routes from where files are only do so when their
// config is in the workspace; a pages/ folder alone proves nothing.
var fileRouteConfigs = map[string][]string{
	"next":      {"next.config.js", "next.config.mjs", "next.config.ts", "next.config.cjs"},
	"nuxt":      {"nuxt.config.ts", "nuxt.config.js", "nuxt.config.mjs"},
	"sveltekit": {"svelte.config.js", "svelte.config.ts"},
}

func (e *extension) linkEntries(l *linker, files map[string]*facts, paths []string) {
	configs := map[string]bool{}
	for f := range l.results.FileRoles {
		configs[path.Base(f)] = true
	}
	present := func(framework string) bool {
		for _, c := range fileRouteConfigs[framework] {
			if configs[c] {
				return true
			}
		}
		return false
	}
	prefix := routePrefixes(files, paths)
	seen := map[string]bool{}
	for _, p := range paths {
		if !own(l.results, p) {
			continue
		}
		for _, x := range files[p].entries {
			if pre := prefix[p]; pre != "" && x.Framework == "symfony" {
				x.Path = joinRoute(pre, x.Path)
			}
			if x.Framework == "pages" {
				switch {
				case present("next"):
					x.Framework = "next"
				case present("nuxt"):
					x.Framework = "nuxt"
				default:
					continue
				}
			} else if _, byPath := fileRouteConfigs[x.Framework]; byPath && x.Line == 1 && x.DeclLine == 1 && !present(x.Framework) {
				continue
			}
			row := entryRow{Kind: x.Kind, Method: x.Method, Path: x.Path, Framework: x.Framework, Handler: x.Handler, Component: componentOf(l.results, p), File: p, Line: x.Line}
			var u *unit.Unit
			var fn *file.Function
			switch {
			case x.Handler != "":
				name, member := x.Handler, ""
				if i := strings.LastIndexByte(name, '.'); i > 0 {
					name, member = name[:i], name[i+1:]
				}
				u = l.resolve(name, p)
				if u == nil {
					// A container's service id: sylius_admin.controller.dashboard.
					if svc := l.services[x.Handler]; svc != nil {
						u, member = svc, ""
					} else if svc := l.services[name]; svc != nil {
						u = svc
					}
				}
				if u == nil && member == "" {
					// A handler named as a bare function in the same file.
					u = l.unitNamed(p, name)
				}
				fn = l.functionNamed(u, member)
				if fn == nil && u != nil && u.Kind == unit.KindFunction {
					fn = l.functionNamed(u, u.Name)
				}
			case x.Member != "" || x.Kind == kindMain && x.DeclLine > 1:
				fn = l.functionAt(p, x.DeclLine)
				u = l.unitOfFunction(p, fn)
				if u == nil && x.Class != "" {
					u = l.unitNamed(p, x.Class)
				}
			case x.Class != "":
				u = l.unitNamed(p, x.Class)
				if x.Kind == kindMessage || x.Kind == kindCLI {
					fn = l.functionNamed(u, "__invoke")
					if fn == nil {
						fn = l.functionNamed(u, "handle")
					}
				}
			default:
				// A file-routed page or a program's main: the file's own unit.
				stem := strings.TrimSuffix(path.Base(p), path.Ext(path.Base(p)))
				u = l.unitNamed(p, stem)
				if u == nil && len(l.units[p]) > 0 {
					u = l.units[p][0]
				}
			}
			row.Unit = idOf(u)
			if fn != nil {
				row.Function = fn.Name
			}
			if row.Handler == "" && fn != nil {
				row.Handler = fn.Name
			}
			key := strings.Join([]string{row.Kind, row.Method, row.Path, row.File, itoa(row.Line)}, "\x00")
			if seen[key] {
				continue
			}
			seen[key] = true
			e.entryRows = append(e.entryRows, row)
		}
	}
	e.conventionalActions(l, paths, seen)
	sortEntries(e.entryRows)
}

var actionResult = regexp.MustCompile(`^public\s+(?:virtual\s+|override\s+|async\s+)*(?:Task\s*<\s*)?(?:I?ActionResult|JsonResult|ViewResult|PartialViewResult|ContentResult|FileResult|RedirectResult|RedirectToActionResult|IResult)\b`)

// conventionalActions are ASP.NET MVC actions routed by convention: a public
// method returning an action result on a controller, reached at
// /{controller}/{action} unless an attribute says otherwise. An action an
// attribute already routed keeps that route; one with an HTTP verb attribute
// but no template gets the conventional path.
func (e *extension) conventionalActions(l *linker, paths []string, seen map[string]bool) {
	routed := map[string]int{}
	for i, r := range e.entryRows {
		if r.Framework == "aspnet" && r.Function != "" {
			routed[r.File+"\x00"+r.Function] = i + 1
		}
	}
	for _, p := range paths {
		if languageOf(p) != langCSharp || !own(l.results, p) {
			continue
		}
		for _, fn := range l.functions[p] {
			parts := strings.Split(fn.Name, ".")
			if len(parts) < 2 || !strings.HasSuffix(parts[len(parts)-2], "Controller") || !actionResult.MatchString(fn.Signature) {
				continue
			}
			ctrl, action := strings.TrimSuffix(parts[len(parts)-2], "Controller"), strings.TrimSuffix(parts[len(parts)-1], "Async")
			conventional := "/" + ctrl + "/" + action
			if i := routed[p+"\x00"+fn.Name]; i > 0 {
				if r := &e.entryRows[i-1]; r.Path == "/" {
					r.Path, r.Framework = conventional, "aspnet-conventional"
				}
				continue
			}
			u := l.unitOfFunction(p, fn)
			row := entryRow{Kind: kindHTTP, Method: "ANY", Path: conventional, Framework: "aspnet-conventional", Handler: fn.Name, Unit: idOf(u), Function: fn.Name, Component: componentOf(l.results, p), File: p, Line: fn.Begin}
			key := strings.Join([]string{row.Kind, row.Method, row.Path, row.File, itoa(row.Line)}, "\x00")
			if seen[key] {
				continue
			}
			seen[key] = true
			e.entryRows = append(e.entryRows, row)
		}
	}
}

// routePrefixes is the prefix every routing file's routes are mounted under,
// joined through the files that import it.
func routePrefixes(files map[string]*facts, paths []string) map[string]string {
	type edge struct{ from, prefix string }
	parents := map[string][]edge{}
	var routing []string
	for _, p := range paths {
		if isRoutingFile(p) {
			routing = append(routing, p)
		}
	}
	for _, p := range paths {
		for _, im := range files[p].imports {
			for _, target := range importTargets(p, im.Resource, routing) {
				parents[target] = append(parents[target], edge{p, im.Prefix})
			}
		}
	}
	out := map[string]string{}
	var walk func(f string, depth int) string
	walk = func(f string, depth int) string {
		if v, ok := out[f]; ok || depth > 8 {
			return v
		}
		out[f] = ""
		if ps := parents[f]; len(ps) > 0 {
			out[f] = joinRoute(walk(ps[0].from, depth+1), ps[0].prefix)
			if out[f] == "/" {
				out[f] = ""
			}
		}
		return out[f]
	}
	for _, f := range routing {
		walk(f, 0)
	}
	return out
}

// importTargets are the routing files a resource names: a path relative to
// the importing file, or a bundle path (`@SyliusAdminBundle/Resources/...`)
// matched by what follows the bundle, preferring the directory the bundle's
// name ends with.
func importTargets(from, resource string, routing []string) []string {
	resource = strings.TrimSpace(resource)
	if !strings.HasPrefix(resource, "@") {
		want := path.Join(path.Dir(from), resource)
		for _, r := range routing {
			if r == want {
				return []string{r}
			}
		}
		return nil
	}
	slash := strings.IndexByte(resource, '/')
	if slash < 0 {
		return nil
	}
	bundle, rest := resource[1:slash], resource[slash:]
	var matches []string
	for _, r := range routing {
		if strings.HasSuffix(r, rest) {
			matches = append(matches, r)
		}
	}
	if len(matches) <= 1 {
		return matches
	}
	for i := 1; i < len(bundle); i++ {
		if bundle[i] >= 'A' && bundle[i] <= 'Z' {
			for _, r := range matches {
				if strings.Contains(r, "/"+bundle[i:]+"/") {
					return []string{r}
				}
			}
		}
	}
	return nil
}

type entityRow struct {
	Entity, Name, Table, Store, Framework, File string
	Line                                        int
}

type accessRow struct {
	Unit, Function, Component, Target, Entity, Access, Via, File string
	Line                                                         int
}

// Supertypes that make a class a mapped model, by framework, for ORMs that
// say so by inheritance rather than by annotation.
var modelBases = map[string]string{
	"models.Model": "django", "Model": "", "db.Model": "flask-sqlalchemy", "DeclarativeBase": "sqlalchemy",
	"Document": "mongoengine", "SQLModel": "sqlmodel",
}

func (e *extension) linkData(l *linker, files map[string]*facts, paths []string) {
	byKey := map[string]*entityRow{}
	var order []string
	addEntity := func(r entityRow) {
		key := r.Entity
		if key == "" {
			key = r.Framework + ":" + r.Name + ":" + r.File
		}
		if existing, ok := byKey[key]; ok {
			if existing.Table == "" {
				existing.Table = r.Table
			}
			return
		}
		cp := r
		byKey[key] = &cp
		order = append(order, key)
	}
	for _, p := range paths {
		if !own(l.results, p) && files[p].lang != langXML && files[p].lang != langPrisma {
			continue
		}
		for _, x := range files[p].entities {
			r := entityRow{Name: x.Name, Table: x.Table, Store: x.Store, Framework: x.Framework, File: p, Line: x.Line}
			if x.Class != "" {
				var u *unit.Unit
				if strings.ContainsAny(x.Class, `.\$`) {
					u = l.resolve(strings.ReplaceAll(x.Class, "$", "."), p)
				} else {
					u = l.unitNamed(p, x.Class)
				}
				r.Entity = idOf(u)
				if u != nil {
					r.Name = u.Name
				}
			}
			if r.Name == "" {
				continue
			}
			addEntity(r)
		}
	}
	// Models an ORM recognises by what they inherit: Django, Eloquent,
	// Flask-SQLAlchemy. A model's subclass is a model.
	memo := map[string]string{}
	var frameworkOf func(u *unit.Unit, depth int) string
	frameworkOf = func(u *unit.Unit, depth int) string {
		if fw, ok := memo[u.ID]; ok {
			return fw
		}
		memo[u.ID] = ""
		if depth > 12 {
			return ""
		}
		f := firstFile(u)
		lang := languageOf(f)
		for _, m := range u.Markers {
			if m.Source != unit.SourceSupertype {
				continue
			}
			key := genericless(m.Key)
			if fw, ok := modelBases[key]; ok && (lang == langPython || lang == langPHP) {
				switch {
				case fw != "":
				case lang == langPHP:
					fw = "eloquent"
				default:
					fw = "django"
				}
				if fw == "django" && lang == langPython && !strings.Contains(strings.Join(l.importsOf(f), " "), "django") && key != "models.Model" {
					continue
				}
				memo[u.ID] = fw
				return fw
			}
			if base := l.supertypeUnit(u, key); base != nil {
				if fw := frameworkOf(base, depth+1); fw != "" {
					memo[u.ID] = fw
					return fw
				}
			}
		}
		return ""
	}
	for _, u := range l.results.Units {
		if u.Kind != unit.KindType {
			continue
		}
		lang := languageOf(firstFile(u))
		if lang != langPython && lang != langPHP {
			continue
		}
		if !own(l.results, firstFile(u)) {
			continue
		}
		if fw := frameworkOf(u, 0); fw != "" && u.Name != "Meta" {
			addEntity(entityRow{Entity: u.ID, Name: u.Name, Store: "sql", Framework: fw, File: firstFile(u), Line: u.Line})
		}
	}
	for _, k := range order {
		e.entityRows = append(e.entityRows, *byKey[k])
	}
	sort.SliceStable(e.entityRows, func(i, j int) bool {
		a, b := e.entityRows[i], e.entityRows[j]
		if a.Name != b.Name {
			return a.Name < b.Name
		}
		return a.File < b.File
	})

	// What a repository stores is an entity, whatever declares it: nopCommerce's
	// entities are plain classes, known as entities only by IRepository<T>.
	for _, p := range paths {
		if !own(l.results, p) {
			continue
		}
		for _, x := range files[p].accesses {
			if x.Via != "repository" {
				continue
			}
			u := l.resolve(x.Target, p)
			if u == nil || u.Kind != unit.KindType {
				continue
			}
			if _, ok := byKey[u.ID]; ok {
				continue
			}
			addEntity(entityRow{Entity: u.ID, Name: u.Name, Framework: "repository", File: firstFile(u), Line: u.Line})
			e.entityRows = append(e.entityRows, *byKey[u.ID])
		}
	}
	sort.SliceStable(e.entityRows, func(i, j int) bool {
		a, b := e.entityRows[i], e.entityRows[j]
		if a.Name != b.Name {
			return a.Name < b.Name
		}
		return a.File < b.File
	})

	// Accesses, linked to the entity they name.
	byName := map[string][]*entityRow{}
	byTable := map[string][]*entityRow{}
	for i := range e.entityRows {
		r := &e.entityRows[i]
		byName[r.Name] = append(byName[r.Name], r)
		if r.Table != "" {
			t := strings.ToLower(r.Table)
			byTable[t] = append(byTable[t], r)
			if i := strings.LastIndexByte(t, '.'); i >= 0 {
				byTable[t[i+1:]] = append(byTable[t[i+1:]], r)
			}
		}
	}
	pick := func(list []*entityRow, from string) *entityRow {
		if len(list) == 1 {
			return list[0]
		}
		comp := componentOf(l.results, from)
		for _, r := range list {
			if r.Entity != "" && l.results.UnitByID[r.Entity] != nil && (l.results.UnitByID[r.Entity].Component == comp || l.uses[comp][l.results.UnitByID[r.Entity].Component]) {
				return r
			}
		}
		if len(list) > 0 {
			return list[0]
		}
		return nil
	}
	seen := map[string]bool{}
	for _, p := range paths {
		if !own(l.results, p) {
			continue
		}
		for _, x := range files[p].accesses {
			r := accessRow{Target: x.Target, Access: x.Access, Via: x.Via, Component: componentOf(l.results, p), File: p, Line: x.Line}
			switch x.Via {
			case "sql|jpql":
				if ent := pick(byName[x.Target], p); ent != nil {
					r.Via, r.Entity = "jpql", ent.Entity
				} else if ent := pick(byTable[strings.ToLower(x.Target)], p); ent != nil {
					r.Via, r.Entity = "sql", ent.Entity
				} else {
					r.Via = "sql"
				}
			case "sql":
				if ent := pick(byTable[strings.ToLower(x.Target)], p); ent != nil {
					r.Entity = ent.Entity
				}
			default:
				ent := pick(byName[x.Target], p)
				if ent == nil {
					if x.Via == "repository" || x.Via == "dbset" {
						// A repository of a type that is not mapped here: still the
						// repository's data, named by its type.
						if u := l.resolve(x.Target, p); u != nil && u.Kind == unit.KindType {
							r.Entity = u.ID
							break
						}
					}
					continue
				}
				if x.Via == "orm?" {
					r.Via = "orm"
				}
				r.Entity = ent.Entity
			}
			fn := l.functionAt(p, x.Line)
			u := l.unitOfFunction(p, fn)
			if u == nil {
				u = l.unitNamed(p, l.typeAt(p, x.Line))
			}
			if u == nil && len(l.units[p]) == 1 {
				u = l.units[p][0]
			}
			r.Unit = idOf(u)
			if fn != nil {
				r.Function = fn.Name
			}
			key := strings.Join([]string{r.File, itoa(r.Line), r.Target, r.Access, r.Via}, "\x00")
			if seen[key] {
				continue
			}
			seen[key] = true
			e.accessRows = append(e.accessRows, r)
		}
	}
}

// importsOf are the raw import specifiers of a file.
func (l *linker) importsOf(f string) []string {
	var out []string
	for _, s := range l.results.SnippetsByFile[f] {
		if strings.Contains(s.Type, "import") {
			out = append(out, s.Value)
		}
	}
	for _, s := range l.results.SnippetsByFile["./"+f] {
		if strings.Contains(s.Type, "import") {
			out = append(out, s.Value)
		}
	}
	return out
}

func genericless(name string) string {
	if i := strings.IndexAny(name, "<[("); i > 0 {
		name = name[:i]
	}
	return strings.TrimSpace(name)
}

// supertypeUnit is the unit a supertype marker names: the one this unit's own
// references resolved to, or failing that the one of that name its file or
// component can see.
func (l *linker) supertypeUnit(u *unit.Unit, name string) *unit.Unit {
	simple := name[strings.LastIndexAny(name, `.\:`)+1:]
	for _, to := range l.connectionsFrom()[u.ID] {
		if t := l.results.UnitByID[to]; t != nil && t.Name == simple && t.ID != u.ID {
			return t
		}
	}
	if t := l.resolve(name, firstFile(u)); t != nil && t.ID != u.ID && t.Kind == unit.KindType {
		return t
	}
	return nil
}

// connectionsFrom is the unit graph by the unit an edge starts at.
func (l *linker) connectionsFrom() map[string][]string {
	if l.from == nil {
		l.from = map[string][]string{}
		for _, c := range l.results.UnitConnections() {
			l.from[c.From] = append(l.from[c.From], c.To)
		}
	}
	return l.from
}

type superRow struct{ Unit, Supertype, Name string }

// Mechanisms that name what is asked for by a string id rather than a type.
var byString = map[string]bool{"spring_xml": true, "symfony_xml": true, "symfony_php": true, "symfony_alias": true}

type bindingRow struct {
	Interface, InterfaceUnit, Implementation, ImplementationUnit, Mechanism, File string
	Line                                                                          int
}

func isInterface(u *unit.Unit) bool {
	for _, m := range u.Markers {
		if m.Source == unit.SourceSupertype && (m.Key == "interface" || m.Key == "protocol") || m.Key == "interface" {
			return true
		}
	}
	return false
}

func (e *extension) linkWiring(l *linker, files map[string]*facts, paths []string) {
	supers := map[string][]*unit.Unit{}
	for _, u := range l.results.Units {
		if u.Kind != unit.KindType || !own(l.results, firstFile(u)) {
			continue
		}
		seen := map[string]bool{}
		for _, m := range u.Markers {
			if m.Source != unit.SourceSupertype || m.Key == "interface" || m.Key == "protocol" {
				continue
			}
			name := genericless(m.Key)
			if name == "" || seen[name] {
				continue
			}
			seen[name] = true
			t := l.supertypeUnit(u, name)
			e.superRows = append(e.superRows, superRow{Unit: u.ID, Supertype: idOf(t), Name: name})
			if t != nil {
				supers[u.ID] = append(supers[u.ID], t)
			}
		}
	}

	seen := map[string]bool{}
	add := func(r bindingRow) {
		key := strings.Join([]string{r.Interface, r.Implementation, r.Mechanism, r.File, itoa(r.Line)}, "\x00")
		if !seen[key] {
			seen[key] = true
			e.bindingRows = append(e.bindingRows, r)
		}
	}
	for _, p := range paths {
		if !own(l.results, p) && files[p].lang != langXML {
			continue
		}
		for _, b := range files[p].bindings {
			r := bindingRow{Interface: b.Interface, Implementation: b.Implementation, Mechanism: b.Mechanism, File: p, Line: b.Line}
			if !byString[b.Mechanism] || b.Mechanism == "symfony_alias" {
				r.InterfaceUnit = idOf(l.resolve(genericless(b.Interface), p))
			} else if u := l.resolve(b.Interface, p); u != nil && strings.ContainsAny(b.Interface, `.\`) {
				r.InterfaceUnit = u.ID
			}
			if b.Implementation == "" {
				r.Implementation = b.Interface
			}
			r.ImplementationUnit = idOf(l.resolve(genericless(r.Implementation), p))
			if byString[b.Mechanism] && b.Mechanism != "symfony_alias" && r.ImplementationUnit != "" {
				l.services[b.Interface] = l.results.UnitByID[r.ImplementationUnit]
			}
			add(r)
		}
	}
	// An alias names a service id; the id names the class.
	for i := range e.bindingRows {
		r := &e.bindingRows[i]
		if r.ImplementationUnit == "" {
			r.ImplementationUnit = idOf(l.services[r.Implementation])
		}
	}
	// Types a container finds by scanning, bound to the interfaces in this
	// codebase that they implement.
	for _, u := range l.results.Units {
		scannedBy := ""
		for _, m := range u.Markers {
			if m.Source == unit.SourceAnnotation && scanned[m.Key] {
				scannedBy = m.Key
				break
			}
		}
		if scannedBy == "" {
			continue
		}
		for _, t := range supers[u.ID] {
			if !isInterface(t) {
				continue
			}
			add(bindingRow{Interface: t.Name, InterfaceUnit: t.ID, Implementation: u.Name, ImplementationUnit: u.ID, Mechanism: "component_scan", File: firstFile(u), Line: u.Line})
		}
	}
	sort.SliceStable(e.bindingRows, func(i, j int) bool {
		a, b := e.bindingRows[i], e.bindingRows[j]
		if a.Interface != b.Interface {
			return a.Interface < b.Interface
		}
		if a.Implementation != b.Implementation {
			return a.Implementation < b.Implementation
		}
		return a.File < b.File
	})
}

type linkRow struct {
	Doc, Component, How string
	Mentions            int
}

func (e *extension) linkDocs(l *linker, files map[string]*facts, paths []string) {
	// Directory -> the components with files directly in it, and every file
	// and directory by path, for resolving what a doc names.
	inDir := map[string]map[string]bool{}
	allFiles := map[string]string{}
	for f, comp := range l.results.FileToComponent {
		f = clean(f)
		if comp == "" {
			continue
		}
		allFiles[f] = comp
		d := path.Dir(f)
		if inDir[d] == nil {
			inDir[d] = map[string]bool{}
		}
		inDir[d][comp] = true
	}
	under := func(dir string) map[string]bool {
		out := map[string]bool{}
		prefix := dir + "/"
		if dir == "." {
			prefix = ""
		}
		for d, comps := range inDir {
			if d == dir || strings.HasPrefix(d, prefix) {
				for c := range comps {
					out[c] = true
				}
			}
		}
		return out
	}
	components := map[string]bool{}
	for c := range l.results.ComponentToFiles {
		components[c] = true
	}
	for _, p := range paths {
		d := files[p].doc
		if d == nil {
			continue
		}
		if r := roleOf(l.results, p); r == "third_party" || r == "generated" {
			continue
		}
		e.docRows = append(e.docRows, d)
		counts := map[[2]string]int{}
		dir := path.Dir(p)
		if comps := inDir[dir]; len(comps) > 0 && len(comps) <= 8 {
			for c := range comps {
				counts[[2]string{c, "located_in"}]++
			}
		} else if dir != "." {
			if comps := under(dir); len(comps) > 0 && len(comps) <= 8 {
				for c := range comps {
					counts[[2]string{c, "located_in"}]++
				}
			}
		}
		for _, m := range d.Paths {
			m = strings.TrimPrefix(m, "./")
			for _, candidate := range []string{m, path.Join(dir, m)} {
				if comp, ok := allFiles[candidate]; ok {
					counts[[2]string{comp, "names_path"}]++
					break
				}
				if comps := under(strings.TrimSuffix(candidate, "/")); len(comps) > 0 && len(comps) <= 8 && candidate != "." {
					for c := range comps {
						counts[[2]string{c, "names_path"}]++
					}
					break
				}
			}
		}
		for _, span := range d.Spans {
			if components[span] && len(span) >= 4 {
				counts[[2]string{span, "names_component"}]++
				continue
			}
			if comp, ok := allFiles[strings.TrimPrefix(span, "./")]; ok {
				counts[[2]string{comp, "names_path"}]++
				continue
			}
			// A type or function named in code, if only one thing has that name.
			name := strings.TrimSuffix(strings.TrimSuffix(span, "()"), ".java")
			if !isQualified(name) || len(name) < 4 {
				continue
			}
			if u := l.results.UnitByID[name]; u != nil && u.Component != "" {
				counts[[2]string{u.Component, "names_unit"}]++
				continue
			}
			simple := name[strings.LastIndexAny(name, `.\`)+1:]
			if us := l.byName[simple]; len(us) == 1 && us[0].Component != "" && hasUpperOrUnderscore(simple) {
				counts[[2]string{us[0].Component, "names_unit"}]++
			}
		}
		keys := make([][2]string, 0, len(counts))
		for k := range counts {
			keys = append(keys, k)
		}
		sort.Slice(keys, func(i, j int) bool {
			if keys[i][0] != keys[j][0] {
				return keys[i][0] < keys[j][0]
			}
			return keys[i][1] < keys[j][1]
		})
		for _, k := range keys {
			e.linkRows = append(e.linkRows, linkRow{Doc: p, Component: k[0], How: k[1], Mentions: counts[k]})
		}
	}
}

func hasUpperOrUnderscore(s string) bool {
	return strings.ContainsAny(s, "_ABCDEFGHIJKLMNOPQRSTUVWXYZ")
}
