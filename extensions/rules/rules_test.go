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
				{Name: "Nop.Core", Dir: "src/Libraries/Nop.Core"},
				{Name: "Nop.Web.Framework", Dir: "src/Presentation/Nop.Web.Framework"},
				{Name: "Nop.Plugin.Payments.PayPal", Dir: "src/Plugins/Nop.Plugin.Payments.PayPal"},
			},
			badFrom: NamedDir{Name: "Nop.Core", Dir: "src/Libraries/Nop.Core"},
			badTo:   NamedDir{Name: "Nop.Plugin.Payments.PayPal", Dir: "src/Plugins/Nop.Plugin.Payments.PayPal"},
			okFrom:  NamedDir{Name: "Nop.Plugin.Payments.PayPal", Dir: "src/Plugins/Nop.Plugin.Payments.PayPal"},
			okTo:    NamedDir{Name: "Nop.Web.Framework", Dir: "src/Presentation/Nop.Web.Framework"},
		},
		{
			// Sylius: the domain must stay usable without Symfony. Note the
			// package names say nothing -- `sylius/order` is the component --
			// so the rule reads where each package sits.
			id: "rules__symfony__component_must_not_depend_on_bundle",
			modules: []NamedDir{
				{Name: "sylius/order", Dir: "src/Sylius/Component/Order"},
				{Name: "sylius/order-bundle", Dir: "src/Sylius/Bundle/OrderBundle"},
			},
			badFrom: NamedDir{Name: "sylius/order", Dir: "src/Sylius/Component/Order"},
			badTo:   NamedDir{Name: "sylius/order-bundle", Dir: "src/Sylius/Bundle/OrderBundle"},
			okFrom:  NamedDir{Name: "sylius/order-bundle", Dir: "src/Sylius/Bundle/OrderBundle"},
			okTo:    NamedDir{Name: "sylius/order", Dir: "src/Sylius/Component/Order"},
		},
		{
			id: "rules__go__internal_must_not_be_imported_from_outside",
			modules: []NamedDir{
				{Name: "github.com/acme/app", Dir: "app", Kind: "go"},
				{Name: "github.com/other/lib", Dir: "lib", Kind: "go"},
			},
			badFrom: NamedDir{Name: "github.com/other/lib", Dir: "lib", Kind: "go"},
			badTo:   NamedDir{Name: "github.com/acme/app/internal/store", Dir: "app/internal/store"},
			okFrom:  NamedDir{Name: "github.com/other/lib", Dir: "lib", Kind: "go"},
			okTo:    NamedDir{Name: "github.com/acme/app/store", Dir: "app/store"},
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
	plain := []NamedDir{{Name: "org.acme:app", Dir: "", Kind: "maven"}, {Name: "@acme/store", Dir: "store", Kind: "node"}}

	for _, r := range loaded {
		assert.Falsef(t, r.when.any(plain), "%s should not apply to %v", r.rule.Id, plain)
	}
}

// The Go internal/ rule once applied to every codebase and so reported "kept"
// on Java and TypeScript projects that have no internal/ boundary at all. It
// applies where there is a Go module, and nowhere else.
func TestGoInternalRuleAppliesOnlyToGoModules(t *testing.T) {
	loaded, err := loadRules(ruleDefs)
	require.NoError(t, err)
	for _, r := range loaded {
		if r.rule.Id != "rules__go__internal_must_not_be_imported_from_outside" {
			continue
		}
		assert.True(t, r.when.any([]NamedDir{{Name: "github.com/acme/app", Dir: "", Kind: "go"}}))
		assert.False(t, r.when.any([]NamedDir{{Name: "org.acme:app", Dir: "", Kind: "maven"}}))
		assert.False(t, r.when.any(nil))
		return
	}
	t.Fatal("go internal rule not loaded")
}

// An empty selector selects nothing. A rule that forgot to say what it is
// about must report no violations rather than every edge in the codebase.
func TestEmptySelectorSelectsNothing(t *testing.T) {
	c, err := Selector{}.compile()
	require.NoError(t, err)
	assert.False(t, c.selects("anything", "anywhere"))
	// But an empty applies_when means "always".
	assert.True(t, c.any([]NamedDir{{Name: "anything", Dir: "anywhere"}}))
}
