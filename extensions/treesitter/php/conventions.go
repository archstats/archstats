package php

import (
	"path"
	"regexp"
	"strings"

	"github.com/archstats/archstats/core/file"
)

// Conventions are PHP's: PHPUnit's and phpspec's file names, and Behat.
func Conventions() file.Conventions {
	return file.Conventions{Name: "php", IsTest: isTest}
}

var (
	testName = regexp.MustCompile(`(Test|Spec)\.php$`)
	// Sylius keeps its Behat contexts under src/Sylius/Behat.
	testDir = regexp.MustCompile(`(^|/)behat(/|$)`)
)

func isTest(p string) bool {
	return testName.MatchString(path.Base(p)) || testDir.MatchString(strings.ToLower(p))
}
