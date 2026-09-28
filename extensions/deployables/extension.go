// Package deployables reads what a workspace builds and ships.
//
// A component is where the code says it lives and a module is what the
// project builds and publishes. A deployable is what gets built and shipped
// as one unit -- a container image, an executable app, a serverless function
// -- and nothing in the source says which code ends up in which. The build
// files, pipelines and deployment descriptors do, and archstats walked them
// and then ignored them.
//
// The engine records deployables and the evidence between them, every fact
// with the file and line it was read from and how it was joined. Whether two
// deployables that call each other synchronously form one architectural
// quantum is a judgement, and the architect makes it as a group.
package deployables

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/archstats/archstats/core"
	"github.com/archstats/archstats/core/file"
)

type extension struct {
	mu    sync.Mutex
	files map[string][]byte
	kinds map[string]string
	model *Model
}

// Extension reads deployables. It costs nothing on a workspace that has no
// build, pipeline or deployment files: every view is empty.
func Extension() core.Extension {
	return &extension{files: map[string][]byte{}, kinds: map[string]string{}}
}

func (e *extension) Init(a core.Analyzer) error {
	// A desktop app scans many times with one set of extensions.
	e.files, e.kinds, e.model = map[string][]byte{}, map[string]string{}, nil
	a.RegisterFileAnalyzer(e)
	a.RegisterResultsEditor(e)
	for _, v := range []*core.ViewFactory{
		{Name: "deployables", CreateViewFunc: e.deployablesView},
		{Name: "deployable_contents", CreateViewFunc: e.contentsView},
		{Name: "deployable_components", CreateViewFunc: e.componentsView},
		{Name: "deployable_links", CreateViewFunc: e.linksView},
		{Name: "deployable_unresolved", CreateViewFunc: e.unresolvedView},
		{Name: "pipelines", CreateViewFunc: e.pipelinesView},
		{Name: "pipeline_deployables", CreateViewFunc: e.pipelineDeployablesView},
		{Name: "deployable_environments", CreateViewFunc: e.environmentsView},
		{Name: "deployable_environment_values", CreateViewFunc: e.envValuesView},
		{Name: "deployable_dependencies", CreateViewFunc: e.dependenciesView},
	} {
		a.RegisterView(v)
	}
	return nil
}

var (
	aspireMarker = []byte("DistributedApplication.CreateBuilder")
	sbomName     = regexp.MustCompile(`(?i)(\.cdx\.json|^bom\.json|^sbom[^/]*\.json|\.bom\.json)$`)
)

// AnalyzeFile keeps the content of the files the linker reads. It records no
// snippets and no stats of its own.
func (e *extension) AnalyzeFile(f file.File) *file.Results {
	p := strings.TrimPrefix(f.Path(), "./")
	content := f.Content()
	kind := file.SystemKind(f.Path(), content)
	if kind == "" {
		base := filepath.Base(p)
		switch {
		case strings.HasSuffix(base, ".cs") && bytes.Contains(content, aspireMarker):
			kind = "aspire"
		case sbomName.MatchString(base) && bytes.Contains(content, []byte("CycloneDX")):
			kind = "sbom"
		}
	}
	if kind == "" {
		return nil
	}
	cp := make([]byte, len(content))
	copy(cp, content)
	e.mu.Lock()
	e.files[p] = cp
	e.kinds[p] = kind
	e.mu.Unlock()
	return nil
}

func (e *extension) EditResults(results *core.Results) {
	clean := func(m map[string]string) map[string]string {
		out := make(map[string]string, len(m))
		for k, v := range m {
			out[strings.TrimPrefix(k, "./")] = v
		}
		return out
	}
	roles := clean(results.FileRoles)
	all := make([]string, 0, len(roles))
	for f := range roles {
		all = append(all, f)
	}
	sort.Strings(all)
	e.model = Build(&Input{
		Files:      e.files,
		Kinds:      e.kinds,
		AllFiles:   all,
		Roles:      roles,
		Components: clean(results.FileToComponent),
		Modules:    results.Modules,
		RepoOf:     repositories(results.RootDirectory),
	})
	// The contents are no longer needed once linked.
	e.files = nil
}

