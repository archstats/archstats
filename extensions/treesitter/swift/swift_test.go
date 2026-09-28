package swift

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/archstats/archstats/core/file"
	"github.com/archstats/archstats/core/unit"
	"github.com/archstats/archstats/extensions/treesitter/apple"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const pkg = `// swift-tools-version: 5.9
import PackageDescription
let package = Package(name: "App", targets: [
  .target(name: "Models"),
  .target(name: "Timeline", dependencies: ["Models"]),
])
`

var sources = map[string]string{
	"Sources/Models/Status.swift": `import Foundation
public struct Status: Codable, Identifiable { public let id: String }
public protocol Client: AnyObject { func get() async throws -> [Status] }
`,
	"Sources/Timeline/Views/TimelineView.swift": `import SwiftUI
import Models

struct TimelineView: View {
  @State private var vm = TimelineViewModel()
  @Environment(Theme.self) var theme
  var body: some View { List(vm.items) { StatusRow(status: $0) } }
}
`,
	"Sources/Timeline/ViewModels/TimelineViewModel.swift": `import Observation
import Models

@MainActor
@Observable
final class TimelineViewModel {
  var items: [Status] = []
  static let shared = TimelineViewModel()
  @MainActor func fetch(client: Client) async {}
}

extension TimelineViewModel: Sendable {
  func reset() {}
}

extension View {
  func card() -> some View { self }
}
`,
	"Sources/Timeline/Rows/StatusRow.swift": `import SwiftUI
import Models
struct StatusRow: View { let status: Status; var body: some View { Text(status.id) } }
struct Card<Content: View>: View { let content: Content; var body: some View { content } }
`,
	"Sources/Timeline/Content/Content.swift": `struct Content {}
struct Feature { struct State {}; func run(state: State) {} }
`,
	"Sources/Timeline/Other/Other.swift": `struct Other { struct State {}; var state: State }
`,
}

func analyse(t *testing.T) (map[string]*file.Results, map[string]*unit.Unit) {
	t.Helper()
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "Package.swift"), []byte(pkg), 0o644))
	lp := createSwiftLanguagePack()
	a := &swiftAnalyzer{lp: lp}
	var all []*file.Results
	byName := map[string]*file.Results{}
	for p, src := range sources {
		full := filepath.Join(root, p)
		require.NoError(t, os.MkdirAll(filepath.Dir(full), 0o755))
		require.NoError(t, os.WriteFile(full, []byte(src), 0o644))
		res := a.analyze(p, []byte(src))
		require.NotNil(t, res, p)
		res.Name = p
		res.Directory = filepath.Dir(p)
		for _, s := range res.Snippets {
			s.Component = res.Directory
		}
		all = append(all, res)
		byName[p] = res
	}
	// Every walked file has results, the manifest among them.
	all = append(all, &file.Results{Name: "./Package.swift", Directory: "."})
	(&apple.Linker{Root: root}).EditFileResults(all)
	units := map[string]*unit.Unit{}
	for _, fr := range all {
		for _, u := range fr.Units {
			if existing, ok := units[u.ID]; ok {
				existing.Markers = append(existing.Markers, u.Markers...)
				existing.Refs = append(existing.Refs, u.Refs...)
				continue
			}
			units[u.ID] = u
		}
	}
	return byName, units
}

func TestUnitsAreNamedByTheirTarget(t *testing.T) {
	_, units := analyse(t)
	require.Contains(t, units, "Models#Status")
	require.Contains(t, units, "Timeline#TimelineView")
	require.Contains(t, units, "Timeline#TimelineViewModel")
	assert.Equal(t, "Timeline#TimelineViewModel", units["Timeline#TimelineViewModel.fetch"].Owner)
	assert.Equal(t, "Timeline#TimelineViewModel", units["Timeline#TimelineViewModel.reset"].Owner, "an extension's method belongs to the type it extends")
	assert.NotContains(t, units, "Timeline#View", "an extension of a platform type declares nothing")
	assert.Equal(t, "Timeline#View", units["Timeline#View.card"].Owner)
}

func TestMarkers(t *testing.T) {
	_, units := analyse(t)
	status := units["Models#Status"]
	assert.True(t, status.HasMarker("struct"))
	assert.True(t, status.HasMarker("Codable"))
	assert.True(t, status.HasMarker("Identifiable"))
	assert.True(t, units["Models#Client"].HasMarker("interface"), "a protocol is an interface")

	vm := units["Timeline#TimelineViewModel"]
	assert.True(t, vm.HasMarker("Observable"))
	assert.True(t, vm.HasMarker("MainActor"))
	assert.True(t, vm.HasMarker("Sendable"), "a conformance added by an extension is the type's")
	assert.False(t, vm.HasMarker("extension"), "the type is declared in its target, so it is not merely extended")
	assert.True(t, units["Timeline#TimelineViewModel.fetch"].HasMarker("MainActor"))

	view := units["Timeline#TimelineView"]
	assert.True(t, view.HasMarker("View"))
	assert.True(t, view.HasMarker("State"), "a property wrapper marks the type holding it")
	assert.True(t, view.HasMarker("Environment"))
}

func TestReferencesResolveWithinTheTargetAndItsImports(t *testing.T) {
	_, units := analyse(t)
	refs := map[string]bool{}
	for _, r := range units["Timeline#TimelineView"].Refs {
		refs[r.Module+"#"+r.Name] = true
	}
	assert.True(t, refs["Timeline#TimelineViewModel"], "the same target, another directory, no import")
	assert.True(t, refs["Timeline#StatusRow"])
	assert.False(t, refs["Timeline#View"], "the platform's types resolve to nothing")
	vmRefs := map[string]bool{}
	for _, r := range units["Timeline#TimelineViewModel"].Refs {
		vmRefs[r.Module+"#"+r.Name] = true
	}
	assert.True(t, vmRefs["Models#Status"], "an imported target's type")
}

func TestComponentEdgesComeFromTypeReferences(t *testing.T) {
	files, _ := analyse(t)
	edges := func(p string) []string {
		var out []string
		for _, s := range files[p].Snippets {
			if s.Type == file.ComponentImport {
				out = append(out, s.Value)
			}
			assert.NotEqual(t, captureRef, s.Type, "reference captures are removed")
		}
		return out
	}
	assert.ElementsMatch(t, []string{"Sources/Timeline/ViewModels", "Sources/Timeline/Rows"}, edges("Sources/Timeline/Views/TimelineView.swift"))
	assert.ElementsMatch(t, []string{"Sources/Models"}, edges("Sources/Timeline/ViewModels/TimelineViewModel.swift"))
	assert.Empty(t, edges("Sources/Timeline/Rows/StatusRow.swift")[1:], "a generic parameter named Content is not the type called Content")
	assert.Empty(t, edges("Sources/Timeline/Other/Other.swift"), "a nested State means its own, not another type's")
}
