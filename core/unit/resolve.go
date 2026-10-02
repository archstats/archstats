package unit

import (
	"path"
	"sort"
	"strings"
)

// Turning what a unit said it used into an edge to the unit it meant.
//
// A pack records a reference the way the source wrote it: `parse`, from
// "./reader". Which unit that is depends on the whole codebase -- where
// "./reader" resolves to, and whether anything there is called `parse` --
// so it is worked out once, here, rather than by every pack separately.
//
// Java never needed this because it imports the type itself. Every other
// language imports a module and takes names out of it, which is why Java had
// a class graph and nothing else did.

// A Connection is one unit using another.
type Connection struct {
	From string
	To   string
	// What the source called the module it took the name from, kept so a
	// reader can see why the edge exists.
	Via string
}

// Index is the codebase, arranged for lookup.
type Index struct {
	// module -> name -> unit id
	byModuleAndName map[string]map[string]string
	// name -> every unit with that name, for the barrel search below
	byName map[string][]string
	// every module, longest first, for suffix matching
	modules []string
	// module -> its place in modules
	rank map[string]int
	// tail -> the places of the modules ending in "/"+tail, so a suffix
	// match is a lookup and not a pass over every module per reference
	endingIn map[string][]int
	// unit id -> the module part of its id
	moduleOf map[string]string
	fileOf   map[string]string
}

// NewIndex arranges units for resolution. A unit's id is `module#name` for
// the languages that qualify by module, and a dotted path for those that
// qualify by package; both split on the last separator.
func NewIndex(units []*Unit) *Index {
	idx := &Index{
		byModuleAndName: map[string]map[string]string{},
		byName:          map[string][]string{},
		moduleOf:        map[string]string{},
		fileOf:          map[string]string{},
	}
	seen := map[string]bool{}
	for _, u := range units {
		module, name := splitID(u.ID)
		// Where every unit lives is recorded, the file-level module unit
		// included: its relative imports resolve against it. Skipping it
		// with the unnamed units below left "./reader" resolved against
		// nothing, so no import a module made at top level ever landed.
		idx.moduleOf[u.ID] = module
		if len(u.Files) > 0 {
			idx.fileOf[u.ID] = u.Files[0]
		}
		// A unit with no name cannot be referred to, so it is no target.
		if name == "" {
			continue
		}
		if idx.byModuleAndName[module] == nil {
			idx.byModuleAndName[module] = map[string]string{}
		}
		// The first wins: a module declaring the same name twice is a thing
		// that does not happen, and picking either is better than dropping
		// both.
		if _, taken := idx.byModuleAndName[module][name]; !taken {
			idx.byModuleAndName[module][name] = u.ID
		}
		idx.byName[name] = append(idx.byName[name], u.ID)
		if !seen[module] {
			seen[module] = true
			idx.modules = append(idx.modules, module)
		}
	}
	// Longest first, so a suffix match prefers the most specific module.
	sort.SliceStable(idx.modules, func(i, j int) bool {
		return len(idx.modules[i]) > len(idx.modules[j])
	})
	idx.rank = make(map[string]int, len(idx.modules))
	idx.endingIn = map[string][]int{}
	for i, m := range idx.modules {
		idx.rank[m] = i
		for k := 0; k < len(m); k++ {
			if m[k] == '/' {
				idx.endingIn[m[k+1:]] = append(idx.endingIn[m[k+1:]], i)
			}
		}
	}
	return idx
}

// withoutAliasPrefix strips a project's own import alias, leaving the path
// the file tree can be searched for. Returns "" when the module carries no
// alias, so a bare package name is never mangled into a false match.
//
// `~/` and `@/` are the two conventions nearly every TypeScript project
// configures; `@scope/pkg` is a published package unless the rest of it
// matches a real directory, which the suffix search decides rather than this.
func withoutAliasPrefix(module string) string {
	switch {
	case strings.HasPrefix(module, "~/"), strings.HasPrefix(module, "@/"):
		return module[2:]
	case strings.HasPrefix(module, "~"), strings.HasPrefix(module, "@"):
		// `@acme/shared/thing` keeps everything after the scope, because the
		// scope is the publisher and the rest is the path.
		if i := strings.Index(module, "/"); i > 0 && i+1 < len(module) {
			return module[i+1:]
		}
	}
	return ""
}

// withoutSourceExtension drops the file extension an import path spells out,
// because a module is named for its file without one. `./ChatPanel.vue` is
// the only way to import a Vue component, and `./reader.js` is how ESM
// TypeScript must write `./reader`; kept, neither names any module. Only a
// path is touched: a dotted Python or Java name is not a file. An index file
// is named by its directory, so `./components/index.js` is `./components`.
func withoutSourceExtension(module string) string {
	if !strings.Contains(module, "/") {
		return module
	}
	switch path.Ext(module) {
	case ".vue", ".svelte", ".js", ".jsx", ".mjs", ".cjs", ".ts", ".tsx", ".mts", ".cts":
		module = strings.TrimSuffix(module, path.Ext(module))
		if path.Base(module) == "index" {
			module = path.Dir(module)
		}
	}
	return module
}

