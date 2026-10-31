package version

import (
	"runtime/debug"
	"testing"
)

func TestFromBuildInfo(t *testing.T) {
	cases := []struct {
		name string
		info debug.BuildInfo
		want string
	}{
		{"go install of a tag", debug.BuildInfo{Main: debug.Module{Path: module, Version: "v0.11.0"}}, "v0.11.0"},
		{"built from a working tree", debug.BuildInfo{Main: debug.Module{Path: module, Version: "(devel)"}}, "dev"},
		{"imported at a tag", debug.BuildInfo{
			Main: debug.Module{Path: "github.com/archstats/archstats-ui", Version: "(devel)"},
			Deps: []*debug.Module{{Path: "github.com/other/lib", Version: "v1.2.3"}, {Path: module, Version: "v0.11.0"}},
		}, "v0.11.0"},
		{"imported at a commit", debug.BuildInfo{
			Main: debug.Module{Path: "github.com/archstats/archstats-ui"},
			Deps: []*debug.Module{{Path: module, Version: "v0.11.1-0.20261030101740-f818da9d0b07"}},
		}, "v0.11.1-0.20261030101740-f818da9d0b07"},
		{"imported through a local replace", debug.BuildInfo{
			Main: debug.Module{Path: "github.com/archstats/archstats-ui"},
			Deps: []*debug.Module{{Path: module, Version: "v0.11.0", Replace: &debug.Module{Path: "../archstats"}}},
		}, "dev"},
		{"not linked at all", debug.BuildInfo{Main: debug.Module{Path: "example.com/x"}}, "dev"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := fromBuildInfo(&c.info); got != c.want {
				t.Errorf("got %q, want %q", got, c.want)
			}
		})
	}
}
