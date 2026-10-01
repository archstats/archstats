package swift

import (
	"os"
	"path"
	"path/filepath"
	"sort"
	"testing"

	"github.com/archstats/archstats/core/file"
	"github.com/archstats/archstats/core/unit"
	"github.com/archstats/archstats/extensions/treesitter/apple"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The mini-apps the UI's iOS and TCA lanes are tested against
// (archstats-ui: features/frameworks/layers.mobile.test.ts). What is asserted
// here -- units, owners, markers by source, raw imports, references -- is
// exactly what that test feeds the profiles, so the two stay in step.

func analyseApp(t *testing.T, manifest string, files map[string]string) (map[string]*file.Results, map[string]*unit.Unit) {
	t.Helper()
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "Package.swift"), []byte(manifest), 0o644))
	a := &swiftAnalyzer{lp: createSwiftLanguagePack()}
	var all []*file.Results
	byName := map[string]*file.Results{}
	for p, src := range files {
		full := filepath.Join(root, p)
		require.NoError(t, os.MkdirAll(filepath.Dir(full), 0o755))
		require.NoError(t, os.WriteFile(full, []byte(src), 0o644))
		res := a.analyze(p, []byte(src))
		require.NotNil(t, res, p)
		res.Name = p
		res.Directory = path.Dir(p) // slash paths, as the analyzer sets them
		for _, s := range res.Snippets {
			s.Component = res.Directory
		}
		all = append(all, res)
		byName[p] = res
	}
	all = append(all, &file.Results{Name: "./Package.swift", Directory: "."})
	(&apple.Linker{Root: root}).EditFileResults(all)
	var raw []*unit.Unit
	for _, fr := range all {
		raw = append(raw, fr.Units...)
	}
	units := map[string]*unit.Unit{}
	for _, u := range unit.Merge(raw) {
		units[u.ID] = u
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

func refIDs(u *unit.Unit) []string {
	var out []string
	for _, r := range u.Refs {
		out = append(out, r.Module+"#"+r.Name)
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

func rawImports(fr *file.Results) []string {
	var out []string
	for _, s := range fr.Snippets {
		if s.Type == file.ImportRaw {
			out = append(out, s.Value)
		}
	}
	return out
}

const iosPackage = `// swift-tools-version: 5.9
import PackageDescription
let package = Package(name: "IceCubes", targets: [
  .target(name: "Models"),
  .target(name: "Network", dependencies: ["Models"]),
  .target(name: "App", dependencies: ["Models", "Network"]),
  .testTarget(name: "AppTests", dependencies: ["App"]),
])
`

// A SwiftUI app with a UIKit corner: screens named for being one, rows that
// gain their View conformance in an extension, an ObservableObject view
// model, an @Observable store, a protocol-only repository, a client behind
// it, value-type models, SwiftData and Core Data records.
var iosApp = map[string]string{
	"Sources/Models/Status.swift": `import Foundation
public struct Status: Codable, Identifiable, Sendable {
  public let id: String
  public let content: String
  public let visibility: Visibility
}
public enum Visibility: String, Codable { case pub, direct }
`,
	"Sources/Models/Account.swift": `import Foundation
public final class Account: Codable {
  public let id: String
  public var statuses: [Status] = []
}
`,
	"Sources/Models/Storage/Trip.swift": `import SwiftData
@Model final class Trip {
  var name: String
  init(name: String) { self.name = name }
}
`,
	"Sources/Models/Storage/StatusEntity.swift": `import CoreData
final class StatusEntity: NSManagedObject {
  @NSManaged var id: String
}
`,
	"Sources/Network/Client.swift": `import Models
public protocol Client: Sendable {
  func get(_ endpoint: Endpoint) async throws -> [Status]
}
`,
	"Sources/Network/MastodonClient.swift": `import Foundation
import Models

public enum Endpoint { case timeline, account(String) }

public final class MastodonClient: Client {
  let session: URLSession
  public init(session: URLSession = .shared) { self.session = session }
  public func get(_ endpoint: Endpoint) async throws -> [Status] { [] }
}
`,
	"Sources/App/IceCubesApp.swift": `import SwiftUI
import Models

@main
struct IceCubesApp: App {
  @UIApplicationDelegateAdaptor(AppDelegate.self) var delegate
  var body: some Scene { WindowGroup { TimelineScreen() } }
}
`,
	"Sources/App/AppDelegate.swift": `import UIKit
final class AppDelegate: UIResponder, UIApplicationDelegate {
  func application(_ application: UIApplication, didFinishLaunchingWithOptions options: [UIApplication.LaunchOptionsKey: Any]? = nil) -> Bool { true }
}
`,
	"Sources/App/Timeline/TimelineScreen.swift": `import SwiftUI
import Models

struct TimelineScreen: View {
  @StateObject private var viewModel = TimelineViewModel()
  var body: some View {
    List(viewModel.statuses) { status in StatusRow(status: status) }
      .task { await viewModel.load() }
  }
}
`,
	"Sources/App/Timeline/TimelineViewModel.swift": `import Foundation
import Models
import Network

@MainActor
final class TimelineViewModel: ObservableObject {
  @Published var statuses: [Status] = []
  private let repository: StatusRepository
  init(repository: StatusRepository = DefaultStatusRepository()) { self.repository = repository }
  func load() async { statuses = (try? await repository.timeline()) ?? [] }
}
`,
	"Sources/App/Timeline/StatusRow.swift": `import Models
struct StatusRow {
  let status: Status
}
`,
	"Sources/App/Timeline/StatusRow+View.swift": `import SwiftUI
extension StatusRow: View {
  var body: some View { Text(status.content) }
}
`,
	"Sources/App/Data/StatusRepository.swift": `import Models
protocol StatusRepository: Sendable {
  func timeline() async throws -> [Status]
}
`,
	"Sources/App/Data/DefaultStatusRepository.swift": `import Models
import Network

final class DefaultStatusRepository: StatusRepository {
  private let client: MastodonClient
  init(client: MastodonClient = MastodonClient()) { self.client = client }
  func timeline() async throws -> [Status] { try await client.get(.timeline) }
  // Planted: a repository that knows a screen.
  func screen() -> TimelineScreen { TimelineScreen() }
}
`,
	"Sources/App/Profile/ProfileStore.swift": `import Observation
import Models

@Observable
final class ProfileStore {
  var account: Account?
  var isLoading = false
}
`,
	"Sources/App/Settings/SettingsViewController.swift": `import UIKit
final class SettingsViewController: UIViewController, UITableViewDataSource {
  override func viewDidLoad() { super.viewDidLoad(); tableView.register(SettingsCell.self) }
  func tableView(_ tableView: UITableView, numberOfRowsInSection section: Int) -> Int { 1 }
}
`,
	"Sources/App/Settings/SettingsCell.swift": `import UIKit
final class SettingsCell: UITableViewCell {}
`,
	// Planted: a model that reaches back up to a view model, written where it
	// can compile, as an extension in the App target.
	"Sources/App/Timeline/Status+ViewModel.swift": `import Models
extension Status {
  var viewModel: TimelineViewModel { TimelineViewModel() }
}
`,
	"Tests/AppTests/TimelineViewModelTests.swift": `import XCTest
@testable import App

final class TimelineViewModelTests: XCTestCase {
  func testLoad() async { let vm = TimelineViewModel(); await vm.load() }
}
`,
}

func TestIOSUnitsAndTheirMarkers(t *testing.T) {
	files, units := analyseApp(t, iosPackage, iosApp)

	status := units["Models#Status"]
	require.NotNil(t, status)
	assert.Equal(t, unit.KindType, status.Kind)
	assert.Equal(t, []string{"struct"}, markers(status, apple.SourceKeyword))
	assert.Equal(t, []string{"Codable", "Identifiable", "Sendable"}, markers(status, unit.SourceSupertype))
	assert.Equal(t, []string{"enum"}, markers(units["Models#Visibility"], apple.SourceKeyword))
	assert.Equal(t, []string{"class"}, markers(units["Models#Account"], apple.SourceKeyword))
	assert.Equal(t, []string{"Codable"}, markers(units["Models#Account"], unit.SourceSupertype))

	assert.Equal(t, []string{"Model"}, markers(units["Models#Trip"], unit.SourceAnnotation), "the SwiftData macro is an attribute")
	assert.Equal(t, []string{"NSManagedObject"}, markers(units["Models#StatusEntity"], unit.SourceSupertype))
	assert.Equal(t, []string{"NSManaged"}, markers(units["Models#StatusEntity"], unit.SourceAnnotation), "a property wrapper marks the type holding it")

	client := units["Network#Client"]
	require.NotNil(t, client, "a protocol-only file declares a unit")
	assert.Equal(t, []string{"protocol"}, markers(client, apple.SourceKeyword))
	assert.Equal(t, []string{"Sendable", "interface"}, markers(client, unit.SourceSupertype))
	assert.Equal(t, []string{"Client"}, markers(units["Network#MastodonClient"], unit.SourceSupertype))

	app := units["App#IceCubesApp"]
	assert.Equal(t, []string{"UIApplicationDelegateAdaptor", "main"}, markers(app, unit.SourceAnnotation))
	assert.Equal(t, []string{"App"}, markers(app, unit.SourceSupertype))
	assert.Equal(t, []string{"UIApplicationDelegate", "UIResponder"}, markers(units["App#AppDelegate"], unit.SourceSupertype))

	screen := units["App#TimelineScreen"]
	assert.Equal(t, []string{"View"}, markers(screen, unit.SourceSupertype))
	assert.Equal(t, []string{"StateObject"}, markers(screen, unit.SourceAnnotation))

	vm := units["App#TimelineViewModel"]
	assert.Equal(t, []string{"ObservableObject"}, markers(vm, unit.SourceSupertype))
	assert.Equal(t, []string{"MainActor", "Published"}, markers(vm, unit.SourceAnnotation))
	assert.Equal(t, "App#TimelineViewModel", units["App#TimelineViewModel.load"].Owner)

	row := units["App#StatusRow"]
	require.NotNil(t, row)
	assert.Equal(t, []string{"View"}, markers(row, unit.SourceSupertype), "a conformance added in another file is the type's")
	assert.Equal(t, []string{"struct"}, markers(row, apple.SourceKeyword), "an extension of a type declared in the target is not merely an extension")
	assert.ElementsMatch(t, []string{"Sources/App/Timeline/StatusRow.swift", "Sources/App/Timeline/StatusRow+View.swift"}, row.Files)

	assert.Equal(t, []string{"Observable"}, markers(units["App#ProfileStore"], unit.SourceAnnotation))
	assert.Equal(t, []string{"Sendable", "interface"}, markers(units["App#StatusRepository"], unit.SourceSupertype))
	assert.Equal(t, []string{"StatusRepository"}, markers(units["App#DefaultStatusRepository"], unit.SourceSupertype))
	assert.Equal(t, []string{"UITableViewDataSource", "UIViewController"}, markers(units["App#SettingsViewController"], unit.SourceSupertype))
	assert.Equal(t, []string{"UITableViewCell"}, markers(units["App#SettingsCell"], unit.SourceSupertype))
	assert.Equal(t, []string{"XCTestCase"}, markers(units["AppTests#TimelineViewModelTests"], unit.SourceSupertype), "a test target's units are named by it")

	// The imports detection reads, framework names as written.
	assert.Equal(t, []string{"SwiftUI", "Models"}, rawImports(files["Sources/App/Timeline/TimelineScreen.swift"]))
	assert.Equal(t, []string{"Foundation", "Models", "Network"}, rawImports(files["Sources/App/Timeline/TimelineViewModel.swift"]))
	assert.Equal(t, []string{"UIKit"}, rawImports(files["Sources/App/Settings/SettingsViewController.swift"]))
	assert.Equal(t, []string{"SwiftData"}, rawImports(files["Sources/Models/Storage/Trip.swift"]))
	assert.Equal(t, []string{"CoreData"}, rawImports(files["Sources/Models/Storage/StatusEntity.swift"]))
	assert.Equal(t, []string{"XCTest", "App"}, rawImports(files["Tests/AppTests/TimelineViewModelTests.swift"]), "@testable import names the module all the same")
}

func TestIOSReferencesRunBetweenTheLanes(t *testing.T) {
	_, units := analyseApp(t, iosPackage, iosApp)
	// The App type reaches its first screen and its delegate.
	assert.Equal(t, []string{"App#AppDelegate", "App#TimelineScreen"}, refIDs(units["App#IceCubesApp"]))
	// Screen -> view model, and the row it lists; the model it lists comes
	// through the row's parameter, not a name the screen writes.
	assert.Equal(t, []string{"App#StatusRow", "App#TimelineViewModel"}, refIDs(units["App#TimelineScreen"]))
	// View model -> repository (own target) and the model (an imported target).
	assert.Equal(t, []string{"App#DefaultStatusRepository", "App#StatusRepository", "Models#Status"}, refIDs(units["App#TimelineViewModel"]))
	assert.Equal(t, []string{"Models#Status"}, refIDs(units["App#StatusRow"]))
	// Repository -> client, the model, and the planted screen.
	assert.Equal(t, []string{"App#StatusRepository", "App#TimelineScreen", "Models#Status", "Network#MastodonClient"}, rolledUp(units, "App#DefaultStatusRepository"))
	assert.Equal(t, []string{"Models#Status", "Network#Client", "Network#Endpoint"}, rolledUp(units, "Network#MastodonClient"))
	assert.Equal(t, []string{"App#SettingsCell"}, rolledUp(units, "App#SettingsViewController"), "UIKit's own types resolve to nothing")
	assert.Equal(t, []string{"Models#Account"}, refIDs(units["App#ProfileStore"]))
	assert.Equal(t, []string{"App#TimelineViewModel"}, refIDs(units["AppTests#TimelineViewModelTests.testLoad"]), "a test target sees the target it imports")

	// The planted model -> view model reference, written as an extension in
	// the App target, belongs to Status itself: an extension of an imported
	// target's type is more of that type, as one of the same target's is.
	assert.Contains(t, rolledUp(units, "Models#Status"), "App#TimelineViewModel")
	assert.Contains(t, units["Models#Status"].Files, "Sources/App/Timeline/Status+ViewModel.swift")
	assert.NotContains(t, units, "App#Status", "the extension declares no type of the App target's")
}

const tcaPackage = `// swift-tools-version: 5.9
import PackageDescription
let package = Package(name: "Counter", targets: [
  .target(name: "App"),
  .testTarget(name: "AppTests", dependencies: ["App"]),
])
`

// A TCA feature with its nested State and Action, the view that renders its
// store, a dependency client, a model, and a plain SwiftUI corner with an
// ObservableObject, as most TCA apps have somewhere.
var tcaApp = map[string]string{
	"Sources/App/Features/CounterFeature.swift": `import ComposableArchitecture
import Foundation

@Reducer
struct CounterFeature {
  @ObservableState
  struct State: Equatable {
    var count = 0
    var fact: Fact?
  }
  enum Action: BindableAction {
    case binding(BindingAction<State>)
    case incrementTapped
    case factResponse(Fact)
  }
  @Dependency(NumberFactClient.self) var numberFact
  var body: some ReducerOf<Self> {
    BindingReducer()
    Reduce { state, action in
      switch action {
      case .incrementTapped:
        state.count += 1
        return .run { [count = state.count] send in await send(.factResponse(try await numberFact.fetch(count))) }
      case let .factResponse(fact):
        state.fact = fact
        return .none
      case .binding:
        return .none
      }
    }
  }
}
`,
	"Sources/App/Features/CounterView.swift": `import ComposableArchitecture
import SwiftUI

struct CounterView: View {
  @Bindable var store: StoreOf<CounterFeature>
  var body: some View {
    VStack {
      Text("\(store.count)")
      Button("+") { store.send(.incrementTapped) }
      if let fact = store.fact { FactRow(fact: fact) }
    }
  }
}

struct FactRow: View {
  let fact: Fact
  var body: some View { Text(fact.text) }
}
`,
	"Sources/App/Dependencies/NumberFactClient.swift": `import ComposableArchitecture
import Foundation

@DependencyClient
struct NumberFactClient {
  var fetch: @Sendable (Int) async throws -> Fact
}

extension NumberFactClient: DependencyKey {
  static let liveValue = NumberFactClient(fetch: { n in Fact(text: "\(n) is a number") })
}

extension DependencyValues {
  var numberFact: NumberFactClient {
    get { self[NumberFactClient.self] }
    set { self[NumberFactClient.self] = newValue }
  }
}
`,
	"Sources/App/Models/Fact.swift": `import Foundation
struct Fact: Equatable, Codable {
  let text: String
}
`,
	"Sources/App/CounterApp.swift": `import ComposableArchitecture
import SwiftUI

@main
struct CounterApp: App {
  static let store = Store(initialState: CounterFeature.State()) { CounterFeature() }
  var body: some Scene { WindowGroup { CounterView(store: Self.store) } }
}
`,
	"Sources/App/Settings/SettingsView.swift": `import SwiftUI

struct SettingsView: View {
  @StateObject private var model = SettingsModel()
  var body: some View { Toggle("Dark", isOn: $model.dark) }
}

final class SettingsModel: ObservableObject {
  @Published var dark = false
}
`,
	"Tests/AppTests/CounterFeatureTests.swift": `import ComposableArchitecture
import XCTest
@testable import App

@MainActor
final class CounterFeatureTests: XCTestCase {
  func testIncrement() async {
    let store = TestStore(initialState: CounterFeature.State()) { CounterFeature() }
    await store.send(.incrementTapped) { $0.count = 1 }
  }
}
`,
}

func TestTCAFeaturesOwnTheirStateAndAction(t *testing.T) {
	files, units := analyseApp(t, tcaPackage, tcaApp)

	feature := units["App#CounterFeature"]
	require.NotNil(t, feature)
	assert.Equal(t, []string{"Dependency", "Reducer"}, markers(feature, unit.SourceAnnotation), "the macro, and the wrapper on its dependency")
	assert.Empty(t, markers(feature, unit.SourceSupertype))

	state := units["App#CounterFeature.State"]
	require.NotNil(t, state, "the nested State is a unit of its own")
	assert.Equal(t, unit.KindType, state.Kind)
	assert.Equal(t, "State", state.Name)
	assert.Equal(t, "App#CounterFeature", state.Owner)
	assert.Equal(t, []string{"ObservableState"}, markers(state, unit.SourceAnnotation), "the macro on State is State's, not the reducer's")
	assert.Equal(t, []string{"Equatable"}, markers(state, unit.SourceSupertype))

	action := units["App#CounterFeature.Action"]
	require.NotNil(t, action)
	assert.Equal(t, "App#CounterFeature", action.Owner)
	assert.Equal(t, []string{"BindableAction"}, markers(action, unit.SourceSupertype))

	// What the feature uses: its dependency by name, the model through its
	// State and Action. A member's references roll up to the feature in
	// the UI.
	assert.Equal(t, []string{"App#NumberFactClient"}, refIDs(feature))
	assert.Equal(t, []string{"App#Fact"}, refIDs(state))
	assert.Equal(t, []string{"App#CounterFeature.State", "App#Fact"}, refIDs(action), "a bare State inside the feature is the feature's own")
	assert.Equal(t, []string{"App#CounterFeature.State", "App#Fact", "App#NumberFactClient"}, rolledUp(units, "App#CounterFeature"))

	view := units["App#CounterView"]
	assert.Equal(t, []string{"View"}, markers(view, unit.SourceSupertype))
	assert.Equal(t, []string{"Bindable"}, markers(view, unit.SourceAnnotation))
	assert.Equal(t, []string{"App#CounterFeature", "App#FactRow"}, refIDs(view))
	assert.Equal(t, []string{"App#Fact"}, refIDs(units["App#FactRow"]))

	client := units["App#NumberFactClient"]
	assert.Equal(t, []string{"DependencyClient"}, markers(client, unit.SourceAnnotation))
	assert.Equal(t, []string{"DependencyKey"}, markers(client, unit.SourceSupertype), "the conformance comes from an extension")
	assert.Equal(t, []string{"App#Fact"}, refIDs(client))
	assert.NotContains(t, units, "App#DependencyValues", "extending the library's DependencyValues declares nothing")

	assert.Equal(t, []string{"struct"}, markers(units["App#Fact"], apple.SourceKeyword))
	assert.Equal(t, []string{"Codable", "Equatable"}, markers(units["App#Fact"], unit.SourceSupertype))

	assert.Equal(t, []string{"App"}, markers(units["App#CounterApp"], unit.SourceSupertype))
	assert.Equal(t, []string{"main"}, markers(units["App#CounterApp"], unit.SourceAnnotation))
	assert.Equal(t, []string{"App#CounterFeature", "App#CounterFeature.State", "App#CounterView"}, refIDs(units["App#CounterApp"]))

	// The plain SwiftUI corner reads as it would without TCA.
	assert.Equal(t, []string{"ObservableObject"}, markers(units["App#SettingsModel"], unit.SourceSupertype))
	assert.Equal(t, []string{"Published"}, markers(units["App#SettingsModel"], unit.SourceAnnotation))
	assert.Equal(t, []string{"App#SettingsModel"}, refIDs(units["App#SettingsView"]))

	assert.Equal(t, []string{"ComposableArchitecture", "Foundation"}, rawImports(files["Sources/App/Features/CounterFeature.swift"]))
	assert.Equal(t, []string{"ComposableArchitecture", "SwiftUI"}, rawImports(files["Sources/App/Features/CounterView.swift"]))
	assert.Equal(t, []string{"SwiftUI"}, rawImports(files["Sources/App/Settings/SettingsView.swift"]))

	test := units["AppTests#CounterFeatureTests"]
	require.NotNil(t, test)
	assert.Equal(t, []string{"XCTestCase"}, markers(test, unit.SourceSupertype))
	assert.Equal(t, []string{"App#CounterFeature", "App#CounterFeature.State"}, refIDs(units["AppTests#CounterFeatureTests.testIncrement"]))
}