func splitID(id string) (module, name string) {
	if i := strings.LastIndex(id, "#"); i >= 0 {
		return id[:i], id[i+1:]
	}
	if i := strings.LastIndex(id, "."); i >= 0 {
		return id[:i], id[i+1:]
	}
	// PHP names its namespaces with backslashes: Acme\Order\Order.
	if i := strings.LastIndex(id, `\`); i >= 0 {
		return id[:i], id[i+1:]
	}
	return "", id
}

// Connections resolves every unit's references into edges.
//
// A reference naming something outside the codebase resolves to nothing and
// is dropped: `react` and `net/http` are real dependencies and not edges in
// this graph, and a resolver loose enough to invent one would fill the graph
// with components that do not exist.
func Connections(units []*Unit) []*Connection {
	idx := NewIndex(units)
	var out []*Connection
	seen := map[string]bool{}
	for _, u := range units {
		for _, ref := range u.Refs {
			target := idx.resolve(u, ref)
			if target == "" || target == u.ID {
				continue
			}
			key := u.ID + "\x00" + target
			if seen[key] {
				continue
			}
			seen[key] = true
			out = append(out, &Connection{From: u.ID, To: target, Via: ref.Module})
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].From != out[j].From {
			return out[i].From < out[j].From
		}
		return out[i].To < out[j].To
	})
	return out
}

func (idx *Index) resolve(from *Unit, ref Ref) string {
	if ref.Exact {
		return idx.byModuleAndName[ref.Module][ref.Name]
	}
	candidates := idx.candidateModules(from, ref.Module)
	for _, module := range candidates {
		if names := idx.byModuleAndName[module]; names != nil {
			if id, ok := names[ref.Name]; ok {
				return id
			}
		}
	}
	// Nothing in the module itself answers to the name, which is what a
	// barrel looks like from here: `import { Message } from "~/components"`
	// resolves to `src/components`, whose index re-exports a name declared
	// in `src/components/Chat/Message`. LibreChat ships 77 of these, so
	// stopping at the directory loses most of what it imports.
	//
	// Searched by name first and filtered by directory, so this costs a map
	// lookup rather than a scan of every module.
	return idx.underDirectory(candidates, ref.Name)
}

// The one unit with this name declared anywhere beneath one of these
// directories. Ambiguity is refused rather than guessed: two files exporting
// the same name give no answer, because inventing one would draw an edge to
// a unit the code never meant.
func (idx *Index) underDirectory(directories []string, name string) string {
	owners := idx.byName[name]
	if len(owners) == 0 {
		return ""
	}
	found := ""
	for _, dir := range directories {
		if dir == "" {
			continue
		}
		prefix := dir + "/"
		for _, id := range owners {
			if !strings.HasPrefix(idx.moduleOf[id], prefix) {
				continue
			}
			if found != "" && found != id {
				return ""
			}
			found = id
		}
		if found != "" {
			return found
		}
	}
	return found
}

// The modules a reference might mean, best first.
func (idx *Index) candidateModules(from *Unit, module string) []string {
	if module == "" {
		return nil
	}
	module = withoutSourceExtension(module)
	// Relative: resolved against the directory the reference was written in.
	if strings.HasPrefix(module, ".") {
		base := idx.moduleOf[from.ID]
		// A module is the file without its extension, so `./reader` written
		// in `src/utils/rules` means `src/utils/reader` -- the sibling, not
		// a child. An index file is already named by its directory, though,
		// and there `./Button` means `src/components/Button`, so both
		// readings are offered and the one that exists wins.
		return []string{
			path.Clean(path.Join(path.Dir(base), module)),
			path.Clean(path.Join(base, module)),
		}
	}

	candidates := []string{module}
	slashed := strings.ReplaceAll(module, ".", "/")
	if slashed != module {
		candidates = append(candidates, slashed)
	}

	// A project's own alias for its own source. `~/components/Chat` and
	// `@/utils` name a directory inside this repository, and the prefix is
	// configuration rather than part of the path -- so the tail is what the
	// file tree can be searched for.
	//
	// This is not a detail. LibreChat writes 3,485 of its 7,984 imports this
	// way, against 1,766 relative ones: read without it, 81% of its units
	// have no edge at all and the graph is empty for the language that needs
	// it most.
	if tail := withoutAliasPrefix(module); tail != "" {
		candidates = append(candidates, tail)
		slashed = tail
	}

	// The two ways an import and a directory fail to line up, and they point
	// in opposite directions.
	//
	// A Python import names a package from the root of the import path,
	// which is almost never the root of the repository: `acme.billing` lives
	// at `src/acme/billing`, so the known module ends with what the import
	// said.
	//
	// A Go import path carries a module prefix the file tree has never heard
	// of: `github.com/stretchr/testify/assert` lives at `assert`, so what
	// the import said ends with the known module. Longest first, so a module
	// whose name happens to be the tail of some unrelated path cannot win
	// over the real one.
	var found []int
	for _, said := range []string{module, slashed} {
		found = append(found, idx.endingIn[said]...)
		for k := 0; k < len(said); k++ {
			if said[k] != '/' {
				continue
			}
			if i, ok := idx.rank[said[k+1:]]; ok {
				found = append(found, i)
			}
		}
	}
	// In the order of idx.modules, longest first, each once.
	sort.Ints(found)
	for i, place := range found {
		known := idx.modules[place]
		if (i > 0 && place == found[i-1]) || known == module || known == slashed {
			continue
		}
		candidates = append(candidates, known)
	}
	return candidates
}
