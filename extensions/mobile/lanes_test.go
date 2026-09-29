package mobile

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/archstats/archstats/core"
	"github.com/archstats/archstats/core/unit"
	"github.com/archstats/archstats/extensions/basic"
	"github.com/archstats/archstats/extensions/treesitter/java"
	"github.com/archstats/archstats/extensions/treesitter/kotlin"
	"github.com/archstats/archstats/extensions/treesitter/typescript"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The Android and React Native mini-apps the UI's lanes are tested against
// (archstats-ui: features/frameworks/layers.mobile.test.ts), scanned end to
// end: language packs, manifest markers and the unit graph together, since
// an Android app is Kotlin and Java in one module and its manifest names
// classes from both.

func scan(t *testing.T, files map[string]string, exts ...core.Extension) *core.Results {
	t.Helper()
	root := t.TempDir()
	for p, src := range files {
		full := filepath.Join(root, p)
		require.NoError(t, os.MkdirAll(filepath.Dir(full), 0o755))
		require.NoError(t, os.WriteFile(full, []byte(src), 0o644))
	}
	results, err := core.New(&core.Config{RootPath: root, Extensions: append([]core.Extension{basic.Extension(), Extension()}, exts...)}).Analyze()
	require.NoError(t, err)
	return results
}

func markersOf(u *unit.Unit, source string) []string {
	var out []string
	for _, m := range u.Markers {
		if m.Source == source {
			out = append(out, m.Key)
		}
	}
	sort.Strings(out)
	return out
}

// edges is the unit graph as the UI reads it: members rolled up to the unit
// that owns them, self-edges dropped.
func edges(r *core.Results) map[string][]string {
	top := func(id string) string {
		for i := 0; i < 8; i++ {
			u := r.UnitByID[id]
			if u == nil || u.Owner == "" {
				return id
			}
			id = u.Owner
		}
		return id
	}
	seen := map[string]bool{}
	out := map[string][]string{}
	for _, c := range unit.Connections(r.Units) {
		from, to := top(c.From), top(c.To)
		if from == to || seen[from+"\x00"+to] {
			continue
		}
		seen[from+"\x00"+to] = true
		out[from] = append(out[from], to)
	}
	for _, list := range out {
		sort.Strings(list)
	}
	return out
}

func rawImportsOf(r *core.Results, file string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range r.SnippetsByFile[file] {
		if s.Type == "modularity__import__raw" && !seen[s.Value] {
			seen[s.Value] = true
			out = append(out, s.Value)
		}
	}
	sort.Strings(out)
	return out
}

const niaManifest = `<?xml version="1.0" encoding="utf-8"?>
<manifest xmlns:android="http://schemas.android.com/apk/res/android">
    <uses-permission android:name="android.permission.INTERNET" />
    <application android:name=".NiaApplication">
        <activity android:name=".MainActivity" android:exported="true">
            <intent-filter>
                <action android:name="android.intent.action.MAIN" />
                <category android:name="android.intent.category.LAUNCHER" />
            </intent-filter>
        </activity>
        <service android:name=".sync.SyncService" android:exported="false" />
        <receiver android:name=".BootReceiver" android:exported="false">
            <intent-filter><action android:name="android.intent.action.BOOT_COMPLETED" /></intent-filter>
        </receiver>
    </application>
</manifest>`

