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

// One real project per language archstats supports.
//
// The failure these guard against is silent. The analyser runs, reports
// success, and hands back a codebase filed under a single component called
// "Unknown" — because the query it uses never matched the way that language
// is actually written today. nopCommerce, 3,650 C# files, came back as five
// components: its namespaces are file-scoped and the query knew only the
// block form. Express came back with no dependencies at all: it is CommonJS
// and the query knew only ESM. Neither showed up as an error, and no unit
// test on a hand-written fixture would have caught either, because the
// fixtures were written in the dialect the query already matched.
//
// So each case is a real project, pinned to a commit and fetched at depth 1.
// The whole set is about twenty megabytes and a second run touches no
// network, because the checkout is already on disk.

type languageFixture struct {
	name   string
	repo   string
	commit string

	// Components that must be present, naming the idiom under test: a
	// package, a namespace, a module directory. A pack that has quietly
	// stopped matching still produces components — the directory fallback
	// covers for it — so a count alone proves nothing and the names do.
	wantComponents []string

	// Nothing is learnt from a codebase that is one component.
	minComponents int

	// Somebody must depend on somebody. A pack that parses every file and
	// matches no imports yields a full component list and an empty graph,
	// which is exactly what express looked like.
	minCoupling int
}

var languageFixtures = []languageFixture{
	{
		name:           "java",
		repo:           "https://github.com/RyanSusana/elepy",
		commit:         "83d3069d4f8c7136d8b2676fa1b186294a402a84",
		wantComponents: []string{"com.elepy", "com.elepy.annotations"},
		minComponents:  30,
		minCoupling:    50,
	},
	{
		name:   "csharp",
		repo:   "https://github.com/jbogard/MediatR",
		commit: "916ef1b3d68ccdc96db8f914eaf1b32fc7db52c5",
		// File-scoped namespaces — `namespace MediatR;` with no block — are
		// 140 of this repo's 151 C# files, and a different grammar node from
		// the block form.
		wantComponents: []string{"MediatR", "MediatR.Examples"},
		minComponents:  20,
		minCoupling:    20,
	},
	{
		name:           "kotlin",
		repo:           "https://github.com/Kotlin/kotlinx-datetime",
		commit:         "b372c49e208779037408832f0d7159a0f985ebbd",
		wantComponents: []string{"kotlinx.datetime", "kotlinx.datetime.internal"},
		minComponents:  10,
		minCoupling:    10,
	},
	{
		name:   "python",
		repo:   "https://github.com/psf/requests",
		commit: "dae7ef63b4df6eded86637f251fc4e3a06c3b479",
		// Python names no component of its own: the directory is the package,
		// which is what the language pack asks for and what the default
		// resolution strategy has to preserve.
		wantComponents: []string{"src/requests", "tests"},
		minComponents:  3,
		minCoupling:    1,
	},
	{
		name:   "javascript",
		repo:   "https://github.com/expressjs/express",
		commit: "9a34acf03cb818ff3f8bc40e44176e277a25cbb9",
		// CommonJS throughout: 66 `require()` calls, zero ESM imports.
		wantComponents: []string{"lib", "test"},
		minComponents:  10,
		minCoupling:    50,
	},
	{
		name:           "typescript",
		repo:           "https://github.com/pmndrs/zustand",
		commit:         "b57db4f86ef179285da216eeb291266da82c361c",
		wantComponents: []string{"src", "src/middleware"},
		minComponents:  5,
		minCoupling:    5,
	},
}

func Test_Languages_RealProjects(t *testing.T) {
	for _, fixture := range languageFixtures {
		t.Run(fixture.name, func(t *testing.T) {
			components := componentsOf(t, fixture.repo, fixture.commit)

			names := make(map[string]bool, len(components))
			coupling := 0
			for _, c := range components {
				names[c.Name] = true
				coupling += c.EfferentCouplings
			}

			for _, want := range fixture.wantComponents {
				assert.Truef(t, names[want], "expected a component named %q in %s, got %d components: %v",
					want, fixture.repo, len(components), firstFew(components))
			}
			assert.GreaterOrEqualf(t, len(components), fixture.minComponents,
				"%s resolved into too few components to be useful", fixture.repo)
			assert.GreaterOrEqualf(t, coupling, fixture.minCoupling,
				"%s produced components but no dependencies between them, which means its imports were not recognised", fixture.repo)

			// The tell that a language went unread. Every file of a project
			// lands here when the component query matches nothing, and the
			// run still succeeds.
			assert.Falsef(t, names["Unknown"], "%s left files in \"Unknown\"", fixture.repo)
		})
	}
}

// componentsOf runs the real CLI against a real checkout, with the default
// settings — the ones the desktop app and anybody typing `archstats` get.
func componentsOf(t *testing.T, url, commit string) []Component {
	t.Helper()

	cloned, err := repo.EnsureCloned(url, commit)
	require.NoErrorf(t, err, "cloning %s", url)

	out := bytes.NewBufferString("")
	err = cmd.Execute(out, bytes.NewBufferString(""), nil,
		[]string{"-o", "csv", "-f", cloned.Location, "view", "components",
			"-c", "name,complexity__files,modularity__coupling__efferent"})
	require.NoErrorf(t, err, "analysing %s", url)

	var components []Component
	require.NoError(t, csvutil.Unmarshal(out.Bytes(), &components))
	require.NotEmptyf(t, components, "%s produced no components at all", url)
	return components
}

func firstFew(components []Component) []string {
	var out []string
	for i, c := range components {
		if i == 8 {
			break
		}
		out = append(out, c.Name)
	}
	return out
}
