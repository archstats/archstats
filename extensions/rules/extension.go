package rules

import (
	"embed"
	"io/fs"
	"path"
	"strings"

	"github.com/archstats/archstats/core"
	"github.com/archstats/archstats/core/component"
	"github.com/archstats/archstats/core/definitions"
	"gopkg.in/yaml.v3"
)

//go:embed definitions/**
var ruleDefs embed.FS

func Extension() core.Extension {
	return &extension{}
}

type extension struct {
	rules []*compiledRule
}

func (e *extension) Init(settings core.Analyzer) error {
	loaded, err := loadRules(ruleDefs)
	if err != nil {
		return err
	}
	e.rules = loaded

	// A rule is vocabulary the user reads, so it is documented the same way
	// a metric is and reaches docs/metrics.md through the same path.
	for _, r := range loaded {
		settings.AddDefinition(&definitions.Definition{
			Id:               r.rule.Id,
			Name:             r.rule.Name,
			ShortDescription: r.rule.ShortDescription,
			LongDescription:  r.rule.LongDescription,
			Category:         r.rule.Category,
		})
	}

	settings.RegisterView(&core.ViewFactory{
		Name:           "rules",
		CreateViewFunc: e.view,
	})
	return nil
}

func loadRules(fileSystem fs.FS) ([]*compiledRule, error) {
	var out []*compiledRule
	entries, err := fs.ReadDir(fileSystem, "definitions")
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yaml") {
			continue
		}
		raw, err := fs.ReadFile(fileSystem, path.Join("definitions", entry.Name()))
		if err != nil {
			return nil, err
		}
		rule := &Rule{}
		if err := yaml.Unmarshal(raw, rule); err != nil {
			return nil, err
		}
		compiled, err := compile(rule)
		if err != nil {
			return nil, err
		}
		out = append(out, compiled)
	}
	return out, nil
}

// Every rule's verdict, and the edges that broke it.
//
// A project that declares no modules produces no rows at all -- not a pass
// and not a failure. A rule about modules cannot be checked where there are
// none, and saying so beats a green tick nobody earned.
func (e *extension) view(results *core.Results) *core.View {
	present := make([]NamedDir, 0, results.Modules.Len())
	for _, m := range results.Modules.Modules() {
		present = append(present, NamedDir{Name: m.Name, Dir: m.Dir})
	}

	var rows []*core.Row
	for _, f := range e.check(results, present) {
		rows = append(rows, &core.Row{
			Data: map[string]interface{}{
				"rule":   f.Rule,
				"status": f.Status,
				"from":   f.From,
				"to":     f.To,
				"kind":   f.Kind,
				"file":   f.File,
				"line":   f.Line,
			},
		})
	}
	return &core.View{
		Name: "rules",
		Columns: []*core.Column{
			core.StringColumn("rule"),
			core.StringColumn("status"),
			core.StringColumn("from"),
			core.StringColumn("to"),
			core.StringColumn("kind"),
			core.StringColumn("file"),
			core.IntColumn("line"),
		},
		Rows: rows,
	}
}

func (e *extension) check(results *core.Results, present []NamedDir) []*Finding {
	var out []*Finding
	if len(present) == 0 {
		// Rules are statements about the modules a project declares. One
		// that declares none earns neither a pass nor a failure, and the
		// view says so by returning nothing at all rather than a row per
		// rule claiming it held.
		return out
	}
	dirOf := make(map[string]string, len(present))
	for _, m := range present {
		dirOf[m.Name] = m.Dir
	}

	// Which module owns each component: a component's files all sit inside
	// one module in every layout seen, and the first answers for the rest.
	componentToModule := make(map[string]string, len(results.ComponentToFiles))
	for comp, files := range results.ComponentToFiles {
		for _, f := range files {
			if mod, ok := results.FileToModule[f]; ok {
				componentToModule[comp] = mod
				break
			}
		}
	}

	for _, rule := range e.rules {
		if !rule.when.any(present) {
			out = append(out, &Finding{Rule: rule.rule.Id, Status: StatusNotApplicable})
			continue
		}
		before := len(out)
		// Type-only edges are checked too: depending on a bundle for its
		// types is still depending on it, and a rule is about what the code
		// is allowed to know, not only about what it links against.
		for _, conn := range results.AllConnections {
			fromModule := results.FileToModule[conn.File]
			toModule := componentToModule[conn.To]
			if fromModule == "" || toModule == "" || fromModule == toModule {
				continue
			}
			if !rule.from.selects(fromModule, dirOf[fromModule]) || !rule.to.selects(toModule, dirOf[toModule]) {
				continue
			}
			line := 0
			if conn.Begin != nil {
				line = conn.Begin.Line
			}
			out = append(out, &Finding{
				Rule:   rule.rule.Id,
				Status: StatusViolation,
				From:   fromModule,
				To:     toModule,
				Kind:   conn.Kind(),
				File:   conn.File,
				Line:   line,
			})
		}
		// A module dependency declared in a manifest with no import anywhere
		// is still a dependency, and in .NET it is the usual way a plugin
		// reaches core.
		for _, mod := range results.Modules.Modules() {
			if !rule.from.selects(mod.Name, mod.Dir) {
				continue
			}
			for _, dep := range mod.DependsOn {
				depMod := results.Modules.ByName(dep)
				if depMod == nil || !rule.to.selects(dep, depMod.Dir) {
					continue
				}
				out = append(out, &Finding{
					Rule:   rule.rule.Id,
					Status: StatusViolation,
					From:   mod.Name,
					To:     dep,
					Kind:   component.KindManifest,
					File:   mod.Manifest,
				})
			}
		}
		if len(out) == before {
			out = append(out, &Finding{Rule: rule.rule.Id, Status: StatusOk})
		}
	}
	return out
}
