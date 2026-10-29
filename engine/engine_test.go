package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/archstats/archstats/core/plugin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func goWorkspace(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	write := func(path, content string) {
		full := filepath.Join(root, path)
		require.NoError(t, os.MkdirAll(filepath.Dir(full), 0o755))
		require.NoError(t, os.WriteFile(full, []byte(content), 0o644))
	}
	write("go.mod", "module example.com/shop\n\ngo 1.23\n")
	write("orders/orders.go", "package orders\n\nimport \"example.com/shop/billing\"\n\nfunc Place() { billing.Charge() }\n")
	write("billing/billing.go", "package billing\n\nfunc Charge() {}\n")
	return root
}

func TestAnalyze_WithNothingButARoot(t *testing.T) {
	results, err := Analyze(Options{Root: goWorkspace(t)})
	require.NoError(t, err)

	assert.Contains(t, strings.Split(results.SnapshotInfo["extensions"], ","), "go")
	var edges []string
	for _, c := range results.Connections {
		edges = append(edges, c.From+" -> "+c.To)
	}
	assert.Contains(t, edges, "orders -> billing")
}

func TestAnalyze_ReadsSettingsFromAMap(t *testing.T) {
	root := goWorkspace(t)
	enabled, _, err := Select(root, nil, nil)
	require.NoError(t, err)

	_, err = Analyze(Options{Root: root, Settings: plugin.Values(map[string]any{"component-strategy": "nonsense"}, enabled)})
	assert.ErrorContains(t, err, "component-strategy")

	results, err := Analyze(Options{Root: root, Settings: plugin.Values(map[string]any{"component-strategy": "directory"}, enabled)})
	require.NoError(t, err)
	assert.NotEmpty(t, results.Connections)
}

func TestSelect_NamedExtensionsReplaceDiscovery(t *testing.T) {
	root := goWorkspace(t)

	_, discovered, err := Select(root, nil, nil)
	require.NoError(t, err)
	assert.Contains(t, discovered, "go")

	enabled, discovered, err := Select(root, nil, []string{"python"})
	require.NoError(t, err)
	assert.Empty(t, discovered)
	var names []string
	for _, d := range enabled {
		names = append(names, d.Name)
	}
	assert.Contains(t, names, "python")
	assert.NotContains(t, names, "go")

	_, _, err = Select(root, nil, []string{"cobol"})
	assert.ErrorContains(t, err, "cobol")
}
