package file

import (
	"path"
	"regexp"
	"strings"
)

// A file's role says whose code it is and what it is for, so a reader can
// tell "the product is hard to change" from "the tests are": production code,
// tests, files a tool wrote, someone else's code carried in the repository,
// or text that is not code at all. One role per file, by precedence:
// third_party > generated > test > non_code > production.
const (
	RoleProduction = "production"
	RoleTest       = "test"
	RoleGenerated  = "generated"
	RoleThirdParty = "third_party"
	RoleNonCode    = "non_code"
)

// Stats counting test code, summed over any group of files.
const (
	TestFileCount = "complexity__files__test"
	TestLineCount = "complexity__lines__test"
)

// Role classifies a file from its path and what the content checks found.
func Role(filePath string, thirdParty, generated bool) string {
	switch {
	case thirdParty:
		return RoleThirdParty
	case generated:
		return RoleGenerated
	case IsTestPath(filePath):
		return RoleTest
	case IsNonCode(filePath):
		return RoleNonCode
	default:
		return RoleProduction
	}
}

// Directories that hold tests, by the conventions of each ecosystem: JVM
// src/test, PHP and Python tests/, spec/, Sylius's Behat/, Go testdata/,
// JavaScript __tests__/, e2e/ and cypress/, and the Gradle source sets
// Android and Kotlin Multiplatform test in (sourceSetTest).
var testDir = regexp.MustCompile(`(^|/)(src/test|tests?|spec|specs|__tests__|e2e|cypress|testdata|behat)(/|$)`)

// File names that are tests: Java/Kotlin *Test(s).java and *IT.java,
// PHP *Test.php / *Spec.php, Python test_*.py / *_test.py / conftest.py,
// Go *_test.go, JavaScript *.test.* / *.spec.*, Gherkin *.feature, Swift and
// Objective-C *Tests.swift / *Tests.m, Dart *_test.dart.
var testName = regexp.MustCompile(`(Tests?|IT)\.(java|kt|scala|groovy)$|(Test|Spec)\.php$|^test_.*\.py$|_test\.(py|go|dart)$|^conftest\.py$|\.(test|spec)\.[cm]?[jt]sx?$|\.feature$|(Tests?|Spec)\.(swift|m|mm)$`)

// A .NET test project: Nop.Tests, Foo.UnitTests, Bar.IntegrationTests.
var dotnetTestProject = regexp.MustCompile(`(^|/)[^/]*\.(Unit|Integration|Functional|Acceptance)?Tests?(/|$)`)

// An Xcode or SwiftPM test target: AppTests, WondrousUITests. Read in the
// path's own case, so `contests/` is not one.
var appleTestTarget = regexp.MustCompile(`(^|/)[A-Za-z0-9_]*[a-z0-9](UI)?Tests(/|$)`)

// A Gradle test source set: src/androidTest, src/commonTest,
// src/androidUnitTest. Also in the path's own case: src/contest is code.
var sourceSetTest = regexp.MustCompile(`(^|/)src/[a-z]+([A-Z][a-z]*)*Test(/|$)`)

// IsTestPath reports whether a path is test code by its ecosystem's
// convention. Paths are as the walker names them ("./x" at the root).
func IsTestPath(filePath string) bool {
	p := strings.TrimPrefix(filePath, "./")
	lower := strings.ToLower(p)
	if testDir.MatchString(lower) {
		return true
	}
	if testName.MatchString(path.Base(p)) {
		return true
	}
	return dotnetTestProject.MatchString(p) || appleTestTarget.MatchString(p) || sourceSetTest.MatchString(p)
}

// nonCodeExts is text that is not code: data, configuration, documents,
// translations, stylesheets and images. It gets no health reading, since a
// health reading is about how code is shaped.
var nonCodeExts = map[string]bool{
	".json": true, ".yaml": true, ".yml": true, ".md": true, ".txt": true,
	".lock": true, ".xml": true, ".toml": true, ".ini": true, ".conf": true,
	".csv": true, ".po": true, ".pot": true, ".mo": true, ".properties": true,
	".css": true, ".scss": true, ".sass": true, ".less": true, ".styl": true,
	".svg": true, ".map": true, ".snap": true, ".rst": true, ".adoc": true,
}

var nonCodeNames = map[string]bool{
	"package-lock.json": true, "yarn.lock": true, "pnpm-lock.yaml": true,
	"go.sum": true, "cargo.lock": true, "composer.lock": true,
}

// IsNonCode reports whether a file is text that is not code.
func IsNonCode(filePath string) bool {
	base := strings.ToLower(path.Base(filePath))
	return nonCodeNames[base] || nonCodeExts[path.Ext(base)]
}
