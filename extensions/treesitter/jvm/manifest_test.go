package jvm

import (
	"testing"

	"github.com/archstats/archstats/core/module"
	"github.com/archstats/archstats/core/module/moduletest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestManifestReaders(t *testing.T) {
	t.Run("gradle", func(t *testing.T) {
		// A gradle build script does not name itself; the directory does.
		moduletest.Check(t, "testdata/manifests/gradle", ManifestReaders(), moduletest.Want{
			Kind: "gradle", Count: 1,
			Modules: map[string]string{"core": "core"},
			Deps:    map[string]string{"core": "exposed-common"},
		})
	})
	t.Run("maven", func(t *testing.T) {
		moduletest.Check(t, "testdata/manifests/maven", ManifestReaders(), moduletest.Want{
			Kind: "maven", Count: 2,
			Modules: map[string]string{"broadleaf": "", "broadleaf-framework-web": "web"},
			Deps:    map[string]string{"broadleaf-framework-web": "broadleaf-framework"},
		})
	})
}

// A malformed pom declares nothing, and does not stop the rest being read.
func TestMaven_MalformedPomIsSkipped(t *testing.T) {
	root := t.TempDir()
	moduletest.Write(t, root, "bad/pom.xml", `<project><artifactId>unclosed`)
	assert.Equal(t, 0, module.Read(root, ManifestReaders()...).Len())
}

// The root build script of a multi-module gradle build configures the build
// rather than declaring a module anybody depends on.
func TestGradle_RootBuildScriptIsNotAModule(t *testing.T) {
	root := t.TempDir()
	moduletest.Write(t, root, "build.gradle.kts", `plugins { kotlin("jvm") }`)
	moduletest.Write(t, root, "core/build.gradle.kts", `dependencies { api(project(":other")) }`)

	m := module.Read(root, ManifestReaders()...)
	require.Equal(t, 1, m.Len())
	assert.Equal(t, "core", m.Modules()[0].Name)
}

// nowinandroid: 36 build scripts, nested two and three deep, depending on each
// other through type-safe accessors and applying convention plugins. Named by
// their last directory, its feature modules were all called "impl" and "api";
// read with `project(":x")` only, not one of its dependencies was declared.
func TestGradle_ProjectPathsAccessorsAndTypes(t *testing.T) {
	root := t.TempDir()
	moduletest.Write(t, root, "settings.gradle.kts", `include(":app")`)
	moduletest.Write(t, root, "build.gradle.kts", `plugins { alias(libs.plugins.android.application) apply false }`)
	moduletest.Write(t, root, "app/build.gradle.kts", `plugins {
    alias(libs.plugins.nowinandroid.android.application)
    alias(libs.plugins.nowinandroid.hilt)
}
dependencies {
    implementation(projects.feature.foryou.impl)
    implementation(projects.core.designsystem)
    testImplementation(projects.core.dataTest)
    androidTestImplementation(libs.androidx.test.core)
}`)
	moduletest.Write(t, root, "feature/foryou/impl/build.gradle.kts", `plugins { alias(libs.plugins.nowinandroid.android.feature.impl) }
dependencies { implementation(projects.core.data) }`)
	moduletest.Write(t, root, "feature/bookmarks/impl/build.gradle.kts", `plugins { id("nowinandroid.android.feature.impl") }`)
	moduletest.Write(t, root, "core/data/build.gradle.kts", `plugins { id("nowinandroid.android.library") }`)
	moduletest.Write(t, root, "core/data-test/build.gradle.kts", `plugins { id("nowinandroid.android.library") }
dependencies { api(project(":core:data")) }`)
	moduletest.Write(t, root, "shared/build.gradle.kts", `plugins { kotlin("multiplatform"); id("com.android.library") }`)
	moduletest.Write(t, root, "legacy/build.gradle", "apply plugin: 'com.android.application'\ndependencies { implementation project(':core:data') }")
	moduletest.Write(t, root, "build-logic/settings.gradle.kts", `rootProject.name = "build-logic"`)
	moduletest.Write(t, root, "build-logic/convention/build.gradle.kts", "plugins { `kotlin-dsl` }")

	m := module.Read(root, ManifestReaders()...)
	byDir := map[string]*module.Module{}
	for _, mod := range m.Modules() {
		byDir[mod.Dir] = mod
	}
	name := func(dir string) string {
		require.Containsf(t, byDir, dir, "%s is missing", dir)
		return byDir[dir].Name
	}
	assert.Equal(t, "app", name("app"))
	assert.Equal(t, "feature:foryou:impl", name("feature/foryou/impl"), "named by project path, not the last directory")
	assert.Equal(t, "feature:bookmarks:impl", name("feature/bookmarks/impl"))
	assert.Equal(t, "core:data-test", name("core/data-test"))
	assert.Equal(t, "convention", name("build-logic/convention"), "an included build's own settings file is its root")

	assert.ElementsMatch(t, []string{"feature:foryou:impl", "core:designsystem", "core:data-test"}, byDir["app"].DependsOn,
		"type-safe accessors resolve, camel case back to the kebab-case directory")
	assert.Equal(t, []string{"core:data"}, byDir["feature/foryou/impl"].DependsOn)
	assert.Equal(t, []string{"core:data"}, byDir["core/data-test"].DependsOn, "a nested project() path keeps every segment")
	assert.Equal(t, []string{"core:data"}, byDir["legacy"].DependsOn)

	assert.Equal(t, "android-application", byDir["app"].Type)
	assert.Equal(t, "android-application", byDir["legacy"].Type)
	assert.Equal(t, "android-library", byDir["feature/foryou/impl"].Type, "a convention plugin says what it applies")
	assert.Equal(t, "android-library", byDir["core/data"].Type)
	assert.Equal(t, "kotlin-multiplatform", byDir["shared"].Type)
	assert.Equal(t, "build-logic", byDir["build-logic/convention"].Type)
}

// Signal applies precompiled script plugins; Thunderbird names its plugins
// through constants. Neither says "android" where the plugin is applied.
func TestGradle_ConventionPluginTypes(t *testing.T) {
	root := t.TempDir()
	moduletest.Write(t, root, "settings.gradle.kts", ``)
	moduletest.Write(t, root, "build-logic/plugins/src/main/java/signal-library.gradle.kts", "plugins {\n  id(\"com.android.library\")\n  id(\"kotlin-android\")\n}\n")
	moduletest.Write(t, root, "core/util/build.gradle.kts", "plugins {\n  id(\"signal-library\")\n  id(\"kotlin-parcelize\")\n}\n")
	moduletest.Write(t, root, "app-thunderbird/build.gradle.kts", "plugins {\n    id(ThunderbirdPlugins.App.androidCompose)\n}\n")
	moduletest.Write(t, root, "feature/mail/build.gradle.kts", "plugins {\n    id(ThunderbirdPlugins.Library.androidCompose)\n}\n")
	moduletest.Write(t, root, "core/common/build.gradle.kts", "plugins {\n    id(ThunderbirdPlugins.Library.kmp)\n}\n")
	m := module.Read(root, ManifestReaders()...)
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
