package declbased

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadsWorkspacePackagesAndPathAliases(t *testing.T) {
	root := t.TempDir()
	write(t, root, "package.json", `{"name":"@acme/root","workspaces":["packages/*"]}`)
	write(t, root, "packages/shared/package.json", `{"name":"@acme/shared","main":"src/index.ts"}`)
	write(t, root, "packages/shared/src/index.ts", "export const x = 1;")
	write(t, root, "apps/web/src/app/orders/service.ts", "export const y = 1;")
	// Comments and a trailing comma: legal in tsconfig, rejected by a plain
	// JSON parser, and present in most real ones.
	write(t, root, "tsconfig.json", `{
		// paths, as every monorepo writes them
		"compilerOptions": {
			"baseUrl": ".",
			"paths": {
				"@app/*": ["apps/web/src/app/*"],
				"*": ["node_modules/*"],
			}
		}
	}`)
	// Must be ignored: a dependency's own manifest names packages that are
	// not this codebase's.
	write(t, root, "node_modules/@acme/shared/package.json", `{"name":"@acme/shared","main":"index.js"}`)

	fileDirs := map[string]string{
		"packages/shared/src/index.ts":       "packages/shared/src",
		"apps/web/src/app/orders/service.ts": "apps/web/src/app/orders",
	}
	aliases := readAliases(root)

	for importValue, want := range map[string]string{
		"@acme/shared":        "packages/shared/src", // main points into src
		"@app/orders/service": "apps/web/src/app/orders",
		"@app/orders":         "apps/web/src/app/orders",
		"react":               "", // third party: no edge, not a guess
		"lodash/fp":           "",
		"@acme/nonexistent":   "",
	} {
		assert.Equalf(t, want, aliases.resolve(importValue, fileDirs), "resolving %q", importValue)
	}
}

func TestIgnoresDependencyManifests(t *testing.T) {
	root := t.TempDir()
	write(t, root, "node_modules/left-pad/package.json", `{"name":"left-pad"}`)
	write(t, root, "dist/package.json", `{"name":"@acme/built"}`)

	// Nothing in a build output or a dependency tree names a component of
	// this codebase, and walking them on a real repo is most of the run.
	assert.Empty(t, readAliases(root).entries)
}

func write(t *testing.T, root, name, content string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(name))
	require.NoError(t, os.MkdirAll(filepath.Dir(full), 0o755))
	require.NoError(t, os.WriteFile(full, []byte(content), 0o644))
}
