package ecmascript

import (
	"encoding/json"

	"github.com/archstats/archstats/core/module"
)

// ---------------------------------------------------------------------------
// node: JavaScript and TypeScript
//
// LibreChat declares `api`, `client` and `packages/*` as workspaces. Those
// are the boundaries an architect asks about, and nothing in any source file
// mentions them.
// ---------------------------------------------------------------------------

type nodeReader struct{}

func (nodeReader) Kind() string         { return "node" }
func (nodeReader) Claims(b string) bool { return b == "package.json" }

type nodeManifest struct {
	Name         string            `json:"name"`
	Dependencies map[string]string `json:"dependencies"`
	Dev          map[string]string `json:"devDependencies"`
}

func (nodeReader) Read(root, path string) []*module.Module {
	raw, ok := module.ReadFile(path)
	if !ok {
		return nil
	}
	var man nodeManifest
	if json.Unmarshal([]byte(raw), &man) != nil || man.Name == "" {
		return nil
	}
	return []*module.Module{{
		Name:      man.Name,
		Dir:       module.RelDir(root, path),
		Manifest:  module.RelFile(root, path),
		DependsOn: append(module.Keys(man.Dependencies), module.Keys(man.Dev)...),
	}}
}

// ManifestReaders reads package.json.
func ManifestReaders() []module.Reader { return []module.Reader{nodeReader{}} }
