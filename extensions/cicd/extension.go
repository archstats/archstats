// Package cicd reads how a workspace builds and ships: its pipelines, read
// the way they run (what starts them, their jobs and what each waits for,
// the steps inside, what they hand work to, what they produce), the images
// and other artifacts it builds and the code inside them, and the hygiene
// facts a reviewer would flag.
//
// It reads GitHub Actions, GitLab CI, Jenkins (declarative, scripted and
// shared libraries) and Rancher (CLI deploys, Fleet bundles and Rancher
// Pipelines) in full, and Azure Pipelines, Bitbucket Pipelines and CircleCI
// into the same model. Everything is read offline from the checkout: no
// pipeline is run, no include fetched, no registry asked. Every fact carries
// the file and line it was read at, and what a reader could not follow is a
// row of its own, never a silent gap.
package cicd

import (
	"bufio"
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
	a.RegisterConventions(Conventions())
	a.RegisterFileAnalyzer(e)
	a.RegisterResultsEditor(e)
	for _, v := range []*core.ViewFactory{
		{Name: "pipelines", CreateViewFunc: e.pipelinesView},
		{Name: "pipeline_unread", CreateViewFunc: e.unreadView},
		{Name: "pipeline_triggers", CreateViewFunc: e.triggersView},
		{Name: "pipeline_jobs", CreateViewFunc: e.jobsView},
		{Name: "pipeline_steps", CreateViewFunc: e.stepsView},
		{Name: "pipeline_handoffs", CreateViewFunc: e.handoffsView},
		{Name: "pipeline_outputs", CreateViewFunc: e.outputsView},
		{Name: "artifacts", CreateViewFunc: e.artifactsView},
		{Name: "artifact_contents", CreateViewFunc: e.contentsView},
		{Name: "artifact_components", CreateViewFunc: e.componentsView},
		{Name: "dockerfiles", CreateViewFunc: e.dockerfilesView},
		{Name: "dockerfile_stages", CreateViewFunc: e.stagesView},
		{Name: "cicd_flags", CreateViewFunc: e.flagsView},
		{Name: "cicd_unresolved", CreateViewFunc: e.unresolvedView},
	} {
		a.RegisterView(v)
	}
	return nil
}

var (
	// A Jenkins shared library's global step: vars/name.groovy.
	jenkinsVar = regexp.MustCompile(`(^|/)vars/[A-Za-z0-9_]+\.groovy$`)
	// Files a pipeline may include or load: YAML and Groovy.
	includable   = regexp.MustCompile(`\.(ya?ml|groovy|gvy|jenkinsfile)$`)
	aspireMarker = []byte("DistributedApplication.CreateBuilder")
	sbomName     = regexp.MustCompile(`(?i)(\.cdx\.json|^bom\.json|^sbom[^/]*\.json|\.bom\.json)$`)
)

// AnalyzeFile keeps the content of the files the linker reads. It records no
// snippets and no stats of its own.
func (e *extension) AnalyzeFile(f file.File) *file.Results {
	p := strings.TrimPrefix(f.Path(), "./")
	content := f.Content()
	kind, keep := KindOf(f.Path(), content)
	if !keep {
		return nil
	}
	cp := make([]byte, len(content))
	copy(cp, content)
	e.mu.Lock()
	e.files[p] = cp
	if kind != "" {
		e.kinds[p] = kind
	}
	e.mu.Unlock()
	return nil
}

// KindOf says what a file is to this extension, and whether to keep its
// content: system files by their kind; Jenkins library steps, Fleet bundles,
// app hosts and SBOMs by their own; YAML and Groovy a pipeline may include
// or load with no kind at all.
func KindOf(path string, content []byte) (kind string, keep bool) {
	p := strings.TrimPrefix(path, "./")
	kind = SystemKind(path, content)
	base := strings.ToLower(filepath.Base(p))
	switch {
	case jenkinsVar.MatchString(p):
		return "ci_library", true
	case base == "fleet.yaml" || base == "fleet.yml":
		return "fleet", true
	case base == ".rancher-pipeline.yml" || base == ".rancher-pipeline.yaml":
		return "ci", true
	}
	if kind == "" {
		switch {
		case strings.HasSuffix(base, ".cs") && bytes.Contains(content, aspireMarker):
			kind = "aspire"
		case sbomName.MatchString(base) && bytes.Contains(content, []byte("CycloneDX")):
			kind = "sbom"
		}
	}
	if kind == "" {
		return "", len(content) < 512*1024 && includable.MatchString(strings.ToLower(p))
	}
	return kind, true
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
	repoOf := repositories(results.RootDirectory)
	e.model = Build(&Input{
		Files:      e.files,
		Kinds:      e.kinds,
		AllFiles:   all,
		Roles:      roles,
		Components: clean(results.FileToComponent),
		Modules:    results.Modules,
		RepoOf:     repoOf,
		Remotes:    Remotes(results.RootDirectory, all, repoOf),
	})
	// The contents are no longer needed once linked.
	e.files = nil
}

// Remotes reads each repository's origin, reduced to its path
// (group/sub/name), so a reference to another project can be resolved to
// the folder the workspace holds it in.
func Remotes(root string, files []string, repoOf func(string) string) map[string]string {
	out := map[string]string{}
	if root == "" {
		return out
	}
	seen := map[string]bool{}
	for _, f := range files {
		dir := repoOf(f)
		key, path := ".", filepath.Join(root, ".git", "config")
		if strings.HasPrefix(f, dir+"/") {
			key, path = dir, filepath.Join(root, filepath.FromSlash(dir), ".git", "config")
		}
		if seen[key] {
			continue
		}
		seen[key] = true
		if u := originURL(path); u != "" {
			out[key] = repoPathOfURL(u)
		}
	}
	return out
}

// originURL reads the url of the origin remote from a git config file.
func originURL(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	inOrigin := false
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if strings.HasPrefix(line, "[") {
			inOrigin = line == `[remote "origin"]`
			continue
		}
		if inOrigin {
			if k, v, ok := strings.Cut(line, "="); ok && strings.TrimSpace(k) == "url" {
				return strings.TrimSpace(v)
			}
		}
	}
	return ""
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
// walkerNames applies walkerName to each id in a comma list.
func walkerNames(list string) string {
	if list == "" {
		return ""
	}
	parts := strings.Split(list, ", ")
	for i, p := range parts {
		parts[i] = walkerName(p)
	}
	return strings.Join(parts, ", ")
}

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
