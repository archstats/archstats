package e2eTest

import (
	"bytes"
	"testing"

	"github.com/archstats/archstats/cmd"
	"github.com/jszwec/csvutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The shapes an enterprise codebase is actually written in, with every edge
// known in advance.
//
// The real-project suite answers "can we read this language at all". It
// cannot answer "is the graph right", because nobody knows what express's
// dependency graph should be. These two fixtures are small enough to hold in
// your head and are checked edge by edge — and both were built by taking the
// idioms a monorepo uses and finding out, one at a time, that they produced
// nothing:
//
//   - `@acme/shared`, a workspace package imported by the name its own
//     package.json gives it: no edge.
//   - `@app/billing/invoice` and `~/components/Chat`, a tsconfig path alias:
//     resolved by guessing at directory suffixes, which happens to work in a
//     single-package repo. LibreChat writes 2,050 of its client imports this
//     way.
//   - `acme.billing.invoice`, an absolute Python import under the `src/`
//     layout every modern Python package uses: no edge, because the resolver
//     compared it to directories by equality and the import root is never the
//     repository root.
//   - `import acme.shared.log as slog`: never captured at all, because the
//     alias wraps the name the query was looking for.
//
// Third-party imports appear in both fixtures on purpose. `react`, `lodash`
// and `os` must resolve to nothing: an import that names something outside
// the codebase is not an edge, and a resolver loose enough to invent one
// would fill an enterprise graph with phantom components.

func Test_Enterprise_TypeScriptMonorepo(t *testing.T) {
	assertConnections(t, "enterprise/ts", []ComponentConnectionDirect{
		// A workspace package, imported by name — once as an ESM import and
		// once through require(), which is one edge of weight two.
		directConnection("apps/web/src/app/orders", "packages/shared/src", "apps/web/src/app/orders/service.ts", 2),
		// A scoped subpath import, landing in the package's source directory
		// rather than its root.
		directConnection("apps/web/src/app/orders", "packages/ui/src", "apps/web/src/app/orders/service.ts", 1),
		// The same target reached two ways: a relative path and a tsconfig
		// alias. `import type` counts — a type-only dependency is still one.
		directConnection("apps/web/src/app/orders", "apps/web/src/app/billing", "apps/web/src/app/orders/service.ts", 2),
		// Package to package.
		directConnection("packages/ui/src", "packages/shared/src", "packages/ui/src/Button.ts", 1),
	})
}

func Test_Enterprise_PythonSrcLayout(t *testing.T) {
	assertConnections(t, "enterprise/py", []ComponentConnectionDirect{
		// An absolute import and an aliased one, both naming the same package.
		directConnection("src/acme/orders", "src/acme/shared", "src/acme/orders/service.py", 2),
		// `from acme.billing.invoice import Invoice` and
		// `from acme.billing import invoice as inv`.
		directConnection("src/acme/orders", "src/acme/billing", "src/acme/orders/service.py", 2),
		// `from . import models` is the component importing itself, which is
		// not a dependency and must not appear.
	})
}

func assertConnections(t *testing.T, dir string, expected []ComponentConnectionDirect) {
	t.Helper()

	out := bytes.NewBufferString("")
	err := cmd.Execute(out, bytes.NewBufferString(""), nil,
		[]string{"-o", "csv", "-f", dir, "view", "component_connections_direct"})
	require.NoError(t, err)

	var actual []ComponentConnectionDirect
	require.NoError(t, csvutil.Unmarshal(out.Bytes(), &actual))

	// Exactly these, and nothing else: a missing edge is a dependency the
	// architect cannot see, and an extra one is a dependency that is not there.
	assert.ElementsMatch(t, expected, actual)
}
