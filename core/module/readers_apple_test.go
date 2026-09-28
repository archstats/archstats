package module

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const packageSwift = `// swift-tools-version: 5.9
import PackageDescription

var package = Package(
  name: "Timeline",
  dependencies: [
    .package(name: "Models", path: "../Models"),
    .package(url: "https://github.com/siteline/swiftui-introspect", exact: "26.0.1"),
  ],
  targets: [
    .target(
      name: "Timeline",
      dependencies: [
        .product(name: "Models", package: "Models"),
        .product(name: "SwiftUIIntrospect", package: "SwiftUI-Introspect"),
        .target(name: "TimelineCore"),
        "Env",
      ]
    ),
    // .target(name: "Commented"),
    .target(name: "TimelineCore", path: "Sources/Core"),
    .testTarget(name: "TimelineTests", dependencies: ["Timeline"]),
  ]
)
package.targets.append(contentsOf: [
  .executableTarget(name: "server", dependencies: [.byName(name: "Timeline")]),
])
`

func TestSwiftPM_Targets(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "Packages/Timeline/Package.swift", packageSwift)
	m := Read(root)
	byName := map[string]*Module{}
	for _, mod := range m.Modules() {
		byName[mod.Name] = mod
	}
	require.Len(t, byName, 4, "a .target(name:) in a dependency list or a comment declares nothing")
	tl := byName["Timeline"]
	assert.Equal(t, "swiftpm", tl.Kind)
	assert.Equal(t, "library", tl.Type)
	assert.Equal(t, "Packages/Timeline/Sources/Timeline", tl.Dir)
	assert.Equal(t, []string{"Models", "SwiftUIIntrospect", "TimelineCore", "Env"}, tl.DependsOn, "a product's package name is not a module")
	assert.Equal(t, "Packages/Timeline/Sources/Core", byName["TimelineCore"].Dir)
	assert.Equal(t, "Packages/Timeline/Tests/TimelineTests", byName["TimelineTests"].Dir)
	assert.Equal(t, "test", byName["TimelineTests"].Type)
	assert.Equal(t, "executable", byName["server"].Type)
	assert.Equal(t, []string{"Timeline"}, byName["server"].DependsOn)
}

const pbxproj = `// !$*UTF8*$!
{
	archiveVersion = 1;
	objects = {
		F1 /* App.swift in Sources */ = {isa = PBXBuildFile; fileRef = R1 /* App.swift */; };
		F2 /* Feed.swift in Sources */ = {isa = PBXBuildFile; fileRef = R2 /* Feed.swift */; };
		R1 /* App.swift */ = {isa = PBXFileReference; lastKnownFileType = sourcecode.swift; path = App.swift; sourceTree = "<group>"; };
		R2 /* Feed.swift */ = {isa = PBXFileReference; path = "Feed/Feed.swift"; sourceTree = "<group>"; };
		UIKIT = {isa = PBXFileReference; path = UIKit.framework; sourceTree = SDKROOT; };
		G0 = {isa = PBXGroup; children = (G1, G2, UIKIT, ); sourceTree = "<group>"; };
		G1 /* MyApp */ = {isa = PBXGroup; children = (R1, R2, ); path = MyApp; sourceTree = "<group>"; };
		G2 /* Widget */ = {isa = PBXFileSystemSynchronizedRootGroup; path = Widget; sourceTree = "<group>"; };
		S1 /* Sources */ = {isa = PBXSourcesBuildPhase; files = (F1, F2, ); };
		T1 /* MyApp */ = {
			isa = PBXNativeTarget;
			buildPhases = (S1 /* Sources */, );
			dependencies = (D1, );
			name = MyApp;
			packageProductDependencies = (P1 /* Timeline */, );
			productType = "com.apple.product-type.application";
		};
		T2 /* Widget */ = {isa = PBXNativeTarget; buildPhases = (); fileSystemSynchronizedGroups = (G2, ); name = WidgetExtension; productType = "com.apple.product-type.app-extension"; };
		D1 = {isa = PBXTargetDependency; target = T2 /* Widget */; };
		P1 /* Timeline */ = {isa = XCSwiftPackageProductDependency; productName = Timeline; };
		PRJ = {isa = PBXProject; mainGroup = G0; targets = (T1, T2, ); };
	};
	rootObject = PRJ;
}
`

