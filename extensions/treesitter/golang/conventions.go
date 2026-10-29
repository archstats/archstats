package golang

import (
	"path"
	"regexp"
	"strings"

	"github.com/archstats/archstats/core/file"
)

// Conventions are Go's: *_test.go, and testdata/, which the go tool ignores.
func Conventions() file.Conventions {
	return file.Conventions{Name: "go", IsTest: isTest}
}

var testDir = regexp.MustCompile(`(^|/)testdata(/|$)`)

func isTest(p string) bool {
	return strings.HasSuffix(path.Base(p), "_test.go") || testDir.MatchString(strings.ToLower(p))
}
