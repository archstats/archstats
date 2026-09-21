package basic

import (
	"testing"

	"github.com/archstats/archstats/core"
	"github.com/archstats/archstats/core/module"
	"github.com/archstats/archstats/core/unit"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestModuleView_SeparatesOwnPackagesFromThirdParty(t *testing.T) {
	// LibreChat's client declares 116 dependencies and five of them are its
	// own. Only those five are edges inside the codebase.
	r := &core.Results{
		Modules: module.New(
			&module.Module{Name: "@acme/web", Dir: "client", Kind: "node", Manifest: "client/package.json",
				DependsOn: []string{"@acme/shared", "react", "lodash"}},
			&module.Module{Name: "@acme/shared", Dir: "packages/shared", Kind: "node", Manifest: "packages/shared/package.json"},
		),
		ModuleToFiles: map[string][]string{"@acme/web": {"client/a.ts", "client/b.ts"}},
	}

	rows := moduleView(r).Rows
	require.Len(t, rows, 2)

	byName := map[string]core.RowData{}
	for _, row := range rows {
		byName[row.Data["name"].(string)] = row.Data
	}
	web := byName["@acme/web"]
	assert.Equal(t, 3, web["declared_dependencies"])
	assert.Equal(t, 1, web["internal_dependencies"], "react and lodash are not modules of this project")
	assert.Equal(t, "@acme/shared", web["depends_on"])
	assert.Equal(t, 2, web["files"])
	assert.Equal(t, "node", web["kind"])
}

// Most repositories declare no modules at all. That is a real answer, not a
// failure, and it must render as an empty table rather than a crash.
func TestModuleView_NoModules(t *testing.T) {
	v := moduleView(&core.Results{Modules: module.New()})
	assert.Empty(t, v.Rows)
	assert.NotEmpty(t, v.Columns, "the shape of the table does not depend on the data")
}

func TestUnitView_RendersKindAndSpanningFiles(t *testing.T) {
	r := &core.Results{Units: []*unit.Unit{
		{ID: "Acme.Core.Customer", Kind: unit.KindType, Name: "Customer",
			Files: []string{"a.cs", "b.cs"}, Component: "Acme.Core", Module: "Acme.Core",
			Markers: []unit.Marker{
				{Source: unit.SourceAnnotation, Key: "Table", Value: "customers"},
				{Source: unit.SourceSupertype, Key: "BaseEntity"},
			}},
		{ID: "lib/router.go#Engine.ServeHTTP", Kind: unit.KindFunction, Name: "ServeHTTP",
			Files: []string{"lib/router.go"}, Owner: "Engine"},
	}}

	rows := unitView(r).Rows
	require.Len(t, rows, 2)

	assert.Equal(t, 2, rows[0].Data["files"], "a partial class is one unit over several files")
	assert.Equal(t, "a.cs", rows[0].Data["file"])
	// Evidence is qualified: "annotation:Table" and "filename:Table" are not
	// the same claim and a reader must not have to guess which they see.
	assert.Equal(t, "annotation:Table=customers supertype:BaseEntity", rows[0].Data["markers"])

	assert.Equal(t, unit.KindFunction, rows[1].Data["kind"])
	assert.Equal(t, "Engine", rows[1].Data["owner"], "a method's receiver is not where it lives")
}

// An ecosystem with no unit provider yet renders an empty table. Honest, and
// not a crash.
func TestUnitView_NoUnits(t *testing.T) {
	v := unitView(&core.Results{})
	assert.Empty(t, v.Rows)
	assert.NotEmpty(t, v.Columns)
}

func TestUnitCounts_PerKind(t *testing.T) {
	r := &core.Results{UnitsByKind: map[string][]*unit.Unit{
		unit.KindType:     {{ID: "A"}, {ID: "B"}},
		unit.KindFunction: {{ID: "C"}},
	}}
	got := map[string]interface{}{}
	for _, row := range unitCounts(r) {
		got[row.Data[NameColumn].(string)] = row.Data[ValueColumn]
	}
	assert.Equal(t, map[string]interface{}{"unit_count__type": 2, "unit_count__function": 1}, got)
}
