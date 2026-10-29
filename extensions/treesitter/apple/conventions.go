package apple

import (
	"path"
	"regexp"

	"github.com/archstats/archstats/core/file"
)

// Conventions are Xcode's and SwiftPM's: XCTest file names and test targets.
func Conventions() file.Conventions {
	return file.Conventions{Name: "apple", IsTest: isTest}
}

var (
	testName = regexp.MustCompile(`(Tests?|Spec)\.(swift|m|mm)$`)
	// An Xcode or SwiftPM test target: AppTests, WondrousUITests. Read in the
	// path's own case, so `contests/` is not one.
	testTarget = regexp.MustCompile(`(^|/)[A-Za-z0-9_]*[a-z0-9](UI)?Tests(/|$)`)
)

func isTest(p string) bool {
	return testName.MatchString(path.Base(p)) || testTarget.MatchString(p)
}
