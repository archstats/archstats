// Package module reads the modules a project declares for itself.
//
// A component is where the code says it lives -- a package header, a
// namespace, a directory. A module is something else: the unit the project
// builds, publishes and depends on, and it is written down in a manifest
// rather than in any source file.
//
// The distinction is not academic. nopCommerce is 3,650 C# files whose
// namespaces say `Nop.Core`, `Nop.Services`, `Nop.Plugin.Payments.PayPal` --
// but what makes it a plugin architecture is the 40 `.csproj` files, and the
// rule that matters ("core must never depend on a plugin") is only checkable
// against those. Sylius is the same story: its `Component` and `Bundle` split
// is 60 composer packages, and the whole design rests on Component not
// knowing about Bundle.
//
// Core owns the model -- what a module is and which file it owns -- and the
// order manifests are read in. It knows no ecosystem: each language pack
// registers the readers for its own manifests (ADR 0025), and a scan reads
// only with the readers of the packs it runs.
package module

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// A Module is a unit of distribution the project declares for itself.
type Module struct {
	// What the project calls it: "Nop.Core", "sylius/order", "@librechat/api".
	Name string
	// Repo-relative directory the module owns, "" for a module at the root.
	Dir string
	// Repo-relative path of the manifest that declared it.
	Manifest string
	// Which reader found it, by the reader's Kind.
	Kind string
	// Modules this one declares a dependency on, by the name the manifest
	// used. Not every ecosystem records these, and a name here need not
	// belong to this project.
	DependsOn []string
	// What the module builds, where the manifest says: android-application,
	// android-library, kotlin-multiplatform, build-logic. Empty when the
	// manifest does not say, which for most ecosystems is always.
	Type string
	// The files the manifest lists one by one, where it does: an Xcode
	// target's Sources build phase. They belong to this module whatever
	// directory they are in, and a file two targets list belongs to the
	// first.
	Files []string
	// Further directories the module owns beside Dir: an Xcode target
	// synchronised with several folders. IceCubesApp's app target owns
	// IceCubesApp/, IceCubesAppIntents/ and AlternateIcons/, whose common
	// parent is the whole repository.
	Dirs []string
}

// Map answers which module owns a file, and what the project declared.
type Map struct {
	// Longest Dir first, so a module nested inside another claims its own
	// files before the outer one does. A Sylius bundle lives beneath the root
	// composer.json; the bundle is the right answer for its files.
	modules []*Module
	byName  map[string]*Module
	byFile  map[string]*Module
	// Every directory a module owns, Dir and Dirs alike, longest first.
	owned []ownedDir
}

type ownedDir struct {
	dir string
	mod *Module
}

// A Reader turns one manifest into the modules it declares.
//
// Readers are registered rather than switched on, for the same reason
// component resolution is pluggable (ADR 0007): the consumers of a module map
// must not learn any ecosystem's name.
type Reader interface {
	// Kind names the reader in Module.Kind and in diagnostics.
	Kind() string
	// Claims reports whether this reader handles a file with this base name.
	Claims(base string) bool
	// Read parses one manifest. root and path are absolute; the returned
	// Module.Dir and Module.Manifest are repo-relative. Returning nil is
	// normal and means the file declared nothing.
	Read(root, path string) []*Module
}

// A Finisher is a Reader with work to do once every manifest is read: a
// dependency named the way the build tool names it, resolved against the
// modules the other manifests declared. It may change a module's DependsOn
// and Type, never its Name or Dir.
type Finisher interface {
	Finish(m *Map, root string, paths []string)
}

// skipDir keeps Read, the walk for tests, out of version control.
var skipDir = map[string]bool{".git": true}

// ReadFrom builds the map from files the analysis has already discovered.
//
// This is the entry point the engine uses, and it takes the file list rather
// than walking for itself for one reason: the walker honours .gitignore and
// .archstatsignore (ADR 0012) and a second walk would not. archstats's own
// repository ignores `**/temp_testdata/**`, which holds the checkouts of
// MediatR, kotlinx-datetime and elepy that the e2e tests analyse. Walking
// raw, archstats reported itself as 84 modules -- 48 of them Broadleaf's and
// MediatR's -- instead of the 3 go.mod files it actually has.
//
// Paths are repo-relative, as the walker and file.Results.Name give them.
//
// Readers are tried in the order given. A file claimed by two readers yields
// two modules; the longest directory still wins, and a tie goes to the
// reader given first.
func ReadFrom(root string, paths []string, readers []Reader) *Map {
	m := &Map{byName: map[string]*Module{}}
	if root == "" {
		return m
	}
	for _, rel := range paths {
		base := filepath.Base(rel)
		for _, r := range readers {
			if !r.Claims(base) {
				continue
			}
			for _, mod := range r.Read(root, filepath.Join(root, filepath.FromSlash(rel))) {
				if mod == nil || mod.Name == "" {
					continue
				}
				mod.Kind = r.Kind()
				m.modules = append(m.modules, mod)
			}
		}
	}
	m.index()
	for _, r := range readers {
		if f, ok := r.(Finisher); ok {
			f.Finish(m, root, paths)
		}
	}
	return m
}