// A Hilt app with a Compose feature, a Room and Retrofit data layer, and a
// Java corner (a Fragment with its ViewModel and adapter, a receiver), as an
// app that has been around for a while looks.
var androidApp = map[string]string{
	"settings.gradle.kts":         `include(":app")` + "\n",
	"app/build.gradle.kts":        "plugins { id(\"com.android.application\") }\nandroid {\n    namespace = \"com.acme.nia\"\n}\n",
	"app/src/main/AndroidManifest.xml": niaManifest,
	"app/src/main/java/com/acme/nia/NiaApplication.kt": `package com.acme.nia

import android.app.Application
import dagger.hilt.android.HiltAndroidApp

@HiltAndroidApp
class NiaApplication : Application()
`,
	"app/src/main/java/com/acme/nia/MainActivity.kt": `package com.acme.nia

import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import com.acme.nia.feature.foryou.ForYouScreen
import dagger.hilt.android.AndroidEntryPoint

@AndroidEntryPoint
class MainActivity : ComponentActivity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        setContent { ForYouScreen() }
    }
}
`,
	"app/src/main/java/com/acme/nia/feature/foryou/ForYouScreen.kt": `package com.acme.nia.feature.foryou

import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.ui.tooling.preview.Preview
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.acme.nia.model.Topic

@Composable
fun ForYouScreen(viewModel: ForYouViewModel = hiltViewModel()) {
    val state by viewModel.uiState.collectAsStateWithLifecycle()
    LazyColumn { items(state.topics) { TopicRow(topic = it) } }
}

@Composable
fun TopicRow(topic: Topic) {
    Text(topic.name)
}

@Preview
@Composable
private fun TopicRowPreview() {
    TopicRow(Topic("1", "Compose"))
}
`,
	"app/src/main/java/com/acme/nia/feature/foryou/ForYouViewModel.kt": `package com.acme.nia.feature.foryou

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.acme.nia.data.TopicsRepository
import com.acme.nia.model.Topic
import dagger.hilt.android.lifecycle.HiltViewModel
import javax.inject.Inject
import kotlinx.coroutines.flow.SharingStarted
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.map
import kotlinx.coroutines.flow.stateIn

@HiltViewModel
class ForYouViewModel @Inject constructor(
    private val repository: TopicsRepository,
) : ViewModel() {
    val uiState: StateFlow<ForYouUiState> = repository.topics()
        .map { ForYouUiState(it) }
        .stateIn(viewModelScope, SharingStarted.WhileSubscribed(5_000), ForYouUiState())
}

data class ForYouUiState(val topics: List<Topic> = emptyList())
`,
	"app/src/main/java/com/acme/nia/data/TopicsRepository.kt": `package com.acme.nia.data

import android.content.Context
import android.content.Intent
import com.acme.nia.MainActivity
import com.acme.nia.data.local.TopicDao
import com.acme.nia.data.local.toTopic
import com.acme.nia.data.network.NiaNetworkApi
import com.acme.nia.model.Topic
import javax.inject.Inject
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.map

interface TopicsRepository {
    fun topics(): Flow<List<Topic>>
}

class OfflineFirstTopicsRepository @Inject constructor(
    private val dao: TopicDao,
    private val api: NiaNetworkApi,
) : TopicsRepository {
    override fun topics(): Flow<List<Topic>> = dao.all().map { entities -> entities.map { it.toTopic() } }
    suspend fun refresh() { api.topics() }
    // Planted: a repository that starts a screen.
    fun open(context: Context) = context.startActivity(Intent(context, MainActivity::class.java))
}
`,
	"app/src/main/java/com/acme/nia/data/local/TopicDao.kt": `package com.acme.nia.data.local

import androidx.room.Dao
import androidx.room.Query
import kotlinx.coroutines.flow.Flow

@Dao
interface TopicDao {
    @Query("SELECT * FROM topics")
    fun all(): Flow<List<TopicEntity>>
}
`,
	"app/src/main/java/com/acme/nia/data/local/TopicEntity.kt": `package com.acme.nia.data.local

import androidx.room.Entity
import androidx.room.PrimaryKey
import com.acme.nia.model.Topic

@Entity(tableName = "topics")
data class TopicEntity(
    @PrimaryKey val id: String,
    val name: String,
)

fun TopicEntity.toTopic() = Topic(id = id, name = name)
`,
	"app/src/main/java/com/acme/nia/data/local/NiaDatabase.kt": `package com.acme.nia.data.local

import androidx.room.Database
import androidx.room.RoomDatabase

@Database(entities = [TopicEntity::class], version = 1)
abstract class NiaDatabase : RoomDatabase() {
    abstract fun topicDao(): TopicDao
}
`,
	"app/src/main/java/com/acme/nia/data/network/NiaNetworkApi.kt": `package com.acme.nia.data.network

import kotlinx.serialization.Serializable
import retrofit2.http.GET

interface NiaNetworkApi {
    @GET("topics")
    suspend fun topics(): List<NetworkTopic>
}

@Serializable
data class NetworkTopic(val id: String, val name: String)
`,
	"app/src/main/java/com/acme/nia/di/DataModule.kt": `package com.acme.nia.di

import android.content.Context
import androidx.room.Room
import com.acme.nia.data.OfflineFirstTopicsRepository
import com.acme.nia.data.TopicsRepository
import com.acme.nia.data.local.NiaDatabase
import com.acme.nia.data.local.TopicDao
import com.acme.nia.data.network.NiaNetworkApi
import dagger.Binds
import dagger.Module
import dagger.Provides
import dagger.hilt.InstallIn
import dagger.hilt.android.qualifiers.ApplicationContext
import dagger.hilt.components.SingletonComponent
import javax.inject.Singleton
import retrofit2.Retrofit

@Module
@InstallIn(SingletonComponent::class)
object DataModule {
    @Provides
    @Singleton
    fun provideDatabase(@ApplicationContext context: Context): NiaDatabase =
        Room.databaseBuilder(context, NiaDatabase::class.java, "nia").build()

    @Provides
    fun provideTopicDao(db: NiaDatabase): TopicDao = db.topicDao()

    @Provides
    fun provideApi(): NiaNetworkApi = Retrofit.Builder().baseUrl("https://example.com").build().create(NiaNetworkApi::class.java)
}

@Module
@InstallIn(SingletonComponent::class)
abstract class RepositoryModule {
    @Binds
    abstract fun bindTopicsRepository(impl: OfflineFirstTopicsRepository): TopicsRepository
}
`,
	"app/src/main/java/com/acme/nia/model/Topic.kt": `package com.acme.nia.model

data class Topic(val id: String, val name: String)
`,
	// Planted: a model that reaches back up to a view model.
	"app/src/main/java/com/acme/nia/model/TopicDraft.kt": `package com.acme.nia.model

import com.acme.nia.feature.foryou.ForYouViewModel

class TopicDraft(val viewModel: ForYouViewModel) {
    var name: String = ""
}
`,
	"app/src/main/java/com/acme/nia/sync/SyncService.kt": `package com.acme.nia.sync

import android.app.Service
import android.content.Intent
import android.os.IBinder
import androidx.hilt.work.HiltWorker
import androidx.work.CoroutineWorker
import androidx.work.WorkerParameters
import android.content.Context
import com.acme.nia.data.OfflineFirstTopicsRepository
import dagger.assisted.Assisted
import dagger.assisted.AssistedInject

class SyncService : Service() {
    override fun onBind(intent: Intent): IBinder? = null
}

@HiltWorker
class SyncWorker @AssistedInject constructor(
    @Assisted context: Context,
    @Assisted params: WorkerParameters,
    private val repository: OfflineFirstTopicsRepository,
) : CoroutineWorker(context, params) {
    override suspend fun doWork(): Result { repository.refresh(); return Result.success() }
}
`,
	"app/src/main/java/com/acme/nia/BootReceiver.java": `package com.acme.nia;

import android.content.BroadcastReceiver;
import android.content.Context;
import android.content.Intent;
import com.acme.nia.sync.SyncService;

public class BootReceiver extends BroadcastReceiver {
    @Override
    public void onReceive(Context context, Intent intent) {
        context.startService(new Intent(context, SyncService.class));
    }
}
`,
	"app/src/main/java/com/acme/nia/feature/settings/SettingsFragment.java": `package com.acme.nia.feature.settings;

import android.os.Bundle;
import android.view.LayoutInflater;
import android.view.View;
import android.view.ViewGroup;
import androidx.fragment.app.Fragment;
import androidx.lifecycle.ViewModelProvider;
import androidx.recyclerview.widget.RecyclerView;

public class SettingsFragment extends Fragment {
    private SettingsViewModel viewModel;

    @Override
    public View onCreateView(LayoutInflater inflater, ViewGroup container, Bundle savedInstanceState) {
        viewModel = new ViewModelProvider(this).get(SettingsViewModel.class);
        RecyclerView list = new RecyclerView(requireContext());
        list.setAdapter(new SettingsAdapter());
        return list;
    }
}
`,
	"app/src/main/java/com/acme/nia/feature/settings/SettingsViewModel.java": `package com.acme.nia.feature.settings;

import androidx.lifecycle.MutableLiveData;
import androidx.lifecycle.ViewModel;
import com.acme.nia.data.local.NiaDatabase;

public class SettingsViewModel extends ViewModel {
    private final MutableLiveData<Boolean> darkMode = new MutableLiveData<>(false);
    private NiaDatabase database;
}
`,
	"app/src/main/java/com/acme/nia/feature/settings/SettingsAdapter.java": `package com.acme.nia.feature.settings;

import android.view.ViewGroup;
import android.widget.TextView;
import androidx.recyclerview.widget.RecyclerView;

public class SettingsAdapter extends RecyclerView.Adapter<SettingsAdapter.Holder> {
    static class Holder extends RecyclerView.ViewHolder {
        Holder(TextView view) { super(view); }
    }
    @Override public Holder onCreateViewHolder(ViewGroup parent, int viewType) { return new Holder(new TextView(parent.getContext())); }
    @Override public void onBindViewHolder(Holder holder, int position) {}
    @Override public int getItemCount() { return 0; }
}
`,
	"app/src/test/java/com/acme/nia/feature/foryou/ForYouViewModelTest.kt": `package com.acme.nia.feature.foryou

import com.acme.nia.data.TopicsRepository
import kotlinx.coroutines.test.runTest
import org.junit.Test

class ForYouViewModelTest {
    @Test
    fun exposesTopics() = runTest {
        val vm = ForYouViewModel(FakeTopicsRepository())
    }
}
`,
	"app/src/androidTest/java/com/acme/nia/MainActivityTest.kt": `package com.acme.nia

import androidx.test.ext.junit.runners.AndroidJUnit4
import dagger.hilt.android.testing.HiltAndroidTest
import org.junit.runner.RunWith

@HiltAndroidTest
@RunWith(AndroidJUnit4::class)
class MainActivityTest
`,
}

