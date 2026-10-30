package dart

import (
	"path"
	"strings"

	"github.com/archstats/archstats/core/file"
)

// Conventions are Dart's: package:test's *_test.dart.
func Conventions() file.Conventions {
	return file.Conventions{Name: "dart", IsTest: func(p string) bool { return strings.HasSuffix(path.Base(p), "_test.dart") }}
}
