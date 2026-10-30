package golang

import (
	"regexp"

	"github.com/archstats/archstats/core/module"
)

// ---------------------------------------------------------------------------
// go
//
// One module per go.mod. Go's components are directories -- `package core`
// says nothing about where it sits -- so the value here is the module path,
// which every absolute import carries as a prefix the file tree has never
// heard of, and the `internal/` boundaries the compiler enforces.
// ---------------------------------------------------------------------------

type goReader struct{}

func (goReader) Kind() string         { return "go" }
func (goReader) Claims(b string) bool { return b == "go.mod" }

var (
	goModulePath = regexp.MustCompile(`(?m)^\s*module\s+(\S+)`)
	goRequire    = regexp.MustCompile(`(?m)^\s*(?:require\s+)?([a-z0-9._~-]+\.[a-z]{2,}/\S+)\s+v\S+`)
)

func (goReader) Read(root, path string) []*module.Module {
	raw, ok := module.ReadFile(path)
	if !ok {
		return nil
	}
	name := goModulePath.FindStringSubmatch(raw)
	if name == nil {
		return nil
	}
	var deps []string
	for _, m := range goRequire.FindAllStringSubmatch(raw, -1) {
		deps = module.AppendUnique(deps, m[1])
	}
	return []*module.Module{{
		Name:      name[1],
		Dir:       module.RelDir(root, path),
		Manifest:  module.RelFile(root, path),
		DependsOn: deps,
	}}
}

// ManifestReaders reads go.mod.
func ManifestReaders() []module.Reader { return []module.Reader{goReader{}} }
