package ecmascript

import (
	"testing"

	"github.com/archstats/archstats/core/module"
	"github.com/archstats/archstats/core/module/moduletest"
	"github.com/stretchr/testify/assert"
)

func TestManifestReaders(t *testing.T) {
	moduletest.Check(t, "testdata/manifests/node", ManifestReaders(), moduletest.Want{
		Kind: "node", Count: 2,
		Modules: map[string]string{"librechat": "", "@librechat/shared": "packages/shared"},
	})
}

func TestNode_AnonymousPackageIsSkipped(t *testing.T) {
	root := t.TempDir()
	moduletest.Write(t, root, "pkg/package.json", `{"version":"1.0.0"}`)
	assert.Equal(t, 0, module.Read(root, ManifestReaders()...).Len())
}