func TestXcode_Targets(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "ios/MyApp.xcodeproj/project.pbxproj", pbxproj)
	m := Read(root)
	byName := map[string]*Module{}
	for _, mod := range m.Modules() {
		byName[mod.Name] = mod
	}
	require.Contains(t, byName, "MyApp")
	app := byName["MyApp"]
	assert.Equal(t, "xcode", app.Kind)
	assert.Equal(t, "ios-application", app.Type)
	assert.Equal(t, []string{"ios/MyApp/App.swift", "ios/MyApp/Feed/Feed.swift"}, app.Files, "paths resolve through the group tree")
	assert.Equal(t, "ios/MyApp", app.Dir)
	assert.Equal(t, []string{"WidgetExtension", "Timeline"}, app.DependsOn, "target dependencies and package products")

	w := byName["WidgetExtension"]
	require.NotNil(t, w)
	assert.Equal(t, "app-extension", w.Type)
	assert.Equal(t, "ios/Widget", w.Dir, "a synchronised folder is the target's directory")
	assert.Equal(t, "WidgetExtension", m.NameOf("ios/Widget/Provider.swift"))
	assert.Equal(t, "MyApp", m.NameOf("ios/MyApp/Feed/Feed.swift"))
}

// Probe against a real checkout: ARCHSTATS_MODULE_PROBE=/path go test -run Probe -v
func TestProbe(t *testing.T) {
	dir := os.Getenv("ARCHSTATS_MODULE_PROBE")
	if dir == "" {
		t.Skip()
	}
	for _, mod := range Read(dir).Modules() {
		t.Logf("%-8s %-18s %-40s %-50s files=%d deps=%v", mod.Kind, mod.Type, mod.Name, mod.Dir, len(mod.Files), mod.DependsOn)
	}
}

func TestPub(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "mobile/pubspec.yaml", "name: immich_mobile\ndependencies:\n  flutter:\n    sdk: flutter\n  hooks_riverpod: ^2.4.0\n  openapi:\n    path: openapi\ndev_dependencies:\n  build_runner: any\n")
	writeFile(t, root, "mobile/lib/main.dart", "void main() {}")
	writeFile(t, root, "mobile/openapi/pubspec.yaml", "name: openapi\ndependencies:\n  http: any\n")
	m := Read(root)
	app := m.ByName("immich_mobile")
	require.NotNil(t, app)
	assert.Equal(t, "pub", app.Kind)
	assert.Equal(t, "flutter-app", app.Type)
	assert.Equal(t, "mobile", app.Dir)
	assert.Equal(t, []string{"flutter", "hooks_riverpod", "openapi", "build_runner"}, app.DependsOn)
	assert.Equal(t, "dart-package", m.ByName("openapi").Type)
	assert.Equal(t, "openapi", m.NameOf("mobile/openapi/lib/api.dart"))
}

// Signal applies precompiled script plugins; Thunderbird names its plugins
// through constants. Neither says "android" where the plugin is applied.
func TestGradle_ConventionPluginTypes(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "settings.gradle.kts", ``)
	writeFile(t, root, "build-logic/plugins/src/main/java/signal-library.gradle.kts", "plugins {\n  id(\"com.android.library\")\n  id(\"kotlin-android\")\n}\n")
	writeFile(t, root, "core/util/build.gradle.kts", "plugins {\n  id(\"signal-library\")\n  id(\"kotlin-parcelize\")\n}\n")
	writeFile(t, root, "app-thunderbird/build.gradle.kts", "plugins {\n    id(ThunderbirdPlugins.App.androidCompose)\n}\n")
	writeFile(t, root, "feature/mail/build.gradle.kts", "plugins {\n    id(ThunderbirdPlugins.Library.androidCompose)\n}\n")
	writeFile(t, root, "core/common/build.gradle.kts", "plugins {\n    id(ThunderbirdPlugins.Library.kmp)\n}\n")
	m := Read(root)
	typeOf := func(dir string) string {
		for _, mod := range m.Modules() {
			if mod.Dir == dir {
				return mod.Type
			}
		}
		return "(missing)"
	}
	assert.Equal(t, "android-library", typeOf("core/util"))
	assert.Equal(t, "android-application", typeOf("app-thunderbird"))
	assert.Equal(t, "android-library", typeOf("feature/mail"))
	assert.Equal(t, "kotlin-multiplatform", typeOf("core/common"))
}
