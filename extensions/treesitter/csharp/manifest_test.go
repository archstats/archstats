package csharp

import (
	"testing"

	"github.com/archstats/archstats/core/module"
	"github.com/archstats/archstats/core/module/moduletest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestManifestReaders(t *testing.T) {
	moduletest.Check(t, "testdata/manifests/dotnet", ManifestReaders(), moduletest.Want{
		Kind: "dotnet", Count: 2,
		Modules: map[string]string{
			"Nop.Core":                   "src/Core",
			"Nop.Plugin.Payments.PayPal": "src/Plugins/Nop.Plugin.Payments.PayPal",
		},
		// `..\..\Core\Nop.Core.csproj` names a project, not a path.
		Deps: map[string]string{"Nop.Plugin.Payments.PayPal": "Nop.Core"},
	})
}

// A .csproj that cannot be parsed still declares a module: its name is the
// filename, and only its dependencies are lost.
func TestDotnet_UnparseableProjectStillNamesAModule(t *testing.T) {
	root := t.TempDir()
	moduletest.Write(t, root, "src/Acme.Core/Acme.Core.csproj", `<Project><ItemGroup>`)

	m := module.Read(root, ManifestReaders()...)
	require.Equal(t, 1, m.Len())
	assert.Equal(t, "Acme.Core", m.Modules()[0].Name)
	assert.Empty(t, m.Modules()[0].DependsOn)
}