// repositories answers which checkout a path belongs to, for a workspace
// that holds several: the nearest folder above it with a .git, named by its
// path, or the root folder's own name.
func repositories(root string) func(string) string {
	rootName := filepath.Base(root)
	cache := map[string]string{}
	var mu sync.Mutex
	var of func(dir string) string
	of = func(dir string) string {
		if dir == "." || dir == "" || dir == "/" {
			return rootName
		}
		mu.Lock()
		if r, ok := cache[dir]; ok {
			mu.Unlock()
			return r
		}
		mu.Unlock()
		r := ""
		if root != "" {
			if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(dir), ".git")); err == nil {
				r = dir
			}
		}
		if r == "" {
			r = of(dirOf(dir))
		}
		mu.Lock()
		cache[dir] = r
		mu.Unlock()
		return r
	}
	return func(p string) string { return of(dirOf(p)) }
}

// walkerName restores the walker's form of a path: root-level files are
// "./x", so a file column joins files.name.
func walkerName(p string) string {
	if p == "" || strings.Contains(p, "/") || strings.Contains(p, "#") {
		return p
	}
	return "./" + p
}

func (e *extension) m() *Model {
	if e.model == nil {
		return &Model{}
	}
	return e.model
}

func view(columns []*core.Column, rows []*core.Row) *core.View {
	return &core.View{Columns: columns, Rows: rows}
}

func (e *extension) deployablesView(*core.Results) *core.View {
	var rows []*core.Row
	for _, d := range e.m().Deployables {
		rows = append(rows, &core.Row{Data: core.RowData{
			"id": d.ID, "name": d.Name, "kind": d.Kind, "repository": d.Repository,
			"file": walkerName(d.File), "line": d.Line, "built_by": d.BuiltBy, "context": d.Context,
			"base_image": d.BaseImage, "runtime": d.Runtime, "platform": d.Platform, "files": d.Files, "components": d.Components,
		}})
	}
	return view([]*core.Column{
		core.StringColumn("id"), core.StringColumn("name"), core.StringColumn("kind"), core.StringColumn("repository"),
		core.StringColumn("file"), core.IntColumn("line"), core.StringColumn("built_by"), core.StringColumn("context"),
		core.StringColumn("base_image"), core.StringColumn("runtime"), core.StringColumn("platform"), core.IntColumn("files"), core.IntColumn("components"),
	}, rows)
}

func (e *extension) contentsView(*core.Results) *core.View {
	var rows []*core.Row
	for _, c := range e.m().Contents {
		rows = append(rows, &core.Row{Data: core.RowData{
			"deployable": c.Deployable, "path": c.Path, "pattern": c.Pattern, "module": c.Module,
			"file": walkerName(c.File), "line": c.Line, "resolution": c.Resolution,
		}})
	}
	return view([]*core.Column{
		core.StringColumn("deployable"), core.StringColumn("path"), core.StringColumn("pattern"), core.StringColumn("module"),
		core.StringColumn("file"), core.IntColumn("line"), core.StringColumn("resolution"),
	}, rows)
}

func (e *extension) componentsView(*core.Results) *core.View {
	var rows []*core.Row
	for _, c := range e.m().Components {
		rows = append(rows, &core.Row{Data: core.RowData{"deployable": c.Deployable, "component": c.Component, "files": c.Files}})
	}
	return view([]*core.Column{core.StringColumn("deployable"), core.StringColumn("component"), core.IntColumn("files")}, rows)
}

func (e *extension) linksView(*core.Results) *core.View {
	var rows []*core.Row
	for _, l := range e.m().Links {
		rows = append(rows, &core.Row{Data: core.RowData{
			"from": l.From, "to": l.To, "to_kind": l.ToKind, "kind": l.Kind, "mode": l.Mode, "via": l.Via,
			"file": walkerName(l.File), "line": l.Line, "resolution": l.Resolution,
		}})
	}
	return view([]*core.Column{
		core.StringColumn("from"), core.StringColumn("to"), core.StringColumn("to_kind"), core.StringColumn("kind"),
		core.StringColumn("mode"), core.StringColumn("via"), core.StringColumn("file"), core.IntColumn("line"),
		core.StringColumn("resolution"),
	}, rows)
}

