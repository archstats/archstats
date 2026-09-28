package module

import (
	"path/filepath"
	"sort"

	"gopkg.in/yaml.v3"
)

// ---------------------------------------------------------------------------
// pub: Dart and Flutter
//
// A pubspec names its package, and `import 'package:wonders/logic/x.dart'`
// uses that name, so it is how Dart imports resolve to directories. It also
// lists every dependency, which is a Flutter app's stack: its state
// management (riverpod, bloc, provider, get), its HTTP client (dio, http),
// its storage (drift, isar, hive, sqflite).
// ---------------------------------------------------------------------------

type pubReader struct{}

func (pubReader) Kind() string         { return "pub" }
func (pubReader) Claims(b string) bool { return b == "pubspec.yaml" }

type pubspec struct {
	Name            string               `yaml:"name"`
	Dependencies    map[string]yaml.Node `yaml:"dependencies"`
	DevDependencies map[string]yaml.Node `yaml:"dev_dependencies"`
	Flutter         *yaml.Node           `yaml:"flutter"`
}

func (pubReader) Read(root, manifest string) []*Module {
	raw, ok := readFile(manifest)
	if !ok {
		return nil
	}
	var p pubspec
	if err := yaml.Unmarshal([]byte(raw), &p); err != nil || p.Name == "" {
		return nil
	}
	var deps []string
	for _, group := range []map[string]yaml.Node{p.Dependencies, p.DevDependencies} {
		names := make([]string, 0, len(group))
		for name := range group {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			deps = appendUnique(deps, name)
		}
	}
	_, flutter := p.Dependencies["flutter"]
	typ := "dart-package"
	if flutter {
		typ = "flutter-package"
		if _, ok := readFile(filepath.Join(filepath.Dir(manifest), "lib", "main.dart")); ok {
			typ = "flutter-app"
		}
	}
	return []*Module{{
		Name:      p.Name,
		Dir:       relDir(root, manifest),
		Manifest:  relFile(root, manifest),
		DependsOn: deps,
		Type:      typ,
	}}
}
