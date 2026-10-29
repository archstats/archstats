package python

import (
	"testing"

	"github.com/archstats/archstats/core/module/moduletest"
)

// The app is named the way Python imports it -- `shop.orders` -- not
// `orders`, because `src` holds no __init__.py and `shop` does.
func TestManifestReaders(t *testing.T) {
	moduletest.Check(t, "testdata/manifests/django", ManifestReaders(), moduletest.Want{
		Kind: "django", Count: 1,
		Modules: map[string]string{"shop.orders": "src/shop/orders"},
	})
}