func TestAndroidUnitsAndTheirMarkers(t *testing.T) {
	r := scan(t, androidApp, &kotlin.Extension{}, &java.Extension{})

	app := r.UnitByID["com.acme.nia.NiaApplication"]
	require.NotNil(t, app)
	assert.Equal(t, []string{"HiltAndroidApp"}, markersOf(app, unit.SourceAnnotation))
	assert.Equal(t, []string{"Application"}, markersOf(app, unit.SourceSupertype))
	assert.Equal(t, []string{"application"}, markersOf(app, unit.SourceManifest), "the manifest names it, relative to the Gradle namespace")

	main := r.UnitByID["com.acme.nia.MainActivity"]
	require.NotNil(t, main)
	assert.Equal(t, []string{"AndroidEntryPoint"}, markersOf(main, unit.SourceAnnotation))
	assert.Equal(t, []string{"ComponentActivity"}, markersOf(main, unit.SourceSupertype))
	assert.Equal(t, []string{"activity", "exported", "launcher"}, markersOf(main, unit.SourceManifest))

	// Compose: the screen and the row are functions, told apart only by name.
	screen := r.UnitByID["com.acme.nia.feature.foryou.ForYouScreen"]
	require.NotNil(t, screen)
	assert.Equal(t, unit.KindFunction, screen.Kind)
	assert.Equal(t, []string{"Composable"}, markersOf(screen, unit.SourceAnnotation))
	row := r.UnitByID["com.acme.nia.feature.foryou.TopicRow"]
	assert.Equal(t, []string{"Composable"}, markersOf(row, unit.SourceAnnotation))
	assert.Equal(t, []string{"Composable", "Preview"}, markersOf(r.UnitByID["com.acme.nia.feature.foryou.TopicRowPreview"], unit.SourceAnnotation))

	vm := r.UnitByID["com.acme.nia.feature.foryou.ForYouViewModel"]
	assert.Equal(t, []string{"HiltViewModel", "Inject"}, markersOf(vm, unit.SourceAnnotation), "the injected constructor marks the class")
	assert.Equal(t, []string{"ViewModel"}, markersOf(vm, unit.SourceSupertype))
	assert.Equal(t, []string{"data"}, markersOf(r.UnitByID["com.acme.nia.feature.foryou.ForYouUiState"], "keyword"), "a data class says so, and nothing else")

	assert.Equal(t, []string{"Dao"}, markersOf(r.UnitByID["com.acme.nia.data.local.TopicDao"], unit.SourceAnnotation))
	assert.Equal(t, []string{"Entity"}, markersOf(r.UnitByID["com.acme.nia.data.local.TopicEntity"], unit.SourceAnnotation), "the Room entity; a parameter's @PrimaryKey marks no class")
	assert.Equal(t, "com.acme.nia.data.local.TopicEntity", r.UnitByID["com.acme.nia.data.local.TopicEntity.toTopic"].Owner, "a mapping extension belongs to the entity")
	db := r.UnitByID["com.acme.nia.data.local.NiaDatabase"]
	assert.Equal(t, []string{"Database"}, markersOf(db, unit.SourceAnnotation))
	assert.Equal(t, []string{"RoomDatabase"}, markersOf(db, unit.SourceSupertype))
	api := r.UnitByID["com.acme.nia.data.network.NiaNetworkApi"]
	require.NotNil(t, api)
	assert.Empty(t, api.Markers, "a Retrofit interface is known by its imports and name alone")
	assert.Equal(t, []string{"Serializable"}, markersOf(r.UnitByID["com.acme.nia.data.network.NetworkTopic"], unit.SourceAnnotation))
	assert.Empty(t, markersOf(r.UnitByID["com.acme.nia.data.TopicsRepository"], unit.SourceAnnotation))
	assert.Equal(t, []string{"TopicsRepository"}, markersOf(r.UnitByID["com.acme.nia.data.OfflineFirstTopicsRepository"], unit.SourceSupertype))

	module := r.UnitByID["com.acme.nia.di.DataModule"]
	require.NotNil(t, module, "a Kotlin object is a type")
	assert.Equal(t, []string{"InstallIn", "Module"}, markersOf(module, unit.SourceAnnotation))
	assert.Equal(t, []string{"InstallIn", "Module"}, markersOf(r.UnitByID["com.acme.nia.di.RepositoryModule"], unit.SourceAnnotation))
	assert.Equal(t, []string{"Provides", "Singleton"}, markersOf(r.UnitByID["com.acme.nia.di.DataModule.provideDatabase"], unit.SourceAnnotation), "a method's annotations are the method's")

	assert.Equal(t, []string{"data"}, markersOf(r.UnitByID["com.acme.nia.model.Topic"], "keyword"))
	assert.Empty(t, r.UnitByID["com.acme.nia.model.TopicDraft"].Markers, "a plain class carries nothing")

	svc := r.UnitByID["com.acme.nia.sync.SyncService"]
	assert.Equal(t, []string{"Service"}, markersOf(svc, unit.SourceSupertype))
	assert.Equal(t, []string{"service"}, markersOf(svc, unit.SourceManifest), "a dotted manifest name resolves under the namespace")
	worker := r.UnitByID["com.acme.nia.sync.SyncWorker"]
	assert.Equal(t, []string{"AssistedInject", "HiltWorker"}, markersOf(worker, unit.SourceAnnotation))
	assert.Equal(t, []string{"CoroutineWorker"}, markersOf(worker, unit.SourceSupertype))

	// The Java corner, in the same module and package tree.
	receiver := r.UnitByID["com.acme.nia.BootReceiver"]
	require.NotNil(t, receiver)
	assert.Equal(t, []string{"BroadcastReceiver"}, markersOf(receiver, unit.SourceSupertype))
	assert.Equal(t, []string{"receiver"}, markersOf(receiver, unit.SourceManifest), "a manifest class declared in Java")
	assert.Equal(t, []string{"Fragment"}, markersOf(r.UnitByID["com.acme.nia.feature.settings.SettingsFragment"], unit.SourceSupertype))
	assert.Equal(t, []string{"ViewModel"}, markersOf(r.UnitByID["com.acme.nia.feature.settings.SettingsViewModel"], unit.SourceSupertype))
	adapter := r.UnitByID["com.acme.nia.feature.settings.SettingsAdapter"]
	require.NotNil(t, adapter)
	// The Java pack records a qualified supertype as both of its parts; the
	// lane rules read the last one.
	assert.Equal(t, []string{"Adapter", "RecyclerView"}, markersOf(adapter, unit.SourceSupertype))
	holder := r.UnitByID["com.acme.nia.feature.settings.SettingsAdapter.Holder"]
	require.NotNil(t, holder, "a nested class is named through its outer class")
	assert.Equal(t, "com.acme.nia.feature.settings.SettingsAdapter", holder.Owner)
	assert.Equal(t, []string{"RecyclerView", "ViewHolder"}, markersOf(holder, unit.SourceSupertype))

	assert.Equal(t, []string{"HiltAndroidTest", "RunWith"}, markersOf(r.UnitByID["com.acme.nia.MainActivityTest"], unit.SourceAnnotation))
	assert.NotNil(t, r.UnitByID["com.acme.nia.feature.foryou.ForYouViewModelTest"])

	// The imports detection reads, as written.
	assert.Equal(t, []string{"android.os.Bundle", "androidx.activity.ComponentActivity", "androidx.activity.compose.setContent", "com.acme.nia.feature.foryou.ForYouScreen", "dagger.hilt.android.AndroidEntryPoint"},
		rawImportsOf(r, "app/src/main/java/com/acme/nia/MainActivity.kt"))
	assert.Equal(t, []string{"androidx.room.Dao", "androidx.room.Query", "kotlinx.coroutines.flow.Flow"}, rawImportsOf(r, "app/src/main/java/com/acme/nia/data/local/TopicDao.kt"))
	assert.Equal(t, []string{"kotlinx.serialization.Serializable", "retrofit2.http.GET"}, rawImportsOf(r, "app/src/main/java/com/acme/nia/data/network/NiaNetworkApi.kt"))
	assert.Equal(t, []string{"android.content.BroadcastReceiver", "android.content.Context", "android.content.Intent", "com.acme.nia.sync.SyncService"}, rawImportsOf(r, "app/src/main/java/com/acme/nia/BootReceiver.java"))
}

