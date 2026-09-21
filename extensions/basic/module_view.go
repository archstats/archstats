package basic

import (
	"strings"

	"github.com/archstats/archstats/core"
)

// The modules a project declares for itself, read from its own manifests.
//
// A component is where the code says it lives; a module is what the project
// builds and publishes, and the two answer different questions. nopCommerce's
// namespaces do not say which of its parts is a plugin -- its 40 .csproj
// files do, and "core must never depend on a plugin" is only checkable
// against those. Sylius is 60 composer packages whose whole design rests on
// Component not depending on Bundle.
//
// A project that declares nothing yields no rows. That is the common case for
// a single-package repository and is not an error.
func moduleView(results *core.Results) *core.View {
	var rows []*core.Row
	for _, mod := range results.Modules.Modules() {
		// A declared dependency that is also a module of this project is an
		// edge inside the codebase. Everything else is a third-party package,
		// and there are far more of those -- LibreChat's client declares 116
		// dependencies and five of them are its own.
		var internal []string
		for _, dep := range mod.DependsOn {
			if results.Modules.ByName(dep) != nil {
				internal = append(internal, dep)
			}
		}
		rows = append(rows, &core.Row{
			Data: core.RowData{
				"name":                  mod.Name,
				"kind":                  mod.Kind,
				"directory":             mod.Dir,
				"manifest":              mod.Manifest,
				"files":                 len(results.ModuleToFiles[mod.Name]),
				"declared_dependencies": len(mod.DependsOn),
				"internal_dependencies": len(internal),
				"depends_on":            strings.Join(internal, ", "),
			},
		})
	}
	return &core.View{
		Columns: []*core.Column{
			core.StringColumn("name"),
			core.StringColumn("kind"),
			core.StringColumn("directory"),
			core.StringColumn("manifest"),
			core.IntColumn("files"),
			core.IntColumn("declared_dependencies"),
			core.IntColumn("internal_dependencies"),
			core.StringColumn("depends_on"),
		},
		Rows: rows,
	}
}
