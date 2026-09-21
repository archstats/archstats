package components

import (
	"testing"

	"github.com/archstats/archstats/core"
	"github.com/archstats/archstats/core/file"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func dyn(component, names, f string, line int) *file.Snippet {
	return &file.Snippet{
		Type: file.ComponentImportDynamic, Component: component, Value: names, File: f,
		Begin: &file.Position{Line: line}, End: &file.Position{Line: line},
	}
}

func withSnippets(snippets []*file.Snippet, components ...string) *core.Results {
	byType := file.SnippetGroup{}
	for _, s := range snippets {
		byType[s.Type] = append(byType[s.Type], s)
	}
	byComponent := file.SnippetGroup{}
	for _, c := range components {
		byComponent[c] = []*file.Snippet{{Component: c}}
	}
	return &core.Results{SnippetsByType: byType, SnippetsByComponent: byComponent}
}

// A lookup that landed on a real component is an edge, not a gap.
func TestUnresolvedEdges_ResolvedLookupsAreNotReported(t *testing.T) {
	r := withSnippets([]*file.Snippet{
		dyn("shop/basket", "shop/catalogue", "shop/basket/models.py", 3),
	}, "shop/basket", "shop/catalogue")

	assert.Empty(t, UnresolvedEdgesView(r).Rows)
}

// The point of the view: a gap an architect can see and act on, with enough
// to find it. django-oscar resolves a third of its graph this way, and the
// handful that do not resolve are worth more on screen than dropped.
func TestUnresolvedEdges_ReportsWhereAndWhy(t *testing.T) {
	r := withSnippets([]*file.Snippet{
		dyn("shop/basket", "nowhere.at.all", "shop/basket/models.py", 7),
	}, "shop/basket")

	rows := UnresolvedEdgesView(r).Rows
	require.Len(t, rows, 1)
	assert.Equal(t, "shop/basket", rows[0].Data["from"])
	assert.Equal(t, "nowhere.at.all", rows[0].Data["names"])
	assert.Equal(t, "shop/basket/models.py", rows[0].Data["file"])
	assert.Equal(t, 7, rows[0].Data["line"])
	assert.Equal(t, "names a module this analysis did not see", rows[0].Data["reason"])
}

// A lookup whose module is named by a variable was never a string to begin
// with. It must be reported as such rather than guessed into an edge.
func TestUnresolvedEdges_DistinguishesAnExpressionFromAMissingModule(t *testing.T) {
	r := withSnippets([]*file.Snippet{dyn("shop/basket", "", "shop/basket/models.py", 9)}, "shop/basket")

	rows := UnresolvedEdgesView(r).Rows
	require.Len(t, rows, 1)
	assert.Equal(t, "named by an expression rather than a string", rows[0].Data["reason"])
}

// A codebase with no dynamic lookups reports nothing, which is the common
// case and must not look like a finding.
func TestUnresolvedEdges_QuietWhenNothingIsDynamic(t *testing.T) {
	assert.Empty(t, UnresolvedEdgesView(withSnippets(nil, "a", "b")).Rows)
}

// Columns are part of the contract: an export whose shape moves breaks
// every saved query against it.
func TestUnresolvedEdges_Columns(t *testing.T) {
	var names []string
	for _, c := range UnresolvedEdgesView(withSnippets(nil)).Columns {
		names = append(names, c.Name)
	}
	assert.Equal(t, []string{"from", "names", "file", "line", "reason"}, names)
}
