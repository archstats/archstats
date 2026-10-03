package typescript

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/archstats/archstats/core"
	"github.com/archstats/archstats/core/walker"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// archstats-ui's cycles.ts joins keys with a literal NUL. With the
// TypeScript pack loaded it is source: it reaches the snapshot and its
// imports are read, including the one after the NUL.
func TestTypeScriptWithNULIsAnalysed(t *testing.T) {
	dir := t.TempDir()
	src := "import { graph } from \"./graph\";\n" +
		"const SEP = \"\x00\";\n" +
		"import { walk } from \"./walk\";\n" +
		"export function key(a: string, b: string) { return a + SEP + b; }\n"
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "utils"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "utils", "cycles.ts"), []byte(src), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "logo.png"), []byte("\x89PNG\r\n\x1a\n\x00\x00"), 0o644))

	results, err := core.New(&core.Config{RootPath: dir, Extensions: []core.Extension{&Extension{}}}).Analyze()
	require.NoError(t, err)

	var imports []string
	for _, s := range results.Snippets {
		if strings.HasSuffix(s.File, "utils/cycles.ts") && s.Type == "modularity__component__imports" {
			imports = append(imports, s.Value)
		}
	}
	assert.ElementsMatch(t, []string{"./graph", "./walk"}, imports)
	assert.Equal(t, []walker.Skipped{{Path: "logo.png", Reason: walker.SkipBinary, Detail: "NUL byte at offset 8"}}, results.SkippedFiles)
}
