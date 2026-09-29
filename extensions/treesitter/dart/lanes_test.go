package dart

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/archstats/archstats/core/file"
	"github.com/archstats/archstats/core/unit"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A Flutter app mixing the state managers real apps mix -- Bloc for the
// timeline, Riverpod for settings (with and without codegen), GetX in one
// corner -- with a Freezed model, a Dio client, a repository, and the part
// files Bloc and Freezed spread a library over. The mini-app the UI's Flutter
// lanes are tested against (archstats-ui:
// features/frameworks/layers.mobile.test.ts).
var flutterApp = map[string]string{
	"pubspec.yaml": "name: wonders\ndependencies:\n  flutter:\n    sdk: flutter\n  flutter_bloc: ^8.0.0\n  flutter_riverpod: ^2.0.0\n  get: ^4.0.0\n  dio: ^5.0.0\n",
	"lib/main.dart": `import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:wonders/ui/screens/home_screen.dart';

void main() => runApp(const ProviderScope(child: WondersApp()));

class WondersApp extends StatelessWidget {
  const WondersApp({super.key});
  @override
  Widget build(BuildContext context) => MaterialApp(home: const HomeScreen());
}
`,
	"lib/ui/screens/home_screen.dart": `import 'package:flutter/material.dart';
import 'package:flutter_bloc/flutter_bloc.dart';
import 'package:wonders/logic/wonders_bloc.dart';
import 'package:wonders/ui/widgets/wonder_card.dart';

class HomeScreen extends StatefulWidget {
  const HomeScreen({super.key});
  @override
  State<HomeScreen> createState() => _HomeScreenState();
}

class _HomeScreenState extends State<HomeScreen> {
  @override
  void initState() { super.initState(); context.read<WondersBloc>().add(LoadWonders()); }
  @override
  Widget build(BuildContext context) {
    return BlocBuilder<WondersBloc, WondersState>(
      builder: (context, state) => ListView(children: [for (final w in state.wonders) WonderCard(wonder: w)]),
    );
  }
}
`,
	"lib/ui/screens/settings_page.dart": `import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:wonders/logic/settings_provider.dart';

class SettingsPage extends ConsumerWidget {
  const SettingsPage({super.key});
  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final dark = ref.watch(settingsProvider);
    return SwitchListTile(value: dark, onChanged: (_) => ref.read(settingsProvider.notifier).toggle());
  }
}
`,
	"lib/ui/screens/home_view.dart": `import 'package:flutter/material.dart';
import 'package:get/get.dart';
import 'package:wonders/logic/home_controller.dart';

class HomeView extends GetView<HomeController> {
  @override
  Widget build(BuildContext context) => Obx(() => Text('${controller.count.value}'));
}
`,
	"lib/ui/widgets/wonder_card.dart": `import 'package:flutter/material.dart';
import 'package:wonders/models/wonder.dart';

class WonderCard extends StatelessWidget {
  const WonderCard({super.key, required this.wonder});
  final Wonder wonder;
  @override
  Widget build(BuildContext context) => Card(child: Text(wonder.title));
}
`,
	"lib/logic/wonders_bloc.dart": `import 'package:bloc/bloc.dart';
import 'package:equatable/equatable.dart';
import 'package:wonders/data/wonders_repository.dart';
import 'package:wonders/models/wonder.dart';

part 'wonders_event.dart';
part 'wonders_state.dart';

class WondersBloc extends Bloc<WondersEvent, WondersState> {
  WondersBloc(this.repository) : super(const WondersState()) {
    on<LoadWonders>(_onLoad);
  }
  final WondersRepository repository;
  Future<void> _onLoad(LoadWonders event, Emitter<WondersState> emit) async {
    emit(WondersState(wonders: await repository.all()));
  }
}
`,
	"lib/logic/wonders_event.dart": `part of 'wonders_bloc.dart';

sealed class WondersEvent extends Equatable {
  const WondersEvent();
  @override
  List<Object> get props => [];
}

final class LoadWonders extends WondersEvent {}
`,
	"lib/logic/wonders_state.dart": `part of 'wonders_bloc.dart';

class WondersState extends Equatable {
  const WondersState({this.wonders = const []});
  final List<Wonder> wonders;
  @override
  List<Object> get props => [wonders];
}
`,
	"lib/logic/settings_provider.dart": `import 'package:riverpod_annotation/riverpod_annotation.dart';
import 'package:wonders/data/settings_store.dart';

part 'settings_provider.g.dart';

@riverpod
class Settings extends _$Settings {
  @override
  bool build() => ref.watch(settingsStoreProvider).darkMode;
  void toggle() => state = !state;
}

@Riverpod(keepAlive: true)
SettingsStore settingsStore(SettingsStoreRef ref) => SettingsStore();
`,
	"lib/logic/counter_provider.dart": `import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:wonders/data/api_client.dart';
import 'package:wonders/data/wonders_repository.dart';

final counterProvider = StateNotifierProvider<CounterNotifier, int>((ref) => CounterNotifier());

final wondersRepositoryProvider = Provider.autoDispose((ref) => WondersRepository(ApiClient()));

class CounterNotifier extends StateNotifier<int> {
  CounterNotifier() : super(0);
  void increment() => state++;
}
`,
	"lib/logic/home_controller.dart": `import 'package:get/get.dart';

class HomeController extends GetxController {
  final count = 0.obs;
  void increment() => count.value++;
}
`,
	"lib/data/wonders_repository.dart": `import 'package:wonders/data/api_client.dart';
import 'package:wonders/models/wonder.dart';
import 'package:wonders/ui/screens/home_screen.dart';

class WondersRepository {
  WondersRepository(this.client);
  final ApiClient client;
  Future<List<Wonder>> all() async => (await client.get('/wonders')).map(Wonder.fromJson).toList();
  // Planted: a repository that knows a screen.
  Widget route() => const HomeScreen();
}
`,
	"lib/data/api_client.dart": `import 'package:dio/dio.dart';

class ApiClient {
  final Dio _dio = Dio();
  Future<List<Map<String, dynamic>>> get(String path) async => (await _dio.get(path)).data;
}
`,
	"lib/data/settings_store.dart": `import 'package:shared_preferences/shared_preferences.dart';

class SettingsStore {
  Future<bool> darkMode() async => (await SharedPreferences.getInstance()).getBool('dark') ?? false;
}
`,
	"lib/models/wonder.dart": `import 'package:freezed_annotation/freezed_annotation.dart';

part 'wonder.freezed.dart';
part 'wonder.g.dart';

@freezed
class Wonder with _$Wonder {
  const factory Wonder({required String id, required String title}) = _Wonder;
  factory Wonder.fromJson(Map<String, dynamic> json) => _$WonderFromJson(json);
}
`,
	"lib/models/wonder.freezed.dart": `part of 'wonder.dart';

mixin _$Wonder {
  String get id;
  String get title;
}

class _Wonder implements Wonder {
  const _Wonder({required this.id, required this.title});
  @override
  final String id;
  @override
  final String title;
}
`,
	// Planted: a model that reaches back up to a bloc.
	"lib/models/wonder_draft.dart": `import 'package:wonders/logic/wonders_bloc.dart';

class WonderDraft {
  WonderDraft(this.bloc);
  final WondersBloc bloc;
  String title = '';
}
`,
	"test/wonders_bloc_test.dart": `import 'package:flutter_test/flutter_test.dart';
import 'package:wonders/logic/wonders_bloc.dart';
import 'package:wonders/data/wonders_repository.dart';
import 'package:wonders/data/api_client.dart';

void main() {
  test('loads wonders', () async {
    final bloc = WondersBloc(WondersRepository(ApiClient()));
    expect(bloc.state.wonders, isEmpty);
  });
}
`,
}