func TestAndroidReferencesRunBetweenTheLanes(t *testing.T) {
	r := scan(t, androidApp, &kotlin.Extension{}, &java.Extension{})
	e := edges(r)
	assert.Equal(t, []string{"com.acme.nia.feature.foryou.ForYouScreen"}, e["com.acme.nia.MainActivity"])
	// Screen -> view model, its state and the row; row -> model.
	assert.Equal(t, []string{"com.acme.nia.feature.foryou.ForYouViewModel", "com.acme.nia.feature.foryou.TopicRow"}, e["com.acme.nia.feature.foryou.ForYouScreen"])
	assert.Equal(t, []string{"com.acme.nia.model.Topic"}, e["com.acme.nia.feature.foryou.TopicRow"])
	assert.Equal(t, []string{"com.acme.nia.data.TopicsRepository", "com.acme.nia.feature.foryou.ForYouUiState"}, e["com.acme.nia.feature.foryou.ForYouViewModel"])
	assert.Equal(t, []string{"com.acme.nia.model.Topic"}, e["com.acme.nia.feature.foryou.ForYouUiState"])
	// Repository -> DAO, API, model, and the planted activity. The entity is
	// reached only through its imported mapping extension, `toTopic`, which
	// resolves to nothing: the reference names the package and the function,
	// and the function's unit is filed under the entity it extends.
	assert.Equal(t, []string{"com.acme.nia.MainActivity", "com.acme.nia.data.TopicsRepository", "com.acme.nia.data.local.TopicDao", "com.acme.nia.data.network.NiaNetworkApi", "com.acme.nia.model.Topic"},
		e["com.acme.nia.data.OfflineFirstTopicsRepository"])
	assert.Equal(t, []string{"com.acme.nia.data.local.TopicEntity"}, e["com.acme.nia.data.local.TopicDao"])
	assert.Equal(t, []string{"com.acme.nia.model.Topic"}, e["com.acme.nia.data.local.TopicEntity"], "the mapping extension is the entity's")
	assert.Equal(t, []string{"com.acme.nia.data.local.TopicDao", "com.acme.nia.data.local.TopicEntity"}, e["com.acme.nia.data.local.NiaDatabase"])
	assert.Equal(t, []string{"com.acme.nia.data.network.NetworkTopic"}, e["com.acme.nia.data.network.NiaNetworkApi"])
	// DI wires everything below the screen.
	assert.Equal(t, []string{"com.acme.nia.data.local.NiaDatabase", "com.acme.nia.data.local.TopicDao", "com.acme.nia.data.network.NiaNetworkApi"}, e["com.acme.nia.di.DataModule"])
	assert.Equal(t, []string{"com.acme.nia.data.OfflineFirstTopicsRepository", "com.acme.nia.data.TopicsRepository"}, e["com.acme.nia.di.RepositoryModule"])
	// The planted model -> view model reference.
	assert.Equal(t, []string{"com.acme.nia.feature.foryou.ForYouViewModel"}, e["com.acme.nia.model.TopicDraft"])
	// Java reaches Kotlin and Kotlin's nested types resolve.
	assert.Equal(t, []string{"com.acme.nia.sync.SyncService"}, e["com.acme.nia.BootReceiver"])
	assert.Equal(t, []string{"com.acme.nia.feature.settings.SettingsAdapter", "com.acme.nia.feature.settings.SettingsViewModel"}, e["com.acme.nia.feature.settings.SettingsFragment"])
	assert.Equal(t, []string{"com.acme.nia.data.local.NiaDatabase"}, e["com.acme.nia.feature.settings.SettingsViewModel"])
	assert.Equal(t, []string{"com.acme.nia.data.OfflineFirstTopicsRepository"}, e["com.acme.nia.sync.SyncWorker"])
	assert.Equal(t, []string{"com.acme.nia.feature.foryou.ForYouViewModel"}, e["com.acme.nia.feature.foryou.ForYouViewModelTest"])
}

