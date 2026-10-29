package dart

import (
	"os"
	"testing"

	"github.com/archstats/archstats/core/module"
	"github.com/archstats/archstats/core/module/moduletest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Probe against a real checkout: ARCHSTATS_MODULE_PROBE=/path go test -run Probe -v
func TestProbe(t *testing.T) {
	dir := os.Getenv("ARCHSTATS_MODULE_PROBE")
	if dir == "" {
		t.Skip()
	}
	for _, mod := range module.Read(dir, ManifestReaders()...).Modules() {
		t.Logf("%-8s %-18s %-40s %-50s files=%d deps=%v", mod.Kind, mod.Type, mod.Name, mod.Dir, len(mod.Files), mod.DependsOn)
	}
}

func TestPub(t *testing.T) {
	root := t.TempDir()
	moduletest.Write(t, root, "mobile/pubspec.yaml", "name: immich_mobile\ndependencies:\n  flutter:\n    sdk: flutter\n  hooks_riverpod: ^2.4.0\n  openapi:\n    path: openapi\ndev_dependencies:\n  build_runner: any\n")
	moduletest.Write(t, root, "mobile/lib/main.dart", "void main() {}")
	moduletest.Write(t, root, "mobile/openapi/pubspec.yaml", "name: openapi\ndependencies:\n  http: any\n")
	m := module.Read(root, ManifestReaders()...)
	app := m.ByName("immich_mobile")
	require.NotNil(t, app)
	assert.Equal(t, "pub", app.Kind)
	assert.Equal(t, "flutter-app", app.Type)
	assert.Equal(t, "mobile", app.Dir)
	assert.Equal(t, []string{"flutter", "hooks_riverpod", "openapi", "build_runner"}, app.DependsOn)
	assert.Equal(t, "dart-package", m.ByName("openapi").Type)
	assert.Equal(t, "openapi", m.NameOf("mobile/openapi/lib/api.dart"))
}