func analyseApp(t *testing.T, files map[string]string) (map[string]*file.Results, map[string]*unit.Unit) {
	t.Helper()
	root := t.TempDir()
	a := &analyzer{lp: createPack()}
	var all []*file.Results
	byName := map[string]*file.Results{}
	for p, src := range files {
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
		res.Directory = filepath.Dir(p)
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

func markers(u *unit.Unit, source string) []string {
	var out []string
	for _, m := range u.Markers {
		if m.Source == source {
			out = append(out, m.Key)
		}
	}
	sort.Strings(out)
	return out
}

// rolledUp is what the UI reads for a unit: its own references and its
// members', since members are not listed apart from their owner.
func rolledUp(units map[string]*unit.Unit, id string) []string {
	seen := map[string]bool{}
	var out []string
	add := func(u *unit.Unit) {
		for _, r := range u.Refs {
			key := r.Module + "#" + r.Name
			if !seen[key] && key != id {
				seen[key] = true
				out = append(out, key)
			}
		}
	}
	add(units[id])
	for _, u := range units {
		if u.Owner == id {
			add(u)
		}
	}
	sort.Strings(out)
	return out
}

func TestFlutterUnitsAndTheirMarkers(t *testing.T) {
	files, units := analyseApp(t, flutterApp)

	home := units["lib/ui/screens/home_screen#HomeScreen"]
	require.NotNil(t, home)
	assert.Equal(t, unit.KindType, home.Kind)
	assert.Equal(t, []string{"class"}, markers(home, sourceKeyword))
	assert.Equal(t, []string{"StatefulWidget"}, markers(home, unit.SourceSupertype))
	state := units["lib/ui/screens/home_screen#_HomeScreenState"]
	require.NotNil(t, state, "a widget's State class is a unit of its own, and where its code is")
	assert.Equal(t, []string{"State"}, markers(state, unit.SourceSupertype), "the type argument is not part of the key")
	assert.Equal(t, "lib/ui/screens/home_screen#_HomeScreenState", units["lib/ui/screens/home_screen#_HomeScreenState.build"].Owner)

	assert.Equal(t, []string{"ConsumerWidget"}, markers(units["lib/ui/screens/settings_page#SettingsPage"], unit.SourceSupertype))
	assert.Equal(t, []string{"GetView"}, markers(units["lib/ui/screens/home_view#HomeView"], unit.SourceSupertype))
	assert.Equal(t, []string{"StatelessWidget"}, markers(units["lib/ui/widgets/wonder_card#WonderCard"], unit.SourceSupertype))
	assert.Equal(t, []string{"StatelessWidget"}, markers(units["lib/main#WondersApp"], unit.SourceSupertype))
	assert.Equal(t, unit.KindFunction, units["lib/main#main"].Kind)

	bloc := units["lib/logic/wonders_bloc#WondersBloc"]
	assert.Equal(t, []string{"Bloc"}, markers(bloc, unit.SourceSupertype))
	// A part's declarations are units of the part file.
	event := units["lib/logic/wonders_event#WondersEvent"]
	require.NotNil(t, event)
	assert.Equal(t, []string{"class", "sealed"}, markers(event, sourceKeyword))
	assert.Equal(t, []string{"Equatable"}, markers(event, unit.SourceSupertype))
	assert.Equal(t, []string{"class", "final"}, markers(units["lib/logic/wonders_event#LoadWonders"], sourceKeyword))
	assert.Equal(t, []string{"Equatable"}, markers(units["lib/logic/wonders_state#WondersState"], unit.SourceSupertype))

	settings := units["lib/logic/settings_provider#Settings"]
	require.NotNil(t, settings)
	assert.Equal(t, []string{"riverpod"}, markers(settings, unit.SourceAnnotation), "the codegen annotation on a notifier class")
	assert.Equal(t, []string{"_$Settings"}, markers(settings, unit.SourceSupertype))
	store := units["lib/logic/settings_provider#settingsStore"]
	require.NotNil(t, store)
	assert.Equal(t, unit.KindFunction, store.Kind)
	assert.Equal(t, []string{"Riverpod"}, markers(store, unit.SourceAnnotation), "the annotation with arguments is keyed by its name")

	// Providers declared without codegen are top-level variables. The
	// variable is the unit and what made it is its marker, as a Redux
	// slice's createSlice is.
	counter := units["lib/logic/counter_provider#counterProvider"]
	require.NotNil(t, counter, "a provider variable is a unit")
	assert.Equal(t, unit.KindType, counter.Kind)
	assert.Equal(t, "counterProvider", counter.Name)
	assert.Equal(t, []string{"StateNotifierProvider"}, markers(counter, unit.SourceSupertype))
	repoProvider := units["lib/logic/counter_provider#wondersRepositoryProvider"]
	require.NotNil(t, repoProvider)
	assert.Equal(t, []string{"Provider"}, markers(repoProvider, unit.SourceSupertype), "a modifier like .autoDispose is not part of the key")
	assert.Equal(t, []string{"StateNotifier"}, markers(units["lib/logic/counter_provider#CounterNotifier"], unit.SourceSupertype))
	assert.Equal(t, []string{"GetxController"}, markers(units["lib/logic/home_controller#HomeController"], unit.SourceSupertype))

	assert.Empty(t, units["lib/data/wonders_repository#WondersRepository"].Markers[1:], "a plain class carries only its keyword")
	assert.Equal(t, []string{"class"}, markers(units["lib/data/api_client#ApiClient"], sourceKeyword))

	wonder := units["lib/models/wonder#Wonder"]
	require.NotNil(t, wonder)
	assert.Equal(t, []string{"freezed"}, markers(wonder, unit.SourceAnnotation))
	assert.Equal(t, []string{"_$Wonder"}, markers(wonder, unit.SourceSupertype), "the generated mixin is a supertype, as written")
	assert.Equal(t, []string{"Wonder"}, markers(units["lib/models/wonder.freezed#_Wonder"], unit.SourceSupertype), "the generated part's classes are units too, named by the part")
	assert.Equal(t, []string{"mixin"}, markers(units["lib/models/wonder.freezed#_$Wonder"], sourceKeyword))

	assert.Equal(t, unit.KindFunction, units["test/wonders_bloc_test#main"].Kind)

	// The imports detection reads: the package first, as the profiles match.
	imports := func(p string) []string {
		var out []string
		for _, s := range files[p].Snippets {
			if s.Type == file.ImportRaw {
				out = append(out, s.Value)
			}
		}
		return out
	}
	assert.Equal(t, []string{"flutter/material.dart", "flutter_bloc/flutter_bloc.dart", "wonders/logic/wonders_bloc.dart", "wonders/ui/widgets/wonder_card.dart"}, imports("lib/ui/screens/home_screen.dart"))
	assert.Equal(t, []string{"dio/dio.dart"}, imports("lib/data/api_client.dart"))
	assert.Equal(t, []string{"shared_preferences/shared_preferences.dart"}, imports("lib/data/settings_store.dart"))
	assert.Equal(t, []string{"get/get.dart"}, imports("lib/logic/home_controller.dart"))
	assert.Empty(t, imports("lib/logic/wonders_event.dart"), "a part has no imports of its own")
}

func TestFlutterReferencesRunBetweenTheLanes(t *testing.T) {
	_, units := analyseApp(t, flutterApp)
	assert.Equal(t, []string{"lib/ui/screens/home_screen#HomeScreen"}, rolledUp(units, "lib/main#WondersApp"))
	// The screen's State class is where a StatefulWidget's code is: it
	// reaches the bloc, the bloc's event and state (declared in parts), and
	// the card. Private names, which is what every State class has, count.
	assert.Equal(t, []string{"lib/ui/screens/home_screen#_HomeScreenState"}, rolledUp(units, "lib/ui/screens/home_screen#HomeScreen"))
	assert.Equal(t, []string{"lib/logic/wonders_bloc#WondersBloc", "lib/logic/wonders_event#LoadWonders", "lib/logic/wonders_state#WondersState", "lib/ui/screens/home_screen#HomeScreen", "lib/ui/widgets/wonder_card#WonderCard"},
		rolledUp(units, "lib/ui/screens/home_screen#_HomeScreenState"))
	assert.Equal(t, []string{"lib/logic/home_controller#HomeController"}, rolledUp(units, "lib/ui/screens/home_view#HomeView"))
	assert.Equal(t, []string{"lib/models/wonder#Wonder"}, rolledUp(units, "lib/ui/widgets/wonder_card#WonderCard"))
	// The bloc sees its parts' declarations without an import.
	assert.Equal(t, []string{"lib/data/wonders_repository#WondersRepository", "lib/logic/wonders_event#LoadWonders", "lib/logic/wonders_event#WondersEvent", "lib/logic/wonders_state#WondersState"},
		rolledUp(units, "lib/logic/wonders_bloc#WondersBloc"))
	assert.Equal(t, []string{"lib/models/wonder#Wonder"}, rolledUp(units, "lib/logic/wonders_state#WondersState"), "a part sees its library's imports")
	// Riverpod: the codegen notifier reaches its store through the generated
	// provider name, which is a variable; the plain providers reach what
	// they build.
	assert.Equal(t, []string{"lib/logic/counter_provider#CounterNotifier"}, rolledUp(units, "lib/logic/counter_provider#counterProvider"))
	assert.Equal(t, []string{"lib/data/api_client#ApiClient", "lib/data/wonders_repository#WondersRepository"}, rolledUp(units, "lib/logic/counter_provider#wondersRepositoryProvider"))
	assert.Equal(t, []string{"lib/data/settings_store#SettingsStore"}, rolledUp(units, "lib/logic/settings_provider#settingsStore"))
	// Repository -> client and model, and the planted screen.
	assert.Equal(t, []string{"lib/data/api_client#ApiClient", "lib/models/wonder#Wonder", "lib/ui/screens/home_screen#HomeScreen"}, rolledUp(units, "lib/data/wonders_repository#WondersRepository"))
	assert.Empty(t, rolledUp(units, "lib/data/api_client#ApiClient"), "a dependency's types resolve to nothing")
	// The planted model -> bloc reference.
	assert.Equal(t, []string{"lib/logic/wonders_bloc#WondersBloc"}, rolledUp(units, "lib/models/wonder_draft#WonderDraft"))
	assert.Equal(t, []string{"lib/data/api_client#ApiClient", "lib/data/wonders_repository#WondersRepository", "lib/logic/wonders_bloc#WondersBloc"}, rolledUp(units, "test/wonders_bloc_test#main"))
}
