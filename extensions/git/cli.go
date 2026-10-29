package git

import (
	"fmt"
	"github.com/archstats/archstats/core"
	"github.com/archstats/archstats/core/plugin"
)

const (
	GitSince               = "git-since"
	GitAfter               = "git-after"
	GitMaxChangesPerCommit = "git-max-changes-per-commit"
	GitBasedOn             = "git-based-on"
)

func CLIExtension() *plugin.Descriptor {
	return &plugin.Descriptor{
		Name:        "git",
		Description: "Git extension",
		Arguments: plugin.Arguments{
			GitAfter: {
				Default:     "",
				Description: "Passed to git log --after",
				Required:    false,
				Type:        plugin.String,
			},
			GitSince: {
				Default:     "",
				Description: "Passed to git log --since",
				Required:    false,
				Type:        plugin.String,
			},
			GitBasedOn: {
				Default:     "head",
				Description: "What the last-N-days windows count back from: \"head\", the newest commit scanned, so the same commit reads the same on any day; or \"now\", the moment of the scan",
				Required:    false,
				Type:        plugin.String,
			},
			GitMaxChangesPerCommit: {
				Default:     100,
				Description: "Filter out commits that modify more than this number of files (0 to disable)",
				Required:    false,
				Type:        plugin.Int,
			},
		},
		Initializer: Init,
	}
}

func Init(settings plugin.Settings) (core.Extension, error) {
	gitSince, err := settings.GetString(GitSince)
	if err != nil {
		return nil, err
	}
	gitAfter, err := settings.GetString(GitAfter)
	if err != nil {
		return nil, err
	}
	gitMaxChanges, err := settings.GetInt(GitMaxChangesPerCommit)
	if err != nil {
		return nil, err
	}

	basedOn, err := settings.GetString(GitBasedOn)
	if err != nil {
		return nil, err
	}
	if basedOn != "head" && basedOn != "now" {
		return nil, fmt.Errorf("--%s must be head or now, not %q", GitBasedOn, basedOn)
	}

	ext := Extension().(*extension)
	ext.BasedOnMode = basedOn
	ext.GitSince = gitSince
	ext.GitAfter = gitAfter
	ext.MaxChangesPerCommit = gitMaxChanges
	return ext, nil
}
