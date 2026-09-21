package rules

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A rule that cannot fire is worse than no rule: it reports a clean bill of
// health nobody earned. Every shipped rule is checked here against a module
// name that must break it and one that must not.

func TestShippedRulesCompile(t *testing.T) {
	loaded, err := loadRules(ruleDefs)
	require.NoError(t, err)
	require.NotEmpty(t, loaded)
	for _, r := range loaded {
		assert.NotEmptyf(t, r.rule.Id, "every rule needs an id")
		assert.NotEmptyf(t, r.rule.Name, "%s needs a name", r.rule.Id)
		assert.NotEmptyf(t, r.rule.ShortDescription, "%s needs a short_description", r.rule.Id)
		assert.Truef(t, r.from.matches != nil || r.from.dirMatches != nil,
			"%s selects no source module, so it can never fire", r.rule.Id)
		assert.Truef(t, r.to.matches != nil || r.to.dirMatches != nil,
			"%s selects no target module, so it can never fire", r.rule.Id)
	}
}

func TestShippedRulesFireAndStaySilent(t *testing.T) {
	loaded, err := loadRules(ruleDefs)
	require.NoError(t, err)
	byId := map[string]*compiledRule{}
	for _, r := range loaded {
		byId[r.rule.Id] = r
	}

	cases := []struct {
		id string
		// modules present in the project, for applies_when
		modules []NamedDir
		// an edge that must be reported
		badFrom, badTo NamedDir
		// an edge that must not be
		okFrom, okTo NamedDir
	}{
		{
			// nopCommerce's real shape: plugins build on Nop.Web.Framework,
			// and the day Nop.Core references one, the plugin is no longer
			// optional.
			id: "rules__dotnet__core_must_not_depend_on_plugin",
			modules: []NamedDir{
				{"Nop.Core", "src/Libraries/Nop.Core"},
				{"Nop.Web.Framework", "src/Presentation/Nop.Web.Framework"},
				{"Nop.Plugin.Payments.PayPal", "src/Plugins/Nop.Plugin.Payments.PayPal"},
			},
			badFrom: NamedDir{"Nop.Core", "src/Libraries/Nop.Core"},
			badTo:   NamedDir{"Nop.Plugin.Payments.PayPal", "src/Plugins/Nop.Plugin.Payments.PayPal"},
			okFrom:  NamedDir{"Nop.Plugin.Payments.PayPal", "src/Plugins/Nop.Plugin.Payments.PayPal"},
			okTo:    NamedDir{"Nop.Web.Framework", "src/Presentation/Nop.Web.Framework"},
		},
		{
			// Sylius: the domain must stay usable without Symfony. Note the
			// package names say nothing -- `sylius/order` is the component --
			// so the rule reads where each package sits.
			id: "rules__symfony__component_must_not_depend_on_bundle",
			modules: []NamedDir{
				{"sylius/order", "src/Sylius/Component/Order"},
				{"sylius/order-bundle", "src/Sylius/Bundle/OrderBundle"},
			},
			badFrom: NamedDir{"sylius/order", "src/Sylius/Component/Order"},
			badTo:   NamedDir{"sylius/order-bundle", "src/Sylius/Bundle/OrderBundle"},
			okFrom:  NamedDir{"sylius/order-bundle", "src/Sylius/Bundle/OrderBundle"},
			okTo:    NamedDir{"sylius/order", "src/Sylius/Component/Order"},
		},
		{
			id: "rules__go__internal_must_not_be_imported_from_outside",
			modules: []NamedDir{
				{"github.com/acme/app", "app"},
				{"github.com/other/lib", "lib"},
			},
			badFrom: NamedDir{"github.com/other/lib", "lib"},
			badTo:   NamedDir{"github.com/acme/app/internal/store", "app/internal/store"},
			okFrom:  NamedDir{"github.com/other/lib", "lib"},
			okTo:    NamedDir{"github.com/acme/app/store", "app/store"},
		},
	}

	require.Len(t, cases, len(loaded), "every shipped rule needs a case here")

	for _, c := range cases {
		t.Run(c.id, func(t *testing.T) {
			r, ok := byId[c.id]
			require.Truef(t, ok, "no rule with id %s", c.id)

			assert.Truef(t, r.when.any(c.modules), "rule should apply to %v", c.modules)

			assert.Truef(t, r.from.selects(c.badFrom.Name, c.badFrom.Dir) && r.to.selects(c.badTo.Name, c.badTo.Dir),
				"%s -> %s must be reported", c.badFrom.Name, c.badTo.Name)
			assert.Falsef(t, r.from.selects(c.okFrom.Name, c.okFrom.Dir) && r.to.selects(c.okTo.Name, c.okTo.Dir),
				"%s -> %s is the allowed direction and must not be reported", c.okFrom.Name, c.okTo.Name)
		})
	}
}

// A rule written for one ecosystem must stay quiet in a codebase that is not
// that ecosystem. Without applies_when, "core must not depend on a plugin"
// reports on any project with the word Plugin in a module name.
func TestAppliesWhenKeepsRulesQuiet(t *testing.T) {
	loaded, err := loadRules(ruleDefs)
	require.NoError(t, err)
	plain := []NamedDir{{"github.com/acme/app", ""}, {"github.com/acme/app/store", "store"}}

	for _, r := range loaded {
		if r.rule.Id == "rules__go__internal_must_not_be_imported_from_outside" {
			continue // deliberately universal
		}
		assert.Falsef(t, r.when.any(plain), "%s should not apply to %v", r.rule.Id, plain)
	}
}

// An empty selector selects nothing. A rule that forgot to say what it is
// about must report no violations rather than every edge in the codebase.
func TestEmptySelectorSelectsNothing(t *testing.T) {
	c, err := Selector{}.compile()
	require.NoError(t, err)
	assert.False(t, c.selects("anything", "anywhere"))
	// But an empty applies_when means "always".
	assert.True(t, c.any([]NamedDir{{"anything", "anywhere"}}))
}
