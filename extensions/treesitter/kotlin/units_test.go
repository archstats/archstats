package kotlin

import (
	"testing"

	"github.com/archstats/archstats/core/unit"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func unitsIn(t *testing.T, path, src string) map[string]*unit.Unit {
	t.Helper()
	res := createKotlinLanguagePack().AnalyzeFileContent(path, []byte(src))
	require.NotNilf(t, res, "%s was not analysed", path)
	out := map[string]*unit.Unit{}
	for _, u := range unitsFrom(path, res) {
		out[u.ID] = u
	}
	return out
}

const sample = `package org.acme.db

import org.acme.core.Table

class Database(val name: String) : Closeable {
    fun connect() {}
}

object Registry {
    fun all(): List<Table> = emptyList()
}

interface Store {
    fun put(k: String)
}

fun Table.selectAll(): Query = Query(this)

fun topLevel(): Int = 1
`

// Exposed declares 225 extension functions. `fun Table.selectAll()` is
// Table's, written in another file and usually another module, which is
// exactly the case the unit grain keeps apart: where a unit lives and what
// it belongs to are different questions.
func TestExtensionFunctionBelongsToItsReceiver(t *testing.T) {
	byID := unitsIn(t, "src/db/Database.kt", sample)

	ext := byID["org.acme.db.Table.selectAll"]
	require.NotNil(t, ext, "the extension function is missing")
	assert.Equal(t, unit.KindFunction, ext.Kind)
	assert.Equal(t, "selectAll", ext.Name)
	assert.Equal(t, "org.acme.db.Table", ext.Owner, "it belongs to Table, not to the file")

	// A plain top-level function belongs to nothing. The return type is also
	// a user_type and must not be mistaken for a receiver.
	top := byID["org.acme.db.topLevel"]
	require.NotNil(t, top)
	assert.Empty(t, top.Owner)
}

// An interface is a class_declaration in this grammar, and an object is its
// own kind of declaration. Both are types.
func TestClassesObjectsAndInterfacesAreAllTypes(t *testing.T) {
	byID := unitsIn(t, "src/db/Database.kt", sample)

	for _, id := range []string{"org.acme.db.Database", "org.acme.db.Registry", "org.acme.db.Store"} {
		require.Containsf(t, byID, id, "%s is missing", id)
		assert.Equal(t, unit.KindType, byID[id].Kind)
	}
}

func TestSupertypesAreMarkers(t *testing.T) {
	byID := unitsIn(t, "src/db/Database.kt", sample)
	assert.True(t, byID["org.acme.db.Database"].HasMarker("Closeable"))
	assert.False(t, byID["org.acme.db.Registry"].HasMarker("Closeable"))
}

// A function declared in a class body belongs to that class.
func TestMethodsBelongToTheEnclosingType(t *testing.T) {
	byID := unitsIn(t, "src/db/Database.kt", sample)
	connect := byID["org.acme.db.Database.connect"]
	require.NotNil(t, connect)
	assert.Equal(t, "org.acme.db.Database", connect.Owner)
}

// An imported name is a reference of the declarations that use it, and only
// of those. Every top-level declaration used to receive every import of its
// file, so Database "used" Table without mentioning it.
func TestImportsBecomeReferencesWhereUsed(t *testing.T) {
	byID := unitsIn(t, "src/db/Database.kt", sample)
	table := unit.Ref{Module: "org.acme.core", Name: "Table"}
	assert.Contains(t, byID["org.acme.db.Registry.all"].Refs, table)
	assert.NotContains(t, byID["org.acme.db.Database"].Refs, table)
	assert.NotContains(t, byID["org.acme.db.topLevel"].Refs, table)
}

// A class never imports its own package, and a star import names no type:
// both are references all the same, resolved exactly where the compiler
// would look.
func TestSamePackageAndStarImportsAreReferences(t *testing.T) {
	src := `package org.acme.orders

import org.acme.money.*

class OrderService(private val repo: OrderRepository) {
    fun total(): Money = Money.zero()
}

val registry = OrderRegistry()
`
	byID := unitsIn(t, "src/orders/OrderService.kt", src)
	svc := byID["org.acme.orders.OrderService"]
	require.NotNil(t, svc)
	assert.Contains(t, svc.Refs, unit.Ref{Module: "org.acme.orders", Name: "OrderRepository", Exact: true})
	total := byID["org.acme.orders.OrderService.total"]
	require.NotNil(t, total)
	assert.Contains(t, total.Refs, unit.Ref{Module: "org.acme.money", Name: "Money", Exact: true})

	// A use at file level belongs to the file itself.
	mod := byID["org.acme.orders.OrderService#"]
	require.NotNil(t, mod, "file-level uses need a module unit")
	assert.Equal(t, unit.KindModule, mod.Kind)
	assert.Contains(t, mod.Refs, unit.Ref{Module: "org.acme.orders", Name: "OrderRegistry", Exact: true})
}

// A file with no package header still names its units.
func TestNoPackageHeader(t *testing.T) {
	byID := unitsIn(t, "Main.kt", "class Thing\n")
	assert.Contains(t, byID, "Thing")
}

// Exposed declares a `Town` inside six test classes of one package. Named by
// package alone they were one unit using everything all six used.
func TestNestedTypesAreNamedThroughTheirEnclosingType(t *testing.T) {
	src := `package org.acme.tests

class UuidTableEntityTest {
    object Towns : Table()
    class Town(id: Int) {
        fun describe() = 1
    }
}
`
	byID := unitsIn(t, "src/UuidTableEntityTest.kt", src)
	require.Contains(t, byID, "org.acme.tests.UuidTableEntityTest.Town")
	require.Contains(t, byID, "org.acme.tests.UuidTableEntityTest.Towns")
	assert.NotContains(t, byID, "org.acme.tests.Town")
	assert.Equal(t, "Town", byID["org.acme.tests.UuidTableEntityTest.Town"].Name)
	assert.Equal(t, "org.acme.tests.UuidTableEntityTest", byID["org.acme.tests.UuidTableEntityTest.Town"].Owner)
	assert.Empty(t, byID["org.acme.tests.UuidTableEntityTest"].Owner)
	describe := byID["org.acme.tests.UuidTableEntityTest.Town.describe"]
	require.NotNil(t, describe, "a method of a nested class is named through it")
	assert.Equal(t, "org.acme.tests.UuidTableEntityTest.Town", describe.Owner)
}

// Interfaces and sealed types are abstract; abstractness counted only
// abstract classes.
func TestAbstractTypesIncludeInterfacesAndSealedTypes(t *testing.T) {
	src := `package a

abstract class Base
sealed class Result
interface Store
sealed interface Event
fun interface Hasher { fun hash(): Int }
class Plain
enum class Kind { A }
object Registry
`
	res := createKotlinLanguagePack().AnalyzeFileContent("a/Types.kt", []byte(src))
	require.NotNil(t, res)
	counts := map[string]int{}
	for _, st := range res.Stats {
		if v, ok := st.Value.(int); ok {
			counts[st.StatType] += v
		}
	}
	assert.Equal(t, 5, counts["modularity__types__abstract"], "Base, Result, Store, Event, Hasher")
	assert.Equal(t, 8, counts["modularity__types__total"])
}

// A supertype written as a constructor call is a supertype: every Exposed
// table is `object Users : Table()`. Extension receivers are read through
// type parameters.
func TestSupertypesAndReceiversInEveryForm(t *testing.T) {
	src := `package org.acme

object Users : IntIdTable("users") {
    val name = varchar("name", 50)
}

class Cached(private val d: Store) : Store by d, Closeable

fun <T> Column<T>.comment(text: String): Column<T> = this
fun String?.orBlank(): String = this ?: ""
`
	byID := unitsIn(t, "src/Users.kt", src)
	assert.True(t, byID["org.acme.Users"].HasMarker("IntIdTable"))
	assert.True(t, byID["org.acme.Cached"].HasMarker("Store"))
	assert.True(t, byID["org.acme.Cached"].HasMarker("Closeable"))
	comment := byID["org.acme.Column.comment"]
	require.NotNil(t, comment, "an extension on a generic type is still an extension")
	assert.Equal(t, "org.acme.Column", comment.Owner)
	require.NotNil(t, byID["org.acme.String.orBlank"])
}

// An extension on a type parameter extends no type in particular.
func TestAnExtensionOnATypeParameterIsAPlainFunction(t *testing.T) {
	byID := unitsIn(t, "src/Ops.kt", "package org.acme\n\nfun <T : Table> T.deleteWhere(limit: Int): Int = 0\n")
	f := byID["org.acme.deleteWhere"]
	require.NotNil(t, f)
	assert.Empty(t, f.Owner)
	assert.NotContains(t, byID, "org.acme.T.deleteWhere")
}

const compose = `package org.acme.feature

import androidx.compose.runtime.Composable

@Composable
fun LoginScreen(viewModel: LoginViewModel) {}

@HiltViewModel
class LoginViewModel @Inject constructor(private val repo: Repo) : ViewModel() {
    @VisibleForTesting
    fun reset() {}
}

@Preview(showBackground = true)
@androidx.compose.runtime.Composable
private fun LoginPreview() {}

@RunWith(AndroidJUnit4::class)
class LoginTest(@param:Named("x") val x: String) {
    @Test fun opens() {}
}

object NiaButtonDefaults
`

// Compose screens are functions. An annotation used to be handed to the next
// class in the file, so nowinandroid's 168 composables carried nothing and
// NiaButtonDefaults, the object after them, carried Composable five times.
func TestAnnotationsBelongToTheirOwnDeclaration(t *testing.T) {
	byID := unitsIn(t, "src/feature/Login.kt", compose)

	screen := byID["org.acme.feature.LoginScreen"]
	require.NotNil(t, screen)
	assert.True(t, screen.HasMarker("Composable"), "a composable function carries Composable")

	preview := byID["org.acme.feature.LoginPreview"]
	require.NotNil(t, preview)
	assert.True(t, preview.HasMarker("Preview"), "arguments are not part of the key")
	assert.True(t, preview.HasMarker("Composable"), "a qualified annotation is keyed by its simple name")

	vm := byID["org.acme.feature.LoginViewModel"]
	assert.True(t, vm.HasMarker("HiltViewModel"))
	assert.True(t, vm.HasMarker("Inject"), "an injected constructor marks its class")
	assert.False(t, vm.HasMarker("VisibleForTesting"), "a method's annotation is the method's")
	assert.True(t, byID["org.acme.feature.LoginViewModel.reset"].HasMarker("VisibleForTesting"))

	test := byID["org.acme.feature.LoginTest"]
	assert.True(t, test.HasMarker("RunWith"))
	assert.False(t, test.HasMarker("Named"), "a parameter's annotation marks no declaration")
	assert.False(t, test.HasMarker("param:Named"))
	assert.True(t, byID["org.acme.feature.LoginTest.opens"].HasMarker("Test"))

	assert.Empty(t, byID["org.acme.feature.NiaButtonDefaults"].Markers, "nothing leaks onto the next declaration")
}

func TestExpectAndActualAreKeywords(t *testing.T) {
	byID := unitsIn(t, "shared/src/commonMain/kotlin/Platform.kt", "package app\nexpect class Platform()\nexpect fun platformName(): String\n")
	assert.True(t, byID["app.Platform"].HasMarker("expect"))
	assert.True(t, byID["app.platformName"].HasMarker("expect"))
	actual := unitsIn(t, "shared/src/iosMain/kotlin/Platform.ios.kt", "package app\nactual class Platform actual constructor()\n")
	assert.True(t, actual["app.Platform"].HasMarker("actual"))
}
