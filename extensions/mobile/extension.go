// Package mobile reads what mobile apps declare to the platforms they install
// on -- AndroidManifest.xml, Info.plist, entitlements -- and records it two
// ways: as rows of app_declarations, and as markers on the units those files
// name, so a class the manifest calls a launcher activity says so wherever
// units are read.
package mobile

import (
	"path"
	"sort"
	"strings"
	"sync"

	"github.com/archstats/archstats/core"
	"github.com/archstats/archstats/core/file"
	"github.com/archstats/archstats/core/unit"
)

func Extension() core.Extension {
	return &extension{}
}

type extension struct {
	mu    sync.Mutex
	files map[string][]byte
	rows  []declaration
}

// A declaration is one row of app_declarations.
type declaration struct {
	Module, Platform, Kind, Name, Value, Exported, Unit, File string
	Launcher                                                  bool
	Line                                                      int
}

func (e *extension) Init(a core.Analyzer) error {
	// A desktop app scans many times with one set of extensions.
	e.files, e.rows = map[string][]byte{}, nil
	a.RegisterFileAnalyzer(e)
	a.RegisterResultsEditor(e)
	a.RegisterView(&core.ViewFactory{Name: "app_declarations", CreateViewFunc: e.view})
	return nil
}

// AnalyzeFile keeps the manifests, and the Gradle build files that say which
// package an Android module's manifest is relative to. It records nothing of
// its own.
func (e *extension) AnalyzeFile(f file.File) *file.Results {
	p := strings.TrimPrefix(f.Path(), "./")
	base := strings.ToLower(path.Base(p))
	keep := file.SystemKind(f.Path(), nil) == file.SystemKindAppManifest || base == "build.gradle" || base == "build.gradle.kts"
	if !keep {
		return nil
	}
	content := f.Content()
	cp := make([]byte, len(content))
	copy(cp, content)
	e.mu.Lock()
	e.files[p] = cp
	e.mu.Unlock()
	return nil
}

func (e *extension) EditResults(results *core.Results) {
	byID := make(map[string]*unit.Unit, len(results.Units))
	bySimple := map[string][]*unit.Unit{}
	for _, u := range results.Units {
		byID[u.ID] = u
		if u.Kind == unit.KindType {
			bySimple[u.Name] = append(bySimple[u.Name], u)
		}
	}
	moduleOf := func(p string) string {
		if results.Modules == nil {
			return ""
		}
		return results.Modules.NameOf(p)
	}
	// Which unit a declaration names: its qualified name, or failing that the
	// one type of that simple name in the same module. A manifest class can be
	// declared in Java or Kotlin, and a Swift type is never qualified.
	resolve := func(qualified, module string) *unit.Unit {
		if u := byID[qualified]; u != nil {
			return u
		}
		simple := qualified[strings.LastIndex(qualified, ".")+1:]
		var hit *unit.Unit
		for _, u := range bySimple[simple] {
			if module != "" && u.Module != module {
				continue
			}
			if hit != nil {
				return nil
			}
			hit = u
		}
		return hit
	}

	paths := make([]string, 0, len(e.files))
	for p := range e.files {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	for _, p := range paths {
		content := e.files[p]
		base := strings.ToLower(path.Base(p))
		module := moduleOf(p)
		var entries []manifestEntry
		platform, pkg := "ios", ""
		switch {
		case base == "androidmanifest.xml":
			platform = "android"
			m := readAndroidManifest(content)
			if m == nil {
				continue
			}
			pkg = m.Package
			if pkg == "" {
				pkg = e.namespaceFor(p)
			}
			entries = m.Entries
		case strings.HasSuffix(base, ".entitlements"):
			entries = entitlementEntries(readPlist(content))
		case strings.HasSuffix(base, ".plist"):
			entries = infoPlistEntries(readPlist(content))
		default:
			continue
		}
		for _, en := range entries {
			row := declaration{Module: module, Platform: platform, Kind: en.Kind, Name: en.Name, Value: en.Value,
				Exported: en.Exported, Launcher: en.Launcher, File: p, Line: en.Line}
			var u *unit.Unit
			switch en.Kind {
			case "activity", "service", "receiver", "provider", "application":
				row.Name = className(en.Name, pkg)
				u = resolve(row.Name, module)
			case "deep_link":
				// An Android link names the component that answers it; an
				// iOS universal link names no class.
				if platform == "android" {
					row.Name = className(en.Name, pkg)
					u = resolve(row.Name, module)
				}
			case "scene_delegate":
				// `$(PRODUCT_MODULE_NAME).SceneDelegate`
				u = resolve(en.Name[strings.LastIndex(en.Name, ".")+1:], module)
			}
			if u != nil {
				row.Unit = u.ID
				addMarker(u, en.Kind)
				if en.Launcher {
					addMarker(u, "launcher")
				}
				if en.Exported == "true" || en.Exported == "implied" {
					addMarker(u, "exported")
				}
			}
			e.rows = append(e.rows, row)
		}
	}
	e.files = nil
}

// namespaceFor is the `namespace` (or, in older builds, `applicationId`) of
// the Gradle module a manifest belongs to: the build file in the directory
// above `src/`.
func (e *extension) namespaceFor(manifest string) string {
	dir := path.Dir(manifest)
	for dir != "." && dir != "/" && dir != "" {
		for _, b := range []string{"build.gradle.kts", "build.gradle"} {
			if content, ok := e.files[path.Join(dir, b)]; ok {
				if m := gradleNamespace.FindSubmatch(content); m != nil {
					return string(m[1])
				}
				if m := gradleApplicationID.FindSubmatch(content); m != nil {
					return string(m[1])
				}
				return ""
			}
		}
		dir = path.Dir(dir)
	}
	return ""
}

func addMarker(u *unit.Unit, key string) {
	if u == nil {
		return
	}
	for _, m := range u.Markers {
		if m.Source == unit.SourceManifest && m.Key == key {
			return
		}
	}
	u.Markers = append(u.Markers, unit.Marker{Source: unit.SourceManifest, Key: key})
}

func (e *extension) view(*core.Results) *core.View {
	rows := make([]*core.Row, 0, len(e.rows))
	for _, r := range e.rows {
		launcher := 0
		if r.Launcher {
			launcher = 1
		}
		rows = append(rows, &core.Row{Data: core.RowData{
			"module": r.Module, "platform": r.Platform, "kind": r.Kind, "name": r.Name, "value": r.Value,
			"exported": r.Exported, "launcher": launcher, "unit": r.Unit, "file": walkerName(r.File), "line": r.Line,
		}})
	}
	return &core.View{
		Columns: []*core.Column{
			core.StringColumn("module"),
			core.StringColumn("platform"),
			core.StringColumn("kind"),
			core.StringColumn("name"),
			core.StringColumn("value"),
			core.StringColumn("exported"),
			core.IntColumn("launcher"),
			core.StringColumn("unit"),
			core.StringColumn("file"),
			core.IntColumn("line"),
		},
		Rows: rows,
	}
}

// walkerName restores the walker's spelling of a root-level path, so the
// file column joins files.name.
func walkerName(p string) string {
	if !strings.Contains(p, "/") {
		return "./" + p
	}
	return p
}
