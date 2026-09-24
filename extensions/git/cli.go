package git

import (
	"fmt"
	"github.com/archstats/archstats/cmd/config"
	"github.com/archstats/archstats/core"
	"github.com/spf13/cobra"
)

const (
	GitSince               = "git-since"
	GitAfter               = "git-after"
	GitMaxChangesPerCommit = "git-max-changes-per-commit"
	GitBasedOn             = "git-based-on"
)

func CLIExtension() *config.CLIConfiguredExtension {
	return &config.CLIConfiguredExtension{
		Name:        "git",
		Description: "Git extension",
		Arguments: config.Arguments{
			GitAfter: {
				Default:     "",
				Description: "Passed to git log --after",
				Required:    false,
				Type:        config.String,
			},
			GitSince: {
				Default:     "",
				Description: "Passed to git log --since",
				Required:    false,
				Type:        config.String,
			},
			GitBasedOn: {
				Default:     "head",
				Description: "What the last-N-days windows count back from: \"head\", the newest commit scanned, so the same commit reads the same on any day; or \"now\", the moment of the scan",
				Required:    false,
				Type:        config.String,
			},
			GitMaxChangesPerCommit: {
				Default:     100,
				Description: "Filter out commits that modify more than this number of files (0 to disable)",
				Required:    false,
				Type:        config.Int,
			},
		},
		Initializer: Init,
	}
}

func Init(command *cobra.Command) (core.Extension, error) {
	gitSince, err := command.Flags().GetString(GitSince)
	if err != nil {
		return nil, err
	}
	gitAfter, err := command.Flags().GetString(GitAfter)
	if err != nil {
		return nil, err
	}
	gitMaxChanges, err := command.Flags().GetInt(GitMaxChangesPerCommit)
	if err != nil {
		return nil, err
	}

	basedOn, err := command.Flags().GetString(GitBasedOn)
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
