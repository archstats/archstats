package e2eTest

import (
	"bytes"
	"testing"

	"github.com/archstats/archstats/cmd"
	"github.com/jszwec/csvutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The three things Release A added, each checked end to end through the real
// CLI on a fixture small enough to hold in your head.
//
// A rule that cannot fire is worse than no rule: it reports a clean bill of
// health nobody earned. These fixtures are deliberately written to break the
// rules they are about, so a rule that silently stops matching fails here
// rather than reassuring somebody.

type RuleViolation struct {
	Rule   string `csv:"RULE"`
	Status string `csv:"STATUS"`
	From   string `csv:"FROM"`
	To     string `csv:"TO"`
	Kind   string `csv:"KIND"`
	File   string `csv:"FILE"`
}

type UnresolvedEdge struct {
	From   string `csv:"FROM"`
	Names  string `csv:"NAMES"`
	File   string `csv:"FILE"`
	Reason string `csv:"REASON"`
}

// .NET: plugins build on core, and the day core references a plugin that
// plugin is no longer optional. nopCommerce is 40 .csproj files arranged
// this way and its namespaces say nothing about which is which, so the rule
// is checked against the project references.
func Test_Architecture_DotnetCoreMustNotDependOnPlugin(t *testing.T) {
	violations := rulesOf(t, "architecture/dotnet")

	require.Len(t, violations, 1, "got: %v", violations)
	v := violations[0]
	assert.Equal(t, "rules__dotnet__core_must_not_depend_on_plugin", v.Rule)
	assert.Equal(t, "Acme.Core", v.From)
	assert.Equal(t, "Acme.Plugin.Widget", v.To)
	// The edge exists only in the manifest: no C# file imports the plugin.
	assert.Equal(t, "manifest", v.Kind)

	// The allowed direction -- plugin depending on core -- is present in the
	// same fixture and must not be reported.
	for _, other := range violations {
		assert.NotEqual(t, "Acme.Plugin.Widget", other.From)
	}
}

// Symfony: the framework-agnostic component must stay usable without the
// framework. Sylius is 60 composer packages built on this, and the package
// names -- `sylius/order`, `sylius/promotion` -- say nothing about it, so the
// rule reads where each package sits.
func Test_Architecture_SymfonyComponentMustNotDependOnBundle(t *testing.T) {
	violations := rulesOf(t, "architecture/symfony")

	require.Len(t, violations, 1, "got: %v", violations)
	v := violations[0]
	assert.Equal(t, "rules__symfony__component_must_not_depend_on_bundle", v.Rule)
	assert.Equal(t, "acme/order", v.From)
	assert.Equal(t, "acme/order-bundle", v.To)
	// A real `use` statement, not a manifest entry: the bundle's composer.json
	// requires the component, which is the allowed direction.
	assert.Equal(t, "import", v.Kind)
	assert.Contains(t, v.File, "Order.php")
}

// Django resolves a third of django-oscar's dependency graph from strings at
// runtime. Two of the four lookups here name something real and must become
// edges; two do not and must be reported rather than dropped.
func Test_Architecture_DynamicLookupsResolvedAndReported(t *testing.T) {
	connections := connectionsOf(t, "architecture/python")

	var dynamic []ComponentConnectionDirect
	for _, c := range connections {
		if c.Kind == "dynamic" {
			dynamic = append(dynamic, c)
		}
	}
	// `get_model("catalogue", ...)` names an app label -- a single bare
	// segment, which every path-shaped resolution branch ignores -- and
	// `get_class("catalogue.models", ...)` names a module. Both point at the
	// same app.
	require.NotEmpty(t, dynamic, "no dynamic edges: all of them are invisible again")
	for _, c := range dynamic {
		assert.Contains(t, c.To, "catalogue")
	}

	unresolved := unresolvedOf(t, "architecture/python")
	names := map[string]string{}
	for _, u := range unresolved {
		names[u.Names] = u.Reason
	}
	// Named a module nothing in this codebase answers to. Reporting it is
	// the point: an architect can weigh a gap they can see.
	assert.Containsf(t, names, "nowhere.at.all",
		"a lookup naming nothing must be reported, got %v", names)
	// Named by a variable, so there was never a string to capture. It must
	// not be guessed into an edge.
	for _, u := range unresolved {
		assert.NotEqual(t, "some_variable", u.Names)
	}
}

// A project that declares no modules is not a rule violation, and it is not
// a pass either. Most single-package repositories declare none, and a rule
// about modules cannot be checked where there are none -- so the view says
// nothing at all rather than reporting every rule as held.
func Test_Architecture_NoModulesMeansNoVerdict(t *testing.T) {
	assert.Empty(t, findingsOf(t, "simple_components"))
}

// A rule that found nothing has to say which kind of nothing it found.
// "Core must not depend on a plugin" has no opinion about a project with no
// plugins, and reporting that project as clean claims something nobody
// checked.
func Test_Architecture_RulesReportWhetherTheyApplied(t *testing.T) {
	status := map[string]string{}
	for _, f := range findingsOf(t, "architecture/symfony") {
		status[f.Rule] = f.Status
	}

	assert.Equal(t, "violation", status["rules__symfony__component_must_not_depend_on_bundle"])
	// A PHP project has no .NET projects, so that rule has no opinion.
	assert.Equal(t, "not_applicable", status["rules__dotnet__core_must_not_depend_on_plugin"])
	// The internal rule applies everywhere and holds here.
	assert.Equal(t, "ok", status["rules__go__internal_must_not_be_imported_from_outside"])
}

// Only the rows that report a broken rule. Every rule also reports whether
// it held or never applied, which findingsOf returns in full.
func rulesOf(t *testing.T, dir string) []RuleViolation {
	t.Helper()
	var out []RuleViolation
	for _, f := range findingsOf(t, dir) {
		if f.Status == "violation" {
			out = append(out, f)
		}
	}
	return out
}

func findingsOf(t *testing.T, dir string) []RuleViolation {
	t.Helper()
	var out []RuleViolation
	runView(t, dir, "rules", "rule,status,from,to,kind,file", &out)
	return out
}

func unresolvedOf(t *testing.T, dir string) []UnresolvedEdge {
	t.Helper()
	var out []UnresolvedEdge
	runView(t, dir, "unresolved_edges", "from,names,file,reason", &out)
	return out
}

func connectionsOf(t *testing.T, dir string) []ComponentConnectionDirect {
	t.Helper()
	var out []ComponentConnectionDirect
	runView(t, dir, "component_connections_direct", "from,to,kind,file,reference_count", &out)
	return out
}

func runView(t *testing.T, dir, view, columns string, into interface{}) {
	t.Helper()
	out := bytes.NewBufferString("")
	err := cmd.Execute(out, bytes.NewBufferString(""), nil,
		[]string{"-o", "csv", "-f", dir, "view", view, "-c", columns})
	require.NoErrorf(t, err, "rendering %s for %s", view, dir)
	if out.Len() == 0 {
		return
	}
	require.NoError(t, csvutil.Unmarshal(out.Bytes(), into))
}
