package golang

import (
	"testing"

	"github.com/archstats/archstats/core/module"
	"github.com/archstats/archstats/core/module/moduletest"
	"github.com/stretchr/testify/assert"
)

func TestManifestReaders(t *testing.T) {
	moduletest.Check(t, "testdata/manifests/gomod", ManifestReaders(), moduletest.Want{
		Kind: "go", Count: 1,
		Modules: map[string]string{"github.com/archstats/archstats": ""},
		Deps:    map[string]string{"github.com/archstats/archstats": "github.com/samber/lo"},
	})
}

func TestGoMod_WithoutAModuleLineIsSkipped(t *testing.T) {
	root := t.TempDir()
	moduletest.Write(t, root, "mod/go.mod", "go 1.23\n")
	assert.Equal(t, 0, module.Read(root, ManifestReaders()...).Len())
}
