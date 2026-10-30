// Package moduletest checks a pack's manifest readers against a checkout.
package moduletest

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/archstats/archstats/core/module"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Want is what a checkout's manifests must declare.
type Want struct {
	Kind  string
	Count int
	// name -> directory
	Modules map[string]string
	// name -> a dependency it must declare
	Deps map[string]string
}

// Check reads dir with the readers and holds the result to want.
func Check(t *testing.T, dir string, readers []module.Reader, want Want) *module.Map {
	t.Helper()
	m := module.Read(dir, readers...)
	assert.Equalf(t, want.Count, m.Len(), "modules found: %v", Names(m))
	assert.Equal(t, want.Count, m.Kinds()[want.Kind])
	for name, d := range want.Modules {
		mod := m.ByName(name)
		require.NotNilf(t, mod, "no module named %q, got %v", name, Names(m))
		assert.Equalf(t, d, mod.Dir, "wrong directory for %q", name)
		assert.Equal(t, want.Kind, mod.Kind)
	}
	for name, dep := range want.Deps {
		mod := m.ByName(name)
		require.NotNil(t, mod)
		assert.Containsf(t, mod.DependsOn, dep, "%q should depend on %q", name, dep)
	}
	return m
}

// Names lists the modules' names, longest directory first.
func Names(m *module.Map) []string {
	var out []string
	for _, mod := range m.Modules() {
		out = append(out, mod.Name)
	}
	return out
}

// Write puts a file under root, making its directories.
func Write(t *testing.T, root, rel, content string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(rel))
	require.NoError(t, os.MkdirAll(filepath.Dir(full), 0o755))
	require.NoError(t, os.WriteFile(full, []byte(content), 0o644))
}
