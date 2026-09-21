package module

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Each case is written in the idiom that actually breaks a naive reader: a
// .csproj whose project reference is a Windows path to a sibling, a composer
// package nested under another, a gradle module that never names itself, a
// Django app whose name is the dotted path Python would import rather than
// the directory it sits in.

func TestReaders(t *testing.T) {
	cases := []struct {
		dir       string
		wantKind  string
		wantCount int
		// name -> directory
		wantModules map[string]string
		// name -> dependency that must be present
		wantDep map[string]string
	}{
		{
			dir: "dotnet", wantKind: "dotnet", wantCount: 2,
			wantModules: map[string]string{
				"Nop.Core":                     "src/Core",
				"Nop.Plugin.Payments.PayPal":   "src/Plugins/Nop.Plugin.Payments.PayPal",
			},
			// `..\..\Core\Nop.Core.csproj` names a project, not a path.
			wantDep: map[string]string{"Nop.Plugin.Payments.PayPal": "Nop.Core"},
		},
		{
			dir: "composer", wantKind: "composer", wantCount: 3,
			wantModules: map[string]string{
				"sylius/sylius":       "",
				"sylius/order":        "src/Component/Order",
				"sylius/order-bundle": "src/Bundle/OrderBundle",
			},
			wantDep: map[string]string{"sylius/order-bundle": "sylius/order"},
		},
		{
			dir: "node", wantKind: "node", wantCount: 2,
			wantModules: map[string]string{
				"librechat":          "",
				"@librechat/shared":  "packages/shared",
			},
		},
		{
			// A gradle build script does not name itself; the directory does.
			dir: "gradle", wantKind: "gradle", wantCount: 1,
			wantModules: map[string]string{"core": "core"},
			wantDep:     map[string]string{"core": "exposed-common"},
		},
		{
			dir: "maven", wantKind: "maven", wantCount: 2,
			wantModules: map[string]string{
				"broadleaf":               "",
				"broadleaf-framework-web": "web",
			},
			wantDep: map[string]string{"broadleaf-framework-web": "broadleaf-framework"},
		},
		{
			dir: "gomod", wantKind: "go", wantCount: 1,
			wantModules: map[string]string{"github.com/archstats/archstats": ""},
			wantDep:     map[string]string{"github.com/archstats/archstats": "github.com/samber/lo"},
		},
		{
			// The app is named the way Python imports it -- `shop.orders` --
			// not `orders`, because `src` holds no __init__.py and `shop` does.
			dir: "django", wantKind: "django", wantCount: 1,
			wantModules: map[string]string{"shop.orders": "src/shop/orders"},
		},
	}

	for _, c := range cases {
		t.Run(c.dir, func(t *testing.T) {
			m := Read("testdata/" + c.dir)
			assert.Equalf(t, c.wantCount, m.Len(), "modules found: %v", names(m))
			assert.Equal(t, c.wantCount, m.Kinds()[c.wantKind])

			for name, dir := range c.wantModules {
				mod := m.ByName(name)
				require.NotNilf(t, mod, "no module named %q, got %v", name, names(m))
				assert.Equalf(t, dir, mod.Dir, "wrong directory for %q", name)
				assert.Equal(t, c.wantKind, mod.Kind)
			}
			for name, dep := range c.wantDep {
				mod := m.ByName(name)
				require.NotNil(t, mod)
				assert.Containsf(t, mod.DependsOn, dep, "%q should depend on %q", name, dep)
			}
		})
	}
}

// The nested case: a package inside another package owns its own files.
// Sylius has 60 composer packages beneath one root composer.json, and every
// file under src/Component/Order belongs to sylius/order, not sylius/sylius.
func TestOf_LongestDirectoryWins(t *testing.T) {
	m := Read("testdata/composer")

	assert.Equal(t, "sylius/order", m.NameOf("src/Component/Order/Model/Order.php"))
	assert.Equal(t, "sylius/order-bundle", m.NameOf("src/Bundle/OrderBundle/DependencyInjection/Extension.php"))
	// Nothing deeper claims it, so the root package does.
	assert.Equal(t, "sylius/sylius", m.NameOf("src/Sylius/Kernel.php"))
}