// Read walks a checkout itself. For tests and for callers with no file list
// in hand; the engine uses ReadFrom. It honours no ignore file, so nothing
// that matters relies on it.
func Read(root string, readers ...Reader) *Map {
	if root == "" {
		return &Map{byName: map[string]*Module{}}
	}
	var paths []string
	_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if skipDir[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if rel, relErr := filepath.Rel(root, p); relErr == nil {
			paths = append(paths, filepath.ToSlash(rel))
		}
		return nil
	})
	return ReadFrom(root, paths, readers)
}

// New builds a Map from modules already in hand. For tests, and for callers
// that read manifests themselves.
func New(modules ...*Module) *Map {
	m := &Map{modules: modules, byName: map[string]*Module{}}
	m.index()
	return m
}

func (m *Map) index() {
	sort.SliceStable(m.modules, func(i, j int) bool {
		return len(m.modules[i].Dir) > len(m.modules[j].Dir)
	})
	m.byFile = map[string]*Module{}
	m.owned = nil
	for _, mod := range m.modules {
		m.owned = append(m.owned, ownedDir{mod.Dir, mod})
		for _, d := range mod.Dirs {
			m.owned = append(m.owned, ownedDir{d, mod})
		}
	}
	sort.SliceStable(m.owned, func(i, j int) bool { return len(m.owned[i].dir) > len(m.owned[j].dir) })
	for _, mod := range m.modules {
		if _, taken := m.byName[mod.Name]; !taken {
			m.byName[mod.Name] = mod
		}
		for _, f := range mod.Files {
			if _, taken := m.byFile[f]; !taken {
				m.byFile[f] = mod
			}
		}
	}
}

// Of returns the module owning a repo-relative file path, or nil.
func (m *Map) Of(filePath string) *Module {
	filePath = filepath.ToSlash(filePath)
	if mod := m.byFile[strings.TrimPrefix(filePath, "./")]; mod != nil {
		return mod
	}
	for _, o := range m.owned {
		if o.dir == "" {
			return o.mod
		}
		if filePath == o.dir || strings.HasPrefix(filePath, o.dir+"/") {
			return o.mod
		}
	}
	return nil
}

// NameOf is Of, returning "" rather than nil, for callers filling a column.
func (m *Map) NameOf(filePath string) string {
	if mod := m.Of(filePath); mod != nil {
		return mod.Name
	}
	return ""
}

// ByName returns the module the project calls this, or nil.
func (m *Map) ByName(name string) *Module { return m.byName[name] }

// Modules returns every declared module, longest directory first.
func (m *Map) Modules() []*Module { return m.modules }

// Len is the number of declared modules.
func (m *Map) Len() int { return len(m.modules) }

// Kinds counts the modules each reader found, for diagnostics and for the
// e2e tests, which assert that a project's manifests were read at all.
func (m *Map) Kinds() map[string]int {
	out := map[string]int{}
	for _, mod := range m.modules {
		out[mod.Kind]++
	}
	return out
}

// RelDir gives a manifest's directory relative to the root, "" at the root.
func RelDir(root, manifestPath string) string {
	rel, err := filepath.Rel(root, filepath.Dir(manifestPath))
	if err != nil {
		return ""
	}
	rel = filepath.ToSlash(rel)
	if rel == "." {
		return ""
	}
	return rel
}

// RelFile gives a manifest's own path relative to the root.
func RelFile(root, manifestPath string) string {
	rel, err := filepath.Rel(root, manifestPath)
	if err != nil {
		return filepath.ToSlash(manifestPath)
	}
	return filepath.ToSlash(rel)
}

// ReadFile reads a manifest, reporting whether it could.
func ReadFile(path string) (string, bool) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	return string(raw), true
}

// Keys lists a manifest's dependency map, sorted, so a module's DependsOn is
// the same on every scan.
func Keys(m map[string]string) []string {
	if len(m) == 0 {
		return nil
	}
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// AppendUnique appends v unless the list already holds it.
func AppendUnique(list []string, v string) []string {
	for _, existing := range list {
		if existing == v {
			return list
		}
	}
	return append(list, v)
}
