package module

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The map knows no ecosystem, so it is tested with one of its own: a
// `module.json` naming the module and what it depends on. The packs test
// their real readers against real manifests.

type jsonReader struct{}

func (jsonReader) Kind() string         { return "json" }
func (jsonReader) Claims(b string) bool { return b == "module.json" }
func (jsonReader) Read(root, path string) []*Module {
	raw, ok := ReadFile(path)
	if !ok {
		return nil
	}
	var man struct {
		Name string   `json:"name"`
		Deps []string `json:"deps"`
	}
	if json.Unmarshal([]byte(raw), &man) != nil {
		return nil
	}
	return []*Module{{Name: man.Name, Dir: RelDir(root, path), Manifest: RelFile(root, path), DependsOn: man.Deps}}
}

// finishingReader is jsonReader with a pass after every manifest is read.
type finishingReader struct{ jsonReader }

func (finishingReader) Finish(m *Map, root string, paths []string) {
	for _, mod := range m.Modules() {
		mod.Type = "finished"
	}
}

func nested(t *testing.T) string {
	root := t.TempDir()
	writeFile(t, root, "module.json", `{"name":"root"}`)
	writeFile(t, root, "src/order/module.json", `{"name":"order"}`)
	writeFile(t, root, "src/bundle/module.json", `{"name":"bundle","deps":["order"]}`)
	return root
}

// A module inside another owns its own files.
func TestOf_LongestDirectoryWins(t *testing.T) {
	m := Read(nested(t), jsonReader{})
	assert.Equal(t, "order", m.NameOf("src/order/model/Order.x"))
	assert.Equal(t, "bundle", m.NameOf("src/bundle/di/Extension.x"))
	// Nothing deeper claims it, so the root module does.
	assert.Equal(t, "root", m.NameOf("src/kernel/Kernel.x"))
	assert.Equal(t, []string{"order"}, m.ByName("bundle").DependsOn)
	assert.Equal(t, "json", m.ByName("bundle").Kind)
}

func TestOf_NoModules(t *testing.T) {
	m := New()
	assert.Nil(t, m.Of("anything.go"))
	assert.Equal(t, "", m.NameOf("anything.go"))
	assert.Equal(t, 0, m.Len())
}

// With no reader for its manifests, a checkout declares no modules: a scan
// reads only with the readers its packs registered.
func TestRead_NoReadersNoModules(t *testing.T) {
	assert.Equal(t, 0, Read(nested(t)).Len())
}

// ReadFrom must consider only files the analysis saw. archstats's own
// repository ignores `**/temp_testdata/**`, which holds full checkouts of
// MediatR, kotlinx-datetime and elepy; read without that filter, archstats
// reports itself as 84 modules instead of 3.
func TestReadFrom_OnlyGivenFiles(t *testing.T) {
	root := nested(t)
	require.Equal(t, 3, Read(root, jsonReader{}).Len())

	visible := ReadFrom(root, []string{"module.json"}, []Reader{jsonReader{}})
	assert.Equal(t, 1, visible.Len())
	assert.NotNil(t, visible.ByName("root"))
	assert.Nil(t, visible.ByName("order"))
}

// The walker hands back files in whatever order it found them, so the map
// must not depend on it.
func TestReadFrom_OrderIndependent(t *testing.T) {
	root := nested(t)
	readers := []Reader{jsonReader{}}
	forwards := ReadFrom(root, []string{"module.json", "src/order/module.json", "src/bundle/module.json"}, readers)
	backwards := ReadFrom(root, []string{"src/bundle/module.json", "src/order/module.json", "module.json"}, readers)
	assert.Equal(t, forwards.NameOf("src/order/model/Order.x"), backwards.NameOf("src/order/model/Order.x"))
	assert.Equal(t, forwards.Len(), backwards.Len())
}

// A reader with a Finish pass gets it once every manifest is read.
func TestReadFrom_Finisher(t *testing.T) {
	m := Read(nested(t), finishingReader{})
	for _, mod := range m.Modules() {
		assert.Equal(t, "finished", mod.Type, mod.Name)
	}
}

// A manifest that names nothing, or cannot be parsed, declares nothing and
// does not take the rest with it.
func TestReaders_AnonymousAndMalformedAreSkipped(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "anon/module.json", `{"deps":["x"]}`)
	writeFile(t, root, "bad/module.json", `{ not json at all`)
	writeFile(t, root, "good/module.json", `{"name":"good"}`)
	m := Read(root, jsonReader{})
	assert.Equal(t, 1, m.Len())
	assert.NotNil(t, m.ByName("good"))
}

// A file sitting beside a manifest belongs to it, and a file above every
// manifest belongs to none.
func TestOf_Boundaries(t *testing.T) {
	m := New(
		&Module{Name: "root", Dir: "", Kind: "node"},
		&Module{Name: "shared", Dir: "packages/shared", Kind: "node"},
	)
	assert.Equal(t, "shared", m.NameOf("packages/shared/package.json"))
	assert.Equal(t, "shared", m.NameOf("packages/shared/src/deep/a.ts"))
	// Prefix matching must respect directory boundaries: `packages/shared2`
	// is not inside `packages/shared`.
	assert.Equal(t, "root", m.NameOf("packages/shared2/a.ts"))
	assert.Equal(t, "root", m.NameOf("a.ts"))
}

func TestOf_NoRootModule(t *testing.T) {
	m := New(&Module{Name: "shared", Dir: "packages/shared", Kind: "node"})
	assert.Nil(t, m.Of("a.ts"), "nothing claims a file above every manifest")
}

// Two manifests may name the same thing -- Sylius has several test
// applications all called example/test-application. Both are kept as
// modules; lookup by name answers with one of them rather than failing.
func TestDuplicateNames(t *testing.T) {
	m := New(
		&Module{Name: "example/app", Dir: "a", Kind: "composer"},
		&Module{Name: "example/app", Dir: "b", Kind: "composer"},
	)
	assert.Equal(t, 2, m.Len())
	assert.NotNil(t, m.ByName("example/app"))
	assert.Equal(t, "example/app", m.NameOf("a/x.php"))
	assert.Equal(t, "example/app", m.NameOf("b/x.php"))
}

func TestKinds(t *testing.T) {
	m := New(
		&Module{Name: "a", Dir: "a", Kind: "node"},
		&Module{Name: "b", Dir: "b", Kind: "node"},
		&Module{Name: "c", Dir: "c", Kind: "go"},
	)
	assert.Equal(t, map[string]int{"node": 2, "go": 1}, m.Kinds())
}

func writeFile(t *testing.T, root, rel, content string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(rel))
	require.NoError(t, os.MkdirAll(filepath.Dir(full), 0o755))
	require.NoError(t, os.WriteFile(full, []byte(content), 0o644))
}