func TestOf_NoModules(t *testing.T) {
	m := New()
	assert.Nil(t, m.Of("anything.go"))
	assert.Equal(t, "", m.NameOf("anything.go"))
	assert.Equal(t, 0, m.Len())
}

// ReadFrom must consider only files the analysis saw. archstats's own
// repository ignores `**/temp_testdata/**`, which holds full checkouts of
// MediatR, kotlinx-datetime and elepy; read without that filter, archstats
// reports itself as 84 modules instead of 3.
func TestReadFrom_OnlyGivenFiles(t *testing.T) {
	all := Read("testdata/composer")
	require.Equal(t, 3, all.Len())

	visible := ReadFrom("testdata/composer", []string{"composer.json"})
	assert.Equal(t, 1, visible.Len())
	assert.NotNil(t, visible.ByName("sylius/sylius"))
	assert.Nil(t, visible.ByName("sylius/order"))
}

func names(m *Map) []string {
	var out []string
	for _, mod := range m.Modules() {
		out = append(out, mod.Name)
	}
	return out
}

// The walker hands back files in whatever order it found them, so the map
// must not depend on it.
func TestReadFrom_OrderIndependent(t *testing.T) {
	forwards := ReadFrom("testdata/composer", []string{
		"composer.json", "src/Component/Order/composer.json", "src/Bundle/OrderBundle/composer.json",
	})
	backwards := ReadFrom("testdata/composer", []string{
		"src/Bundle/OrderBundle/composer.json", "src/Component/Order/composer.json", "composer.json",
	})
	assert.Equal(t, forwards.NameOf("src/Component/Order/Model/Order.php"),
		backwards.NameOf("src/Component/Order/Model/Order.php"))
	assert.Equal(t, forwards.Len(), backwards.Len())
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

// A manifest that names nothing declares nothing. A dotnet project is named
// by its filename so it always has one; the others can be anonymous.
func TestReaders_AnonymousManifestsAreSkipped(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "composer.json", `{"require":{"php":"^8.2"}}`)
	writeFile(t, root, "pkg/package.json", `{"version":"1.0.0"}`)
	writeFile(t, root, "mod/go.mod", "go 1.23\n")
	assert.Equal(t, 0, Read(root).Len())
}

// Unparseable manifests must not take the run with them: one bad file in a
// large monorepo is common and is not a reason to analyse nothing.
func TestReaders_MalformedManifestsAreSkipped(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "bad/composer.json", `{ not json at all`)
	writeFile(t, root, "bad/pom.xml", `<project><artifactId>unclosed`)
	writeFile(t, root, "good/composer.json", `{"name":"acme/good"}`)

	m := Read(root)
	assert.Equal(t, 1, m.Len())
	assert.NotNil(t, m.ByName("acme/good"))
}

// A .csproj that cannot be parsed still declares a module: its name is the
// filename, and only its dependencies are lost.
func TestDotnet_UnparseableProjectStillNamesAModule(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "src/Acme.Core/Acme.Core.csproj", `<Project><ItemGroup>`)

	m := Read(root)
	require.Equal(t, 1, m.Len())
	assert.Equal(t, "Acme.Core", m.Modules()[0].Name)
	assert.Empty(t, m.Modules()[0].DependsOn)
}

// The root build script of a multi-module gradle build configures the build
// rather than declaring a module anybody depends on.
func TestGradle_RootBuildScriptIsNotAModule(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "build.gradle.kts", `plugins { kotlin("jvm") }`)
	writeFile(t, root, "core/build.gradle.kts", `dependencies { api(project(":other")) }`)

	m := Read(root)
	require.Equal(t, 1, m.Len())
	assert.Equal(t, "core", m.Modules()[0].Name)
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