// An Expo Router app with Redux Toolkit state, a hook per feature, an axios
// client, TurboModule specs, and the route files Expo names by path rather
// than by a Screen suffix.
var reactNativeApp = map[string]string{
	"package.json": `{"name": "social", "dependencies": {"react-native": "0.74.0", "expo-router": "3.0.0", "@reduxjs/toolkit": "2.0.0"}}`,
	"app/_layout.tsx": `import { Stack } from 'expo-router'
import { Provider } from 'react-redux'
import { store } from '../src/state/store'

export default function RootLayout() {
  return <Provider store={store}><Stack /></Provider>
}
`,
	"app/(tabs)/index.tsx": `import { View } from 'react-native'
import { FeedList } from '../../src/components/FeedList'
import { useFeed } from '../../src/hooks/useFeed'

export default function FeedTab() {
  const { posts } = useFeed()
  return <View><FeedList posts={posts} /></View>
}
`,
	"src/screens/ProfileScreen.tsx": `import { Text, View } from 'react-native'
import { useSelector } from 'react-redux'
import { Avatar } from '../components/Avatar'
import { selectProfile } from '../state/profileSlice'

export function ProfileScreen() {
  const profile = useSelector(selectProfile)
  return <View><Avatar uri={profile.avatar} /><Text>{profile.handle}</Text></View>
}
`,
	"src/components/FeedList.tsx": `import { FlatList } from 'react-native'
import type { Post } from '../types'
import { PostCard } from './PostCard'

export function FeedList({ posts }: { posts: Post[] }) {
  return <FlatList data={posts} renderItem={({ item }) => <PostCard post={item} />} />
}
`,
	"src/components/PostCard.tsx": `import { Text } from 'react-native'
import type { Post } from '../types'

export const PostCard = ({ post }: { post: Post }) => <Text>{post.text}</Text>
`,
	"src/components/Avatar.tsx": `import { Image } from 'react-native'

export function Avatar({ uri }: { uri: string }) {
  return <Image source={{ uri }} />
}
`,
	"src/hooks/useFeed.ts": `import { useEffect } from 'react'
import { useDispatch, useSelector } from 'react-redux'
import { loadFeed, selectPosts } from '../state/feedSlice'

export function useFeed() {
  const dispatch = useDispatch()
  const posts = useSelector(selectPosts)
  useEffect(() => { dispatch(loadFeed()) }, [dispatch])
  return { posts }
}
`,
	"src/state/feedSlice.ts": `import { createAsyncThunk, createSlice } from '@reduxjs/toolkit'
import { fetchFeed } from '../api/client'
import type { Post } from '../types'
import type { RootState } from './store'

export interface FeedState { posts: Post[]; status: 'idle' | 'loading' }

export const loadFeed = createAsyncThunk('feed/load', () => fetchFeed())

export const feedSlice = createSlice({
  name: 'feed',
  initialState: { posts: [], status: 'idle' } as FeedState,
  reducers: {
    clear(state) { state.posts = [] },
  },
  extraReducers: (builder) => {
    builder.addCase(loadFeed.fulfilled, (state, action) => { state.posts = action.payload })
  },
})

export const selectPosts = (state: RootState) => state.feed.posts
`,
	"src/state/profileSlice.ts": `import { createSlice } from '@reduxjs/toolkit'
import type { Profile } from '../types'
import type { RootState } from './store'

export const profileSlice = createSlice({
  name: 'profile',
  initialState: { handle: '', avatar: '' } as Profile,
  reducers: { setHandle(state, action) { state.handle = action.payload } },
})

export const selectProfile = (state: RootState) => state.profile
`,
	"src/state/store.ts": `import { configureStore } from '@reduxjs/toolkit'
import { feedSlice } from './feedSlice'
import { profileSlice } from './profileSlice'

export const store = configureStore({ reducer: { feed: feedSlice.reducer, profile: profileSlice.reducer } })
export type RootState = ReturnType<typeof store.getState>
`,
	"src/api/client.ts": `import axios from 'axios'
import type { Post, Profile } from '../types'
import { ProfileScreen } from '../screens/ProfileScreen'

export async function fetchFeed(): Promise<Post[]> {
  return (await axios.get('/feed')).data
}

export async function fetchProfile(): Promise<Profile> {
  return (await axios.get('/me')).data
}

// Planted: a client that knows a screen.
export function openProfile() {
  return ProfileScreen
}
`,
	"src/types.ts": `export interface Post { id: string; text: string }
export interface Profile { handle: string; avatar: string }
`,
	// Planted: a type that reaches back up to a hook.
	"src/types/draft.ts": `import { useFeed } from '../hooks/useFeed'

export interface Draft { feed: ReturnType<typeof useFeed>; text: string }
`,
	"src/native/share.ts": `import { NativeModules } from 'react-native'

export function share(text: string): Promise<void> {
  return NativeModules.ShareModule.share(text)
}
`,
	"specs/NativeCalculator.ts": `import type { TurboModule } from 'react-native'
import { TurboModuleRegistry } from 'react-native'

export interface Spec extends TurboModule {
  add(a: number, b: number): Promise<number>
}

export default TurboModuleRegistry.getEnforcing<Spec>('NativeCalculator')
`,
	"__tests__/FeedList.test.tsx": `import { render } from '@testing-library/react-native'
import { FeedList } from '../src/components/FeedList'

test('renders posts', () => {
  render(<FeedList posts={[]} />)
})
`,
}

