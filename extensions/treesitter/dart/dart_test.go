package dart

import (
	"os"
	"path"
	"path/filepath"
	"testing"

	"github.com/archstats/archstats/core/file"
	"github.com/archstats/archstats/core/unit"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var wonders = map[string]string{
	"pubspec.yaml": "name: wonders\ndependencies:\n  flutter:\n    sdk: flutter\n",
	"lib/main.dart": `import 'package:flutter/material.dart';
import 'package:wonders/ui/screens/home_screen.dart';
void main() => runApp(HomeScreen());
`,
	"lib/ui/screens/home_screen.dart": `import 'package:flutter/material.dart';
import 'package:wonders/logic/common.dart';
import '../widgets/card.dart';
part 'home_screen.g.dart';

@immutable
class HomeScreen extends StatefulWidget with Themed implements Screen {
  @override
  State<HomeScreen> createState() => _HomeScreenState();
}

class _HomeScreenState extends State<HomeScreen> {
  Widget build(BuildContext c) { AppLogic.instance.run(); return WonderCard(); }
}
`,
	"lib/ui/screens/home_screen.g.dart": `part of 'home_screen.dart';
mixin Themed {}
abstract interface class Screen {}
`,
	"lib/logic/common.dart": `export 'app_logic.dart';
`,
	"lib/logic/app_logic.dart": `@riverpod
Future<int> counter(Ref ref) async => 1;
class AppLogic { static final instance = AppLogic(); void run() {} }
`,
	"lib/ui/widgets/card.dart": `import 'package:flutter/material.dart';
class WonderCard extends StatelessWidget {}
`,
}

func analyse(t *testing.T) (map[string]*file.Results, map[string]*unit.Unit) {
	t.Helper()
	root := t.TempDir()
	a := &analyzer{lp: createPack()}
	var all []*file.Results
	byName := map[string]*file.Results{}
	for p, src := range wonders {
		full := filepath.Join(root, p)
		require.NoError(t, os.MkdirAll(filepath.Dir(full), 0o755))
		require.NoError(t, os.WriteFile(full, []byte(src), 0o644))
		var res *file.Results
		if filepath.Ext(p) == ".dart" {
			res = a.analyze(p, []byte(src))
			require.NotNil(t, res, p)
		} else {
			res = &file.Results{}
		}
		res.Name = p
		res.Directory = path.Dir(p) // slash paths, as the analyzer sets them
		for _, s := range res.Snippets {
			s.Component = res.Directory
		}
		all = append(all, res)
		byName[p] = res
	}
	(&linker{root: root}).EditFileResults(all)
	units := map[string]*unit.Unit{}
	for _, fr := range all {
		for _, u := range fr.Units {
			units[u.ID] = u
		}
	}
	return byName, units
}

func TestDartUnits(t *testing.T) {
	_, units := analyse(t)
	home := units["lib/ui/screens/home_screen#HomeScreen"]
	require.NotNil(t, home)
	for _, m := range []string{"StatefulWidget", "Themed", "Screen", "immutable", "class"} {
		assert.Truef(t, home.HasMarker(m), "HomeScreen should carry %s", m)
	}
	assert.True(t, units["lib/ui/screens/home_screen.g#Screen"].HasMarker("interface"), "an abstract interface class is an interface")
	assert.True(t, units["lib/ui/screens/home_screen.g#Themed"].HasMarker("mixin"))
	assert.True(t, units["lib/logic/app_logic#counter"].HasMarker("riverpod"), "a top-level annotation marks the declaration after it")
	assert.Equal(t, "lib/ui/screens/home_screen#_HomeScreenState", units["lib/ui/screens/home_screen#_HomeScreenState.build"].Owner)
}

func TestDartReferencesFollowImportsExportsAndParts(t *testing.T) {
	_, units := analyse(t)
	refs := map[string]bool{}
	for _, r := range units["lib/ui/screens/home_screen#_HomeScreenState.build"].Refs {
		refs[r.Module+"#"+r.Name] = true
	}
	assert.True(t, refs["lib/logic/app_logic#AppLogic"], "through the barrel's export")
	assert.True(t, refs["lib/ui/widgets/card#WonderCard"], "a relative import")
	assert.False(t, refs["lib/logic/app_logic#BuildContext"])
	homeRefs := map[string]bool{}
	for _, r := range units["lib/ui/screens/home_screen#HomeScreen"].Refs {
		homeRefs[r.Module+"#"+r.Name] = true
	}
	assert.True(t, homeRefs["lib/ui/screens/home_screen.g#Themed"], "a part's declarations are the library's")
}

func TestDartEdgesAreImports(t *testing.T) {
	files, _ := analyse(t)
	var edges []string
	for _, s := range files["lib/ui/screens/home_screen.dart"].Snippets {
		if s.Type == file.ComponentImport {
			edges = append(edges, s.Value)
		}
	}
	assert.ElementsMatch(t, []string{"lib/logic", "lib/ui/widgets"}, edges, "package: resolves through the pubspec name; flutter is a dependency")
	var raw []string
	for _, s := range files["lib/main.dart"].Snippets {
		if s.Type == file.ImportRaw {
			raw = append(raw, s.Value)
		}
	}
	assert.Contains(t, raw, "flutter/material.dart")
}
