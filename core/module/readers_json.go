package module

import (
	"encoding/json"
	"path/filepath"
	"regexp"
	"strings"
)

// ---------------------------------------------------------------------------
// composer: PHP
//
// Sylius is 60 composer packages under one repository, and the split its
// whole design rests on -- `Sylius\Component\*` knowing nothing about
// `Sylius\Bundle\*` -- is a fact about those packages, not about namespaces.
// ---------------------------------------------------------------------------

type composerReader struct{}

func (composerReader) Kind() string          { return "composer" }
func (composerReader) Claims(b string) bool  { return b == "composer.json" }

type composerManifest struct {
	Name    string            `json:"name"`
	Require map[string]string `json:"require"`
	Autoload struct {
		PSR4 map[string]interface{} `json:"psr-4"`
	} `json:"autoload"`
}

func (composerReader) Read(root, path string) []*Module {
	raw, ok := readFile(path)
	if !ok {
		return nil
	}
	var man composerManifest
	if json.Unmarshal([]byte(raw), &man) != nil || man.Name == "" {
		return nil
	}
	return []*Module{{
		Name:      man.Name,
		Dir:       relDir(root, path),
		Manifest:  relFile(root, path),
		DependsOn: sortedKeys(man.Require),
	}}
}

// ---------------------------------------------------------------------------
// node: JavaScript and TypeScript
//
// LibreChat declares `api`, `client` and `packages/*` as workspaces. Those
// are the boundaries an architect asks about, and nothing in any source file
// mentions them.
// ---------------------------------------------------------------------------

type nodeReader struct{}

func (nodeReader) Kind() string         { return "node" }
func (nodeReader) Claims(b string) bool { return b == "package.json" }

type nodeManifest struct {
	Name         string            `json:"name"`
	Dependencies map[string]string `json:"dependencies"`
	Dev          map[string]string `json:"devDependencies"`
}

func (nodeReader) Read(root, path string) []*Module {
	raw, ok := readFile(path)
	if !ok {
		return nil
	}
	var man nodeManifest
	if json.Unmarshal([]byte(raw), &man) != nil || man.Name == "" {
		return nil
	}
	return []*Module{{
		Name:      man.Name,
		Dir:       relDir(root, path),
		Manifest:  relFile(root, path),
		DependsOn: append(sortedKeys(man.Dependencies), sortedKeys(man.Dev)...),
	}}
}

// ---------------------------------------------------------------------------
// go
//
// One module per go.mod. Go's components are directories -- `package core`
// says nothing about where it sits -- so the value here is the module path,
// which every absolute import carries as a prefix the file tree has never
// heard of, and the `internal/` boundaries the compiler enforces.
// ---------------------------------------------------------------------------

type goReader struct{}

func (goReader) Kind() string         { return "go" }
func (goReader) Claims(b string) bool { return b == "go.mod" }

var (
	goModulePath = regexp.MustCompile(`(?m)^\s*module\s+(\S+)`)
	goRequire    = regexp.MustCompile(`(?m)^\s*(?:require\s+)?([a-z0-9._~-]+\.[a-z]{2,}/\S+)\s+v\S+`)
)

func (goReader) Read(root, path string) []*Module {
	raw, ok := readFile(path)
	if !ok {
		return nil
	}
	name := goModulePath.FindStringSubmatch(raw)
	if name == nil {
		return nil
	}
	var deps []string
	for _, m := range goRequire.FindAllStringSubmatch(raw, -1) {
		deps = appendUnique(deps, m[1])
	}
	return []*Module{{
		Name:      name[1],
		Dir:       relDir(root, path),
		Manifest:  relFile(root, path),
		DependsOn: deps,
	}}
}

// ---------------------------------------------------------------------------
// django: Python
//
// Django names no module in any manifest a parser can rely on -- the list
// lives in a settings file as `INSTALLED_APPS`, and django-oscar builds its
// own at `src/oscar/__init__.py`. What is reliable is `apps.py`: every app
// has one, django-oscar has 29, and the directory holding it is the app.
//
// The role in Django is in the filename, not in any annotation: `models.py`,
// `views.py`, `forms.py`, `admin.py`. That is read elsewhere, as a marker;
// here `apps.py` only marks the boundary.
// ---------------------------------------------------------------------------

type djangoReader struct{}

func (djangoReader) Kind() string         { return "django" }
func (djangoReader) Claims(b string) bool { return b == "apps.py" }

func (djangoReader) Read(root, path string) []*Module {
	dir := relDir(root, path)
	if dir == "" {
		return nil
	}
	// The app is named the way Python would import it: the dotted path from
	// the nearest directory that is not itself a package. `src/oscar/order`
	// is the app `oscar.order`, because `src` holds no __init__.py.
	segments := strings.Split(dir, "/")
	start := 0
	for i := range segments {
		probe := filepath.Join(root, filepath.Join(segments[:i+1]...), "__init__.py")
		if _, ok := readFile(probe); ok {
			start = i
			break
		}
	}
	name := strings.Join(segments[start:], ".")
	if name == "" {
		name = segments[len(segments)-1]
	}
	return []*Module{{
		Name:     name,
		Dir:      dir,
		Manifest: relFile(root, path),
	}}
}

func sortedKeys(m map[string]string) []string {
	if len(m) == 0 {
		return nil
	}
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func appendUnique(list []string, v string) []string {
	for _, existing := range list {
		if existing == v {
			return list
		}
	}
	return append(list, v)
}
