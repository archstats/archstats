package javascript

import (
	"testing"

	"github.com/archstats/archstats/core/unit"
	"github.com/archstats/archstats/extensions/treesitter/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Sylius's Stimulus controllers are `export default class extends
// Controller`. The class had no name, so it was not a unit, and every method
// was a loose function at the top of the file: ProductTaxonTreeController
// "declared 15 separate things".
func TestAnAnonymousDefaultClassHoldsItsMethods(t *testing.T) {
	src := `import { Controller } from '@hotwired/stimulus';

export default class extends Controller {
    connect() {}
    clickNode(event) {}
}

const handlers = { onClick() {} };
`
	res := createJavaScriptLanguagePack().AnalyzeFileContent("assets/controllers/ProductTaxonTreeController.js", []byte(src))
	require.NotNil(t, res)
	byID := map[string]*unit.Unit{}
	for _, u := range common.JSUnitsFrom("assets/controllers/ProductTaxonTreeController.js", []byte(src), res) {
		byID[u.ID] = u
	}
	cls := byID["assets/controllers/ProductTaxonTreeController#ProductTaxonTreeController"]
	require.NotNil(t, cls, "the default-exported class is named for its file")
	assert.Equal(t, unit.KindType, cls.Kind)
	connect := byID["assets/controllers/ProductTaxonTreeController#ProductTaxonTreeController.connect"]
	require.NotNil(t, connect)
	assert.Equal(t, cls.ID, connect.Owner)
	assert.NotContains(t, byID, "assets/controllers/ProductTaxonTreeController#connect")
	// An object literal's method belongs to no class.
	assert.Empty(t, byID["assets/controllers/ProductTaxonTreeController#onClick"].Owner)
}
