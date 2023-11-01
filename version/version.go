package version

import _ "embed"

//go:embed VERSION
var version string

func Version() string {
	return version
}
