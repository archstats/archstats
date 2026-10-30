package jvm

import (
	"path"
	"regexp"

	"github.com/archstats/archstats/core/file"
)

// Conventions are the JVM's: Gradle's test source sets, JUnit's file names,
// and directories that are packages. Maven's src/test is a test/ folder, which
// core knows in every language.
func Conventions() file.Conventions {
	return file.Conventions{
		Name:           "jvm",
		IsTest:         isTest,
		PackageDirExts: []string{".java", ".kt", ".kts", ".scala", ".groovy"},
	}
}

var (
	// JUnit's *Test(s).java and failsafe's *IT.java, in any JVM language.
	testName = regexp.MustCompile(`(Tests?|IT)\.(java|kt|scala|groovy)$`)
	// A Gradle test source set: src/androidTest, src/commonTest,
	// src/androidUnitTest. Read in the path's own case: src/contest is code.
	sourceSetTest = regexp.MustCompile(`(^|/)src/[a-z]+([A-Z][a-z]*)*Test(/|$)`)
)

func isTest(p string) bool {
	return testName.MatchString(path.Base(p)) || sourceSetTest.MatchString(p)
}
