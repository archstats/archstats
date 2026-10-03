package declbased

import (
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// What a Go import means, read from go.mod rather than guessed.
//
// A Go import is the module path followed by the package's directory under
// that module: `example.com/app/core/file` lives at `core/file` beside the
// go.mod that says `module example.com/app`. Matching the tail of the import
// against directory names instead -- which is what Go resolution did from
// 2026-09-21 -- also matched every third-party package whose last segment
// happened to name a local folder. On archstats-ui, a Wails app,
// `github.com/wailsapp/wails/v2/pkg/runtime` resolved to
// `frontend/wailsjs/runtime`, a directory of generated TypeScript, and a lens
// reported 13 backend-to-bindings crossings that do not exist.
//
// An import outside every module path the tree declares is a library, and
// stays unresolved.

type goModule struct {
	// The module path, as the `module` line of go.mod gives it.
	path string
	// The directory holding that go.mod, as file.Results.Directory spells
	// it: "." for the root of the scan.
	dir string
}

type goModules struct {
	// Longest path first, so a module nested inside another
	// (`example.com/app/tools` under `example.com/app`) claims its own imports.
	mods []goModule
	// Directories holding at least one .go file. A Go import names a package,
	// and a package is Go source; a directory of anything else is not one.
	pkgDirs map[string]bool
}

var goModuleLine = regexp.MustCompile(`(?m)^\s*module\s+(\S+)`)

// readGoModulesFrom reads the go.mod files among the files the analysis
// actually saw, for the same reason readAliasesFrom does: a second walk would
// not honour .gitignore and .archstatsignore (ADR 0012).
//
// A scan rooted below its module -- one service of a monorepo -- has no go.mod
// of its own, and its imports still carry the module path. The go.mod above
// the root is read too, with the path extended to where the scan starts.
func readGoModulesFrom(root string, names []string) []goModule {
	var mods []goModule
	if root == "" {
		return mods
	}
	rootHasModule := false
	for _, name := range names {
		if path.Base(name) != "go.mod" || ignoredByGoTool(name) {
			continue
		}
		modPath := readGoModulePath(filepath.Join(root, filepath.FromSlash(name)))
		if modPath == "" {
			continue
		}
		dir := path.Dir(name)
		if dir == "." {
			rootHasModule = true
		}
		mods = append(mods, goModule{path: modPath, dir: dir})
	}
	if !rootHasModule {
		if m, ok := enclosingGoModule(root); ok {
			mods = append(mods, m)
		}
	}
	sort.Slice(mods, func(i, j int) bool {
		a, b := mods[i], mods[j]
		if len(a.path) != len(b.path) {
			return len(a.path) > len(b.path)
		}
		// The same path declared twice: the shallower copy first, then by
		// name, so the answer does not depend on the order files were found.
		if da, db := strings.Count(a.dir, "/"), strings.Count(b.dir, "/"); da != db {
			return da < db
		}
		return a.dir < b.dir
	})
	return mods
}

// ignoredByGoTool reports whether the go tool would never see this file:
// it skips directories named testdata and those beginning with . or _.
// archstats keeps a fixture at core/module/testdata/gomod/go.mod declaring
// its own module path, and read as a module it swallowed every import of the
// real one, and the Go graph came out empty.
func ignoredByGoTool(name string) bool {
	for _, seg := range strings.Split(path.Dir(name), "/") {
		if seg == "testdata" || (seg != "." && seg != ".." && (strings.HasPrefix(seg, ".") || strings.HasPrefix(seg, "_"))) {
			return true
		}
	}
	return false
}

// enclosingGoModule finds the go.mod above root, as the go tool would.
func enclosingGoModule(root string) (goModule, bool) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return goModule{}, false
	}
	for dir := filepath.Dir(abs); ; dir = filepath.Dir(dir) {
		if modPath := readGoModulePath(filepath.Join(dir, "go.mod")); modPath != "" {
			rel, err := filepath.Rel(dir, abs)
			if err != nil {
				return goModule{}, false
			}
			return goModule{path: modPath + "/" + filepath.ToSlash(rel), dir: "."}, true
		}
		if parent := filepath.Dir(dir); parent == dir {
			return goModule{}, false
		}
	}
}

func readGoModulePath(file string) string {
	raw, err := os.ReadFile(file)
	if err != nil {
		return ""
	}
	m := goModuleLine.FindStringSubmatch(string(raw))
	if m == nil {
		return ""
	}
	// `module "example.com/app"` is legal, if rare.
	return strings.Trim(m[1], "\"`")
}

// resolve maps an import onto the package directory it names. It reports
// false for an import no module in the tree declares -- the standard library,
// a dependency -- and for one that names no directory of Go source.
func (g *goModules) resolve(importValue string) (string, bool) {
	claimed := -1
	for _, mod := range g.mods {
		// The longest module path that claims the import is the one it
		// belongs to; a shorter one cannot hold it too. Only copies of that
		// same path are worth trying after it.
		if claimed >= 0 && len(mod.path) != claimed {
			break
		}
		var rest string
		switch {
		case importValue == mod.path:
		case strings.HasPrefix(importValue, mod.path+"/"):
			rest = importValue[len(mod.path)+1:]
		default:
			continue
		}
		claimed = len(mod.path)
		if dir := path.Join(mod.dir, rest); g.pkgDirs[dir] {
			return dir, true
		}
	}
	return "", false
}

func isGoFile(name string) bool {
	return strings.HasSuffix(name, ".go")
}
