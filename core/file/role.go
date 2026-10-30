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
// third_party > generated > test > non_code > production. A Classifier
// decides it.
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

// Directories that hold tests in any language: tests/, spec/, e2e/. Each
// ecosystem's own conventions -- src/test, __tests__, testdata -- are its
// pack's.
var genericTestDir = regexp.MustCompile(`(^|/)(tests?|specs?|e2e)(/|$)`)

// Gherkin features are tests whatever language runs them.
var genericTestName = regexp.MustCompile(`\.feature$`)

// nonCodeExts is text that is not code: data, configuration, documents,
// translations, stylesheets and images. It gets no health reading, since a
// health reading is about how code is shaped. These are formats, not
// languages; a lockfile is non-code by its system kind instead.
var nonCodeExts = map[string]bool{
	".json": true, ".yaml": true, ".yml": true, ".md": true, ".txt": true,
	".lock": true, ".xml": true, ".toml": true, ".ini": true, ".conf": true,
	".csv": true, ".po": true, ".pot": true, ".mo": true, ".properties": true,
	".css": true, ".scss": true, ".sass": true, ".less": true, ".styl": true,
	".svg": true, ".map": true, ".snap": true, ".rst": true, ".adoc": true,
}

// IsNonCode reports whether a file is, by its format, text that is not code.
func IsNonCode(filePath string) bool {
	return nonCodeExts[path.Ext(strings.ToLower(path.Base(filePath)))]
}
