package java

import (
	"fmt"
	"github.com/archstats/archstats/cmd/config"
	"github.com/archstats/archstats/core"
	"github.com/spf13/cobra"
	"os"
)

const (
	IgnoreCommonImports   = "java-ignore-common-imports"
	TreesitterQueriesFile = "java-treesitter-queries"
)

func Ext() *config.CLIConfiguredExtension {
	return &config.CLIConfiguredExtension{
		Name:        "java",
		Description: "Java language extension",
		Arguments: config.Arguments{
			IgnoreCommonImports: {
				Default:     true,
				Description: "Ignore common Java imports",
				Required:    false,
				Type:        config.Bool,
			},
			TreesitterQueriesFile: {
				Default:     []string{},
				Description: "Path to the treesitter queries file",
				Required:    false,
				Type:        config.StringSlice,
			},
		},
		Initializer: Init,
	}
}

func Init(command *cobra.Command) (core.Extension, error) {
	ignoreCommonImports, err := command.Flags().GetBool(IgnoreCommonImports)
	if err != nil {
		return nil, err
	}
	treesitterQueriesFiles, err := command.Flags().GetStringSlice(TreesitterQueriesFile)
	if err != nil {
		return nil, err
	}

	var extraQueries []string
	for _, queriesFile := range treesitterQueriesFiles {
		file, err := os.ReadFile(queriesFile)
		if err != nil {
			if os.IsNotExist(err) {
				return nil, fmt.Errorf("file '%s' does not exist, can't use it to parse treesitter queries", queriesFile)

			}
			return nil, err
		}

		content := string(file)
		extraQueries = append(extraQueries, content)
	}

	return &Extension{
		IgnoreCommonJavaImports: ignoreCommonImports,
		ExtraQueries:            extraQueries,
	}, nil
}
