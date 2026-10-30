package python

import (
	"path/filepath"
	"strings"

	"github.com/archstats/archstats/core/component"
	"github.com/archstats/archstats/core/file"
)

// ImportResolver reads Python's relative imports, where each leading dot is
// a package level: `.db` is a sibling, `..utils.crypto` is beside the
// parent.
func ImportResolver() component.ImportResolver { return pyImports{} }

type pyImports struct{}

func (pyImports) Name() string                   { return "python" }
func (pyImports) Prepare(string, []*file.Results) {}

func (pyImports) Resolve(fromFile, fromDir, imp string, dirs component.Dirs) (string, bool) {
	if !strings.HasSuffix(fromFile, ".py") || !strings.HasPrefix(imp, ".") || strings.ContainsAny(imp, `/\`) {
		return "", false
	}
	dots := 0
	for dots < len(imp) && imp[dots] == '.' {
		dots++
	}
	// One dot is the importing package, each more one level up.
	baseDir := fromDir
	for i := 0; i < dots-1; i++ {
		baseDir = filepath.Dir(baseDir)
	}
	resolved := filepath.ToSlash(filepath.Join(baseDir, strings.ReplaceAll(imp[dots:], ".", "/")))
	if dirs.Has(resolved) {
		return resolved, true
	}
	// A module inside a known package: its component is the package.
	if parent := filepath.ToSlash(filepath.Dir(resolved)); dirs.Has(parent) {
		return parent, true
	}
	return resolved, true
}
