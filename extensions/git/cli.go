package git

import (
	"github.com/archstats/archstats/cmd/config"
	"github.com/archstats/archstats/core"
	"github.com/spf13/cobra"
)

const (
	GitSince = "git-since"
	GitAfter = "git-after"
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

	ext := Extension().(*extension)
	ext.GitSince = gitSince
	ext.GitAfter = gitAfter
	return ext, nil
}