func (e *extension) unresolvedView(*core.Results) *core.View {
	var rows []*core.Row
	for _, u := range e.m().Unresolved {
		from := u.From
		// A workload is keyed file#name internally; the name is what a
		// reader recognises.
		if i := strings.LastIndex(from, "#"); i >= 0 {
			from = from[i+1:]
		}
		rows = append(rows, &core.Row{Data: core.RowData{
			"from": from, "ref": u.Ref, "file": walkerName(u.File), "line": u.Line, "reason": u.Reason,
		}})
	}
	return view([]*core.Column{
		core.StringColumn("from"), core.StringColumn("ref"), core.StringColumn("file"), core.IntColumn("line"), core.StringColumn("reason"),
	}, rows)
}

func (e *extension) pipelinesView(*core.Results) *core.View {
	var rows []*core.Row
	for _, p := range e.m().Pipelines {
		rows = append(rows, &core.Row{Data: core.RowData{
			"id": walkerName(p.ID), "name": p.Name, "system": p.System, "file": walkerName(p.File), "repository": p.Repository,
			"parsed": p.Parsed, "triggers": p.Triggers, "paths": p.Paths, "stages": p.Stages, "tools": p.Tools,
			"delegates_to": p.DelegatesTo, "delegates_ref": p.DelegatesRef, "environments": p.Environments,
			"deployables": p.Deployables,
		}})
	}
	return view([]*core.Column{
		core.StringColumn("id"), core.StringColumn("name"), core.StringColumn("system"), core.StringColumn("file"),
		core.StringColumn("repository"), core.StringColumn("parsed"), core.StringColumn("triggers"), core.StringColumn("paths"),
		core.StringColumn("stages"), core.StringColumn("tools"), core.StringColumn("delegates_to"), core.StringColumn("delegates_ref"),
		core.StringColumn("environments"), core.IntColumn("deployables"),
	}, rows)
}

func (e *extension) pipelineDeployablesView(*core.Results) *core.View {
	var rows []*core.Row
	for _, p := range e.m().PipelineDeployables {
		rows = append(rows, &core.Row{Data: core.RowData{
			"pipeline": walkerName(p.Pipeline), "deployable": p.Deployable, "action": p.Action,
			"file": walkerName(p.File), "line": p.Line, "resolution": p.Resolution,
		}})
	}
	return view([]*core.Column{
		core.StringColumn("pipeline"), core.StringColumn("deployable"), core.StringColumn("action"),
		core.StringColumn("file"), core.IntColumn("line"), core.StringColumn("resolution"),
	}, rows)
}

func (e *extension) environmentsView(*core.Results) *core.View {
	var rows []*core.Row
	for _, en := range e.m().Environments {
		rows = append(rows, &core.Row{Data: core.RowData{
			"deployable": en.Deployable, "environment": en.Environment, "kind": en.Kind, "source": en.Source,
			"file": walkerName(en.File), "line": en.Line,
		}})
	}
	return view([]*core.Column{
		core.StringColumn("deployable"), core.StringColumn("environment"), core.StringColumn("kind"),
		core.StringColumn("source"), core.StringColumn("file"), core.IntColumn("line"),
	}, rows)
}

func (e *extension) envValuesView(*core.Results) *core.View {
	var rows []*core.Row
	for _, v := range e.m().EnvValues {
		rows = append(rows, &core.Row{Data: core.RowData{
			"deployable": v.Deployable, "environment": v.Environment, "source": v.Source, "key": v.Key,
			"value": v.Value, "secret": v.Secret, "file": walkerName(v.File), "line": v.Line,
		}})
	}
	return view([]*core.Column{
		core.StringColumn("deployable"), core.StringColumn("environment"), core.StringColumn("source"),
		core.StringColumn("key"), core.StringColumn("value"), core.IntColumn("secret"), core.StringColumn("file"), core.IntColumn("line"),
	}, rows)
}

func (e *extension) dependenciesView(*core.Results) *core.View {
	var rows []*core.Row
	for _, d := range e.m().Dependencies {
		rows = append(rows, &core.Row{Data: core.RowData{
			"deployable": d.Deployable, "ecosystem": d.Ecosystem, "name": d.Name, "version": d.Version,
			"role": d.Role, "source": d.Source, "file": walkerName(d.File), "line": d.Line,
		}})
	}
	return view([]*core.Column{
		core.StringColumn("deployable"), core.StringColumn("ecosystem"), core.StringColumn("name"), core.StringColumn("version"),
		core.StringColumn("role"), core.StringColumn("source"), core.StringColumn("file"), core.IntColumn("line"),
	}, rows)
}
