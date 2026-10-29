package php

import (
	"testing"

	"github.com/archstats/archstats/core/module"
	"github.com/archstats/archstats/core/module/moduletest"
	"github.com/stretchr/testify/assert"
)

func TestManifestReaders(t *testing.T) {
	m := moduletest.Check(t, "testdata/manifests/composer", ManifestReaders(), moduletest.Want{
		Kind: "composer", Count: 3,
		Modules: map[string]string{
			"sylius/sylius":       "",
			"sylius/order":        "src/Component/Order",
			"sylius/order-bundle": "src/Bundle/OrderBundle",
		},
		Deps: map[string]string{"sylius/order-bundle": "sylius/order"},
	})
	// Sylius has 60 composer packages beneath one root composer.json, and
	// every file under src/Component/Order belongs to sylius/order.
	assert.Equal(t, "sylius/order", m.NameOf("src/Component/Order/Model/Order.php"))
	assert.Equal(t, "sylius/order-bundle", m.NameOf("src/Bundle/OrderBundle/DependencyInjection/Extension.php"))
	assert.Equal(t, "sylius/sylius", m.NameOf("src/Sylius/Kernel.php"))
}

// A composer.json that names nothing, or cannot be parsed, declares nothing.
func TestComposer_AnonymousAndMalformedAreSkipped(t *testing.T) {
	root := t.TempDir()
	moduletest.Write(t, root, "composer.json", `{"require":{"php":"^8.2"}}`)
	moduletest.Write(t, root, "bad/composer.json", `{ not json at all`)
	moduletest.Write(t, root, "good/composer.json", `{"name":"acme/good"}`)
	m := module.Read(root, ManifestReaders()...)
	assert.Equal(t, []string{"acme/good"}, moduletest.Names(m))
}