func TestReactNativeUnitsAndTheirMarkers(t *testing.T) {
	r := scan(t, reactNativeApp, &typescript.Extension{})

	// Route files are named by path; the component inside by whatever the
	// author chose.
	tab := r.UnitByID["app/(tabs)#FeedTab"]
	require.NotNil(t, tab, "an index file's module is its folder")
	assert.Equal(t, unit.KindFunction, tab.Kind)
	assert.Empty(t, tab.Markers)
	assert.Equal(t, unit.KindFunction, r.UnitByID["app/_layout#RootLayout"].Kind)
	assert.Equal(t, unit.KindFunction, r.UnitByID["src/screens/ProfileScreen#ProfileScreen"].Kind)
	assert.Equal(t, unit.KindFunction, r.UnitByID["src/components/FeedList#FeedList"].Kind)
	assert.Equal(t, unit.KindFunction, r.UnitByID["src/components/PostCard#PostCard"].Kind, "an arrow component is a function")
	assert.Equal(t, unit.KindFunction, r.UnitByID["src/hooks/useFeed#useFeed"].Kind)

	// A slice is a type made by createSlice, owning its reducers.
	slice := r.UnitByID["src/state/feedSlice#feedSlice"]
	require.NotNil(t, slice)
	assert.Equal(t, unit.KindType, slice.Kind)
	assert.Equal(t, []string{"createSlice"}, markersOf(slice, unit.SourceSupertype))
	assert.Equal(t, slice.ID, r.UnitByID["src/state/feedSlice#feedSlice.clear"].Owner)
	thunk := r.UnitByID["src/state/feedSlice#loadFeed"]
	require.NotNil(t, thunk, "a thunk is the action a hook dispatches")
	assert.Equal(t, []string{"createAsyncThunk"}, markersOf(thunk, unit.SourceSupertype))
	assert.Nil(t, r.UnitByID["src/state/store#store"], "configureStore makes no unit")

	assert.Equal(t, []string{"interface"}, markersOf(r.UnitByID["src/types#Post"], unit.SourceSupertype))
	assert.Equal(t, []string{"interface"}, markersOf(r.UnitByID["src/state/feedSlice#FeedState"], unit.SourceSupertype))
	assert.Equal(t, []string{"interface"}, markersOf(r.UnitByID["specs/NativeCalculator#Spec"], unit.SourceSupertype), "the TurboModule it extends is not recorded")
	assert.Equal(t, unit.KindFunction, r.UnitByID["src/native/share#share"].Kind)
	assert.Equal(t, unit.KindFunction, r.UnitByID["src/api/client#fetchFeed"].Kind)

	// The imports detection reads, as written.
	assert.Equal(t, []string{"../../src/components/FeedList", "../../src/hooks/useFeed", "react-native"}, rawImportsOf(r, "app/(tabs)/index.tsx"))
	assert.Equal(t, []string{"../api/client", "../types", "./store", "@reduxjs/toolkit"}, rawImportsOf(r, "src/state/feedSlice.ts"))
	assert.Equal(t, []string{"../screens/ProfileScreen", "../types", "axios"}, rawImportsOf(r, "src/api/client.ts"))
	assert.Equal(t, []string{"react-native"}, rawImportsOf(r, "specs/NativeCalculator.ts"))
	assert.Equal(t, []string{"../src/components/FeedList", "@testing-library/react-native"}, rawImportsOf(r, "__tests__/FeedList.test.tsx"))
}

