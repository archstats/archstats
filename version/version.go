package version

import (
	_ "embed"
	"runtime/debug"
	"strings"
)

// module is this module's path, as importers see it in their build info.
const module = "github.com/archstats/archstats"

// VERSION holds the tag the release workflow stamps in. It is left empty in
// the repository: a committed value goes stale, and every binary that embeds
// the engine without the workflow (go install, the desktop app) reported it.
//
//go:embed VERSION
var stamped string

// Version names the engine release that is running: the stamped tag when the
// release workflow built it, otherwise the module version Go recorded at build
// time, which is the tag (or pseudo-version) for go install and for programs
// that import the engine. A build from a working tree reports "dev".
func Version() string {
	if v := strings.TrimSpace(stamped); v != "" {
		return v
	}
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "dev"
	}
	return fromBuildInfo(info)
}

func fromBuildInfo(info *debug.BuildInfo) string {
	if info.Main.Path == module {
		return known(info.Main.Version)
	}
	for _, dep := range info.Deps {
		if dep.Path != module {
			continue
		}
		if dep.Replace != nil {
			return known(dep.Replace.Version)
		}
		return known(dep.Version)
	}
	return "dev"
}

// known turns Go's placeholder for an unversioned build into "dev".
func known(v string) string {
	if v == "" || v == "(devel)" {
		return "dev"
	}
	return v
}
