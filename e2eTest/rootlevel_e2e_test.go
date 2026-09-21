package e2eTest

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Source files at the repository root must be analysed like any other.
//
// Worth pinning because it is easy to convince yourself they are not. The
// language packs glob `**/*.ts`, `**/*.py` and `**/*.{js,jsx,mjs,cjs}`, and
// those patterns genuinely do not match a bare `index.ts` -- which looks
// like every root-level file in every TypeScript, JavaScript and Python
// project is silently skipped.
//
// They are not, because the walker emits root-level files as `./index.ts`,
// with a separator, and the patterns match that. The behaviour is correct
// and depends on two things agreeing that nothing states out loud: the path
// format the walker produces, and the shape of every pack's glob. Change
// either and root-level files vanish with no error anywhere.

type fileRow struct {
	Name       string `csv:"NAME"`
	Component  string `csv:"COMPONENT"`
	TypesTotal int    `csv:"MODULARITY__TYPES__TOTAL"`
}

func Test_RootLevel_SourceFilesAreAnalysed(t *testing.T) {
	var files []fileRow
	runView(t, "rootlevel", "files", "name,component,modularity__types__total", &files)

	byName := map[string]fileRow{}
	for _, f := range files {
		byName[f.Name] = f
	}

	// One per ecosystem whose glob needs a separator to match.
	for _, name := range []string{"./index.ts", "./setup.py", "./main.js"} {
		row, found := byName[name]
		require.Truef(t, found, "%s was never analysed; got %v", name, keysOf(byName))
		assert.Positivef(t, row.TypesTotal, "%s produced no types, so its pack never matched it", name)
	}

	// And the nested files they sit beside, so a pass here cannot come from
	// the packs having stopped working altogether.
	for _, name := range []string{"sub/helper.ts", "sub/util.py", "sub/lib.js"} {
		_, found := byName[name]
		assert.Truef(t, found, "%s was never analysed", name)
	}
}

// A root-level file imports into a subdirectory, and that edge must exist.
// Its component is the root itself, which is the one component whose name is
// not a path anybody would type.
func Test_RootLevel_EdgesFromTheRootResolve(t *testing.T) {
	var connections []ComponentConnectionDirect
	runView(t, "rootlevel", "component_connections_direct", "from,to,kind,file,reference_count", &connections)

	var fromRoot int
	for _, c := range connections {
		if c.To == "sub" {
			fromRoot++
		}
	}
	assert.Positivef(t, fromRoot, "nothing at the root depends on sub/, got %v", connections)
}

func keysOf(m map[string]fileRow) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	return out
}