func TestReactNativeReferencesRunBetweenTheLanes(t *testing.T) {
	r := scan(t, reactNativeApp, &typescript.Extension{})
	e := edges(r)
	assert.Equal(t, []string{"src/components/FeedList#FeedList", "src/hooks/useFeed#useFeed"}, e["app/(tabs)#FeedTab"])
	// Screen -> component and selector; a selector is an arrow function, so
	// a unit.
	assert.Equal(t, []string{"src/components/Avatar#Avatar", "src/state/profileSlice#selectProfile"}, e["src/screens/ProfileScreen#ProfileScreen"])
	assert.Equal(t, []string{"src/components/PostCard#PostCard", "src/types#Post"}, e["src/components/FeedList#FeedList"])
	assert.Equal(t, []string{"src/types#Post"}, e["src/components/PostCard#PostCard"])
	assert.Equal(t, []string{"src/state/feedSlice#loadFeed", "src/state/feedSlice#selectPosts"}, e["src/hooks/useFeed#useFeed"])
	// The slice itself names nothing imported. The thunk beside it is a unit
	// (createAsyncThunk), so the call to the client is the thunk's.
	assert.Empty(t, e["src/state/feedSlice#feedSlice"])
	assert.Equal(t, []string{"src/api/client#fetchFeed"}, e["src/state/feedSlice#loadFeed"])
	assert.Empty(t, e["src/state/feedSlice#"])
	assert.Equal(t, []string{"src/types#Post"}, e["src/state/feedSlice#FeedState"])
	assert.Equal(t, []string{"src/types#Profile"}, e["src/state/profileSlice#profileSlice"])
	assert.Equal(t, []string{"src/state/feedSlice#feedSlice", "src/state/profileSlice#profileSlice"}, e["src/state/store#"])
	assert.Equal(t, []string{"src/types#Post"}, e["src/api/client#fetchFeed"])
	// The planted client -> screen and type -> hook references.
	assert.Equal(t, []string{"src/screens/ProfileScreen#ProfileScreen"}, e["src/api/client#openProfile"])
	assert.Equal(t, []string{"src/hooks/useFeed#useFeed"}, e["src/types/draft#Draft"])
	assert.Equal(t, []string{"src/components/FeedList#FeedList"}, e["__tests__/FeedList.test#"], "a test file's calls are the file's own")
}
