package rules

import (
	"testing"

	"github.com/archstats/archstats/core"
	"github.com/archstats/archstats/core/component"
	"github.com/archstats/archstats/core/file"
	"github.com/archstats/archstats/core/module"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// check() over a constructed Results, so the rule engine is exercised
// without a codebase behind it. The shipped rules have their own fixtures in
// e2eTest/architecture and their real projects in the real-project suite;
// these cover the wiring those cannot reach.

func loadedExtension(t *testing.T) *extension {
	t.Helper()
	loaded, err := loadRules(ruleDefs)
	require.NoError(t, err)
	return &extension{rules: loaded}
}

func results(modules []*module.Module, connections []*component.Connection, componentFiles map[string][]string, fileModules map[string]string) *core.Results {
	return &core.Results{
		Modules:          module.New(modules...),
		AllConnections:   connections,
		ComponentToFiles: componentFiles,
		FileToModule:     fileModules,
	}
}

func conn(from, to, kind, f string, line int) *component.Connection {
	return &component.Connection{From: from, To: to, Type: kind, File: f, Begin: &file.Position{Line: line}}
}

// Only the findings that report a broken rule. check() now returns a verdict
// for every rule, including the ones that held or never applied.
func violationsOf(findings []*Finding) []*Finding {
	var out []*Finding
	for _, f := range findings {
		if f.Status == StatusViolation {
			out = append(out, f)
		}
	}
	return out
}

func statusOf(findings []*Finding) map[string]string {
	out := map[string]string{}
	for _, f := range findings {
		if f.Status != StatusViolation {
			out[f.Rule] = f.Status
		}
	}
	return out
}

func present(modules []*module.Module) []NamedDir {
	var out []NamedDir
	for _, m := range modules {
		out = append(out, NamedDir{Name: m.Name, Dir: m.Dir})
	}
	return out
}

func TestCheck_ReportsAnImportEdgeWithItsLine(t *testing.T) {
	e := loadedExtension(t)
	modules := []*module.Module{
		{Name: "acme/order", Dir: "src/Component/Order", Kind: "composer"},
		{Name: "acme/order-bundle", Dir: "src/Bundle/OrderBundle", Kind: "composer"},
	}
	r := results(modules,
		[]*component.Connection{conn("Acme\\Component\\Order", "Acme\\Bundle\\OrderBundle", component.KindImport, "src/Component/Order/Model/Order.php", 16)},
		map[string][]string{"Acme\\Bundle\\OrderBundle": {"src/Bundle/OrderBundle/X.php"}},
		map[string]string{
			"src/Component/Order/Model/Order.php": "acme/order",
			"src/Bundle/OrderBundle/X.php":        "acme/order-bundle",
		})

	violations := violationsOf(e.check(r, present(modules)))
	require.Len(t, violations, 1)
	assert.Equal(t, "rules__symfony__component_must_not_depend_on_bundle", violations[0].Rule)
	assert.Equal(t, "acme/order", violations[0].From)
	assert.Equal(t, 16, violations[0].Line, "an architect needs the line, not just the file")
}

// A dependency declared in a manifest with no import anywhere is still a
// dependency, and in .NET it is the usual way a plugin reaches core.
func TestCheck_ReportsManifestOnlyDependencies(t *testing.T) {
	e := loadedExtension(t)
	modules := []*module.Module{
		{Name: "Acme.Core", Dir: "src/Core", Kind: "dotnet", Manifest: "src/Core/Acme.Core.csproj",
			DependsOn: []string{"Acme.Plugin.Widget", "Newtonsoft.Json"}},
		{Name: "Acme.Plugin.Widget", Dir: "src/Plugins/Acme.Plugin.Widget", Kind: "dotnet"},
	}
	r := results(modules, nil, nil, nil)

	violations := violationsOf(e.check(r, present(modules)))
	require.Len(t, violations, 1)
	assert.Equal(t, component.KindManifest, violations[0].Kind)
	assert.Equal(t, "src/Core/Acme.Core.csproj", violations[0].File)
	// Newtonsoft.Json is not a module of this project, so it is not an edge.
	assert.Equal(t, "Acme.Plugin.Widget", violations[0].To)
}

// A rule is about what code is allowed to know. Depending on a bundle only
// for its types is still depending on it.
func TestCheck_TypeOnlyEdgesStillBreakRules(t *testing.T) {
	e := loadedExtension(t)
	modules := []*module.Module{
		{Name: "acme/order", Dir: "src/Component/Order", Kind: "composer"},
		{Name: "acme/order-bundle", Dir: "src/Bundle/OrderBundle", Kind: "composer"},
	}
	r := results(modules,
		[]*component.Connection{conn("C", "B", component.KindTypeOnly, "src/Component/Order/a.php", 3)},
		map[string][]string{"B": {"src/Bundle/OrderBundle/b.php"}},
		map[string]string{"src/Component/Order/a.php": "acme/order", "src/Bundle/OrderBundle/b.php": "acme/order-bundle"})

	assert.Len(t, violationsOf(e.check(r, present(modules))), 1)
}

// The allowed direction must never be reported. A rule that fires both ways
// is not a rule about direction.
func TestCheck_AllowedDirectionIsSilent(t *testing.T) {
	e := loadedExtension(t)
	modules := []*module.Module{
		{Name: "acme/order", Dir: "src/Component/Order", Kind: "composer"},
		{Name: "acme/order-bundle", Dir: "src/Bundle/OrderBundle", Kind: "composer",
			DependsOn: []string{"acme/order"}},
	}
	r := results(modules,
		[]*component.Connection{conn("B", "C", component.KindImport, "src/Bundle/OrderBundle/b.php", 5)},
		map[string][]string{"C": {"src/Component/Order/a.php"}},
		map[string]string{"src/Bundle/OrderBundle/b.php": "acme/order-bundle", "src/Component/Order/a.php": "acme/order"})

	assert.Empty(t, violationsOf(e.check(r, present(modules))))
}

// An edge inside one module is not a dependency between modules.
func TestCheck_EdgesWithinOneModuleAreIgnored(t *testing.T) {
	e := loadedExtension(t)
	modules := []*module.Module{{Name: "acme/order", Dir: "src/Component/Order", Kind: "composer"}}
	r := results(modules,
		[]*component.Connection{conn("C", "C2", component.KindImport, "src/Component/Order/a.php", 1)},
		map[string][]string{"C2": {"src/Component/Order/b.php"}},
		map[string]string{"src/Component/Order/a.php": "acme/order", "src/Component/Order/b.php": "acme/order"})

	assert.Empty(t, violationsOf(e.check(r, present(modules))))
}

// A project that declares no modules is not a pass and not a failure. Saying
// nothing beats a green tick nobody earned.
func TestCheck_NoModulesMeansNoVerdict(t *testing.T) {
	e := loadedExtension(t)
	r := results(nil,
		[]*component.Connection{conn("a", "b", component.KindImport, "a.go", 1)},
		map[string][]string{"b": {"b.go"}}, nil)

	// Nothing at all: not a row per rule claiming it held.
	assert.Empty(t, e.check(r, nil))
}

// A rule written for one ecosystem must stay quiet in another. Without
// applies_when, the .NET plugin rule reports on any project with a module
// whose name happens to contain the word Plugin.
func TestCheck_RulesDoNotLeakAcrossEcosystems(t *testing.T) {
	e := loadedExtension(t)
	modules := []*module.Module{
		{Name: "github.com/acme/app", Dir: "", Kind: "go"},
		{Name: "github.com/acme/app/store", Dir: "store", Kind: "go"},
	}
	r := results(modules,
		[]*component.Connection{conn("app", "store", component.KindImport, "main.go", 1)},
		map[string][]string{"store": {"store/s.go"}},
		map[string]string{"main.go": "github.com/acme/app", "store/s.go": "github.com/acme/app/store"})

	assert.Empty(t, violationsOf(e.check(r, present(modules))))
}

// A rule that reports nothing is ambiguous, and the ambiguity matters: it
// may have been checked and held, or it may never have applied. "Core must
// not depend on a plugin" says nothing about a project with no plugins, and
// showing that project a clean bill of health claims something nobody
// checked.
func TestCheck_EveryRuleReportsWhetherItApplied(t *testing.T) {
	e := loadedExtension(t)
	// A Go project: no .csproj, no composer packages, no bundles.
	modules := []*module.Module{
		{Name: "github.com/acme/app", Dir: "", Kind: "go"},
		{Name: "github.com/acme/app/store", Dir: "store", Kind: "go"},
	}
	r := results(modules, nil, nil, nil)

	status := statusOf(e.check(r, present(modules)))
	assert.Equal(t, StatusNotApplicable, status["rules__dotnet__core_must_not_depend_on_plugin"])
	assert.Equal(t, StatusNotApplicable, status["rules__symfony__component_must_not_depend_on_bundle"])
	// The internal rule is deliberately universal, and it holds here.
	assert.Equal(t, StatusOk, status["rules__go__internal_must_not_be_imported_from_outside"])
}

// A rule that fired reports its violations and nothing else: it must not
// also claim to have held.
func TestCheck_ABrokenRuleDoesNotAlsoReportOk(t *testing.T) {
	e := loadedExtension(t)
	modules := []*module.Module{
		{Name: "acme/order", Dir: "src/Component/Order", Kind: "composer"},
		{Name: "acme/order-bundle", Dir: "src/Bundle/OrderBundle", Kind: "composer"},
	}
	r := results(modules,
		[]*component.Connection{conn("C", "B", component.KindImport, "src/Component/Order/a.php", 3)},
		map[string][]string{"B": {"src/Bundle/OrderBundle/b.php"}},
		map[string]string{"src/Component/Order/a.php": "acme/order", "src/Bundle/OrderBundle/b.php": "acme/order-bundle"})

	status := statusOf(e.check(r, present(modules)))
	_, claimedOk := status["rules__symfony__component_must_not_depend_on_bundle"]
	assert.False(t, claimedOk, "a rule with violations must not also report a status of its own")
}
