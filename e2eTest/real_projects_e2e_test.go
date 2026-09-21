package e2eTest

import (
	"bytes"
	"testing"

	"github.com/archstats/archstats/cmd"
	"github.com/archstats/archstats/e2eTest/repo"
	"github.com/jszwec/csvutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Modules, typed edges, unresolved lookups and units, checked against the
// projects they were designed from.
//
// The hand-built fixtures in architecture/ prove each mechanism works. They
// cannot prove it works on four thousand files written by people who had
// never heard of archstats, and every one of these behaviours was built
// because a real codebase did something no fixture author would think to
// write: a type split across four files, a third of a dependency graph
// resolved from strings, an eighth of an import list erased by the compiler.
//
// The projects here were chosen for evidence per megabyte. Two heavier ones
// that could not be replaced -- Sylius and django-oscar -- live in
// real_projects_heavy_e2e_test.go behind a build tag.

type unitRow struct {
	ID     string `csv:"ID"`
	Kind   string `csv:"KIND"`
	Files  int    `csv:"FILES"`
	Module string `csv:"MODULE"`
}

// C#: one type across many files, and many types in one file, in the same
// codebase. A per-file record of "the type in this file" cannot represent
// either direction.
//
// Polly rather than nopCommerce, which motivated this work: 14MB against
// 131MB, and better evidence for it. 38 of Polly's units are declared across
// more than one file -- `Polly.AsyncPolicy` across eight -- where
// nopCommerce, nine times the size, had 18.
//
// What is lost with nopCommerce is the plugin rule on a real codebase; Polly
// has no plugin projects, so that rule cannot apply here. It is covered by
// architecture/dotnet, which is built to break it.
func Test_Real_Polly_UnitsFoldAcrossFiles(t *testing.T) {
	skipIfShort(t)
	const url, commit = "https://github.com/App-vNext/Polly", "9a81fdc7c1a89d6c45bba2fb7b062b8eeae39eee"

	modules := realModules(t, url, commit)
	kinds := map[string]int{}
	for _, m := range modules {
		kinds[m.Kind]++
	}
	assert.GreaterOrEqualf(t, kinds["dotnet"], 20, "expected the project files to be read, got %v", kinds)

	var units []unitRow
	realView(t, url, commit, "units", "id,kind,files,module", &units)
	require.NotEmpty(t, units, "C# produced no units at all")

	spanning := 0
	var widest unitRow
	for _, u := range units {
		if u.Files > 1 {
			spanning++
		}
		if u.Files > widest.Files {
			widest = u
		}
	}
	// Without folding, each half of a partial class is its own type and the
	// attributes on one are invisible to the other.
	assert.GreaterOrEqualf(t, spanning, 20,
		"only %d units span several files; partial classes are not being folded", spanning)
	assert.GreaterOrEqualf(t, widest.Files, 4,
		"the most-split type is only in %d files, which is not the partial class this pins", widest.Files)

	for _, u := range units {
		assert.NotEmptyf(t, u.Module, "unit %s belongs to no module, so the module map did not reach units", u.ID)
		break
	}

	// Polly declares no plugins, so the .NET plugin rule has no opinion
	// about it. A violation here is the rule leaking into a codebase it is
	// not about; "not applicable" is the honest answer and not a pass.
	findings := realFindings(t, url, commit)
	require.NotEmpty(t, findings, "every rule should report a status")
	status := map[string]string{}
	for _, f := range findings {
		status[f.Rule] = f.Status
		assert.NotEqualf(t, "violation", f.Status, "no rule should fire on Polly: %v", f)
	}
	assert.Equal(t, "not_applicable", status["rules__dotnet__core_must_not_depend_on_plugin"])
}

// Python: a dependency named by a string and resolved when the program runs.
//
// celery rather than django-oscar, which motivated this work: 11MB against
// 80MB. The trade is real and worth stating -- celery has six such
// references where Oscar has 664 -- so this proves the mechanism reaches a
// real codebase, and the scale evidence stays in the heavy suite.
func Test_Real_Celery_DynamicEdgesAreResolved(t *testing.T) {
	skipIfShort(t)
	const url, commit = "https://github.com/celery/celery", "135b83c718f63612c1f1585a3f5823edb31e2d4b"

	var connections []ComponentConnectionDirect
	realView(t, url, commit, "component_connections_direct", "from,to,kind,file,reference_count", &connections)

	refs := map[string]int{}
	for _, c := range connections {
		refs[c.Kind] += c.ReferenceCount
	}
	assert.Greaterf(t, refs["import"], 500, "static imports went missing: %v", refs)
	// `importlib.import_module("celery.app.base")` names a module as a plain
	// string. Read as static imports only, these edges do not exist.
	assert.Positivef(t, refs["dynamic"], "import_module edges are not being resolved: %v", refs)

	var unresolved []UnresolvedEdge
	realView(t, url, commit, "unresolved_edges", "from,names,file,reason", &unresolved)
	for _, u := range unresolved {
		assert.NotContainsf(t, u.Names, "/",
			"unresolved lookups must report what the code wrote, got %q", u.Names)
	}
}

// TypeScript: `import type` is erased by the compiler. zustand is already in
// the language suite and writes a dozen of them, so this costs nothing.
func Test_Real_Zustand_TypeOnlyEdgesAreSeparated(t *testing.T) {
	const url, commit = "https://github.com/pmndrs/zustand", "b57db4f86ef179285da216eeb291266da82c361c"

	var connections []ComponentConnectionDirect
	realView(t, url, commit, "component_connections_direct", "from,to,kind,file,reference_count", &connections)

	kinds := map[string]int{}
	for _, c := range connections {
		kinds[c.Kind]++
	}
	assert.Positivef(t, kinds["type_only"],
		"zustand writes `import type`; none were separated out: %v", kinds)
	assert.Positive(t, kinds["import"], "runtime imports disappeared: %v", kinds)

	// Coupling is computed from the graph, which holds runtime edges only,
	// so the components view must not count the erased ones.
	var components []Component
	realView(t, url, commit, "components", "name,complexity__files,modularity__coupling__efferent", &components)
	total := 0
	for _, c := range components {
		total += c.EfferentCouplings
	}
	runtimeEdges := 0
	for _, c := range connections {
		if c.Kind != "type_only" {
			runtimeEdges++
		}
	}
	assert.LessOrEqualf(t, total, runtimeEdges,
		"efferent coupling counts more edges than there are runtime ones, so erased imports are being counted")
}

func skipIfShort(t *testing.T) {
	t.Helper()
	if testing.Short() {
		t.Skip("clones a large real project; run without -short")
	}
}

func realModules(t *testing.T, url, commit string) []Module {
	t.Helper()
	var out []Module
	realView(t, url, commit, "modules", "name,kind,directory", &out)
	return out
}

// Only the rows reporting a broken rule.
func realRules(t *testing.T, url, commit string) []RuleViolation {
	t.Helper()
	var out []RuleViolation
	for _, f := range realFindings(t, url, commit) {
		if f.Status == "violation" {
			out = append(out, f)
		}
	}
	return out
}

// Every rule's verdict, including the ones that held or never applied.
func realFindings(t *testing.T, url, commit string) []RuleViolation {
	t.Helper()
	var out []RuleViolation
	realView(t, url, commit, "rules", "rule,status,from,to,kind,file", &out)
	return out
}

func realView(t *testing.T, url, commit, view, columns string, into interface{}) {
	t.Helper()
	cloned, err := repo.EnsureCloned(url, commit)
	require.NoErrorf(t, err, "cloning %s", url)

	out := bytes.NewBufferString("")
	err = cmd.Execute(out, bytes.NewBufferString(""), nil,
		[]string{"-o", "csv", "-f", cloned.Location, "view", view, "-c", columns})
	require.NoErrorf(t, err, "rendering %s for %s", view, url)
	if out.Len() == 0 {
		return
	}
	require.NoError(t, csvutil.Unmarshal(out.Bytes(), into))
}
