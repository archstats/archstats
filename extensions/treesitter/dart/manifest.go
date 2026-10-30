package dart

import (
	"path/filepath"
	"sort"

	"github.com/archstats/archstats/core/module"
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

func (pubReader) Read(root, manifest string) []*module.Module {
	raw, ok := module.ReadFile(manifest)
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
			deps = module.AppendUnique(deps, name)
		}
	}
	_, flutter := p.Dependencies["flutter"]
	typ := "dart-package"
	if flutter {
		typ = "flutter-package"
		if _, ok := module.ReadFile(filepath.Join(filepath.Dir(manifest), "lib", "main.dart")); ok {
			typ = "flutter-app"
		}
	}
	return []*module.Module{{
		Name:      p.Name,
		Dir:       module.RelDir(root, manifest),
		Manifest:  module.RelFile(root, manifest),
		DependsOn: deps,
		Type:      typ,
	}}
}

// ManifestReaders reads pubspec.yaml.
func ManifestReaders() []module.Reader { return []module.Reader{pubReader{}} }
