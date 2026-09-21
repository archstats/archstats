//go:build heavy_e2e

package e2eTest

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The two fixtures that could not be made smaller without losing the thing
// they prove. Excluded from an ordinary run; `go test -tags heavy_e2e ./...`
// includes them.
//
// Everything else in the real-project suite was replaced with something
// lighter: nopCommerce (131MB) by Polly (14MB), which is better evidence for
// partial classes anyway, and django-oscar (80MB) by celery (11MB), which is
// weaker but proves the mechanism. These two resisted.
//
//   - Sylius, 107MB. It is the only real codebase found that breaks its own
//     Component/Bundle rule, in exactly one place, which makes it the only
//     test that catches both a rule that stops firing and a rule that starts
//     over-firing. Sylius also publishes each bundle as its own small
//     repository -- SyliusGridBundle is 2.3MB and has the same layout on
//     disk -- but there composer.json sits at the root, the module's
//     directory is "" and the rule cannot see the split at all.
//
//   - django-oscar, 80MB. 664 of its dependency references are resolved from
//     strings at runtime against 973 static ones, which is the evidence for
//     the claim in ADR 0019 that roughly 41% of that graph was invisible.
//     Nothing smaller comes close: celery has 6, scrapy 2, django-allauth 0.
//     wagtail has comparable volume and is 68MB, which saves nothing.

// Sylius breaks its own rule once. Finding none means the rule stopped
// firing; finding several means it started matching things that are not
// violations -- reading a path segment instead of the module's own directory
// reported sixty, because `Bundle/AdminBundle/Twig/Component/` is a Twig UI
// folder and not a domain package.
func Test_Heavy_Sylius_ComponentMustNotDependOnBundle(t *testing.T) {
	const url, commit = "https://github.com/Sylius/Sylius", "6b24cc10fab893e2448c0c274775121b7e0b047a"

	modules := realModules(t, url, commit)
	kinds := map[string]int{}
	for _, m := range modules {
		kinds[m.Kind]++
	}
	assert.GreaterOrEqualf(t, kinds["composer"], 50, "expected the composer packages to be read, got %v", kinds)

	violations := realRules(t, url, commit)
	require.Lenf(t, violations, 1, "Sylius breaks this rule exactly once, got %v", violations)

	v := violations[0]
	assert.Equal(t, "rules__symfony__component_must_not_depend_on_bundle", v.Rule)
	assert.Equal(t, "sylius/promotion", v.From)
	assert.Equal(t, "sylius/promotion-bundle", v.To)
	// Sylius\Component\Promotion\Repository\CatalogPromotionRepositoryInterface
	// imports Sylius\Bundle\PromotionBundle\Criteria\CriteriaInterface.
	assert.Contains(t, v.File, "CatalogPromotionRepositoryInterface.php")
	assert.Equal(t, "import", v.Kind)
}

// The evidence for ADR 0019. Oscar resolves roughly a third of its
// dependency graph from strings, because every app must be overridable;
// read as static imports only, a third of that codebase is missing and
// nothing says so.
func Test_Heavy_DjangoOscar_MostOfTheGraphIsDynamic(t *testing.T) {
	const url, commit = "https://github.com/django-oscar/django-oscar", "0dfeaa797f4ed3c733e0449e70bc0a1da1a37203"

	modules := realModules(t, url, commit)
	kinds := map[string]int{}
	for _, m := range modules {
		kinds[m.Kind]++
	}
	assert.GreaterOrEqualf(t, kinds["django"], 25, "expected the Django apps to be found, got %v", kinds)

	var connections []ComponentConnectionDirect
	realView(t, url, commit, "component_connections_direct", "from,to,kind,file,reference_count", &connections)

	refs := map[string]int{}
	for _, c := range connections {
		refs[c.Kind] += c.ReferenceCount
	}
	assert.Greaterf(t, refs["import"], 500, "static imports went missing: %v", refs)
	assert.Greaterf(t, refs["dynamic"], 400,
		"get_class and get_model edges are not being resolved: %v", refs)
	// The claim this pins: these edges are not a rounding error on the graph.
	assert.Greaterf(t, refs["dynamic"]*100/(refs["import"]+refs["dynamic"]), 30,
		"dynamic edges should be about 40%% of this graph: %v", refs)

	var unresolved []UnresolvedEdge
	realView(t, url, commit, "unresolved_edges", "from,names,file,reason", &unresolved)
	assert.Lessf(t, len(unresolved), 100, "most dynamic lookups should resolve; %d did not", len(unresolved))
	for _, u := range unresolved {
		// What the code wrote, so it can be searched for. The resolver's
		// intermediate form -- `nowhere/at/all` for `nowhere.at.all` --
		// appears in no file and helps nobody.
		assert.NotContainsf(t, u.Names, "/",
			"unresolved lookups must report what the code wrote, got %q", u.Names)
	}
}
