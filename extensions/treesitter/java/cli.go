package java

import (
	"fmt"
	"github.com/archstats/archstats/core"
	"github.com/archstats/archstats/core/plugin"
	"os"
)

const (
	IgnoreCommonImports   = "java-ignore-common-imports"
	TreesitterQueriesFile = "java-treesitter-queries"
	ClassIndirect         = "java-class-indirect"
)

func CLIExtension() *plugin.Descriptor {
	return &plugin.Descriptor{
		Name:        "java",
		Description: "Java language extension",
		Arguments: plugin.Arguments{
			IgnoreCommonImports: {
				Default:     true,
				Description: "Ignore common Java imports",
				Required:    false,
				Type:        plugin.Bool,
			},
			ClassIndirect: {
				Default:     false,
				Description: "Also write java_class_connections_indirect, every class reachable from every class (large)",
				Required:    false,
				Type:        plugin.Bool,
			},
			TreesitterQueriesFile: {
				Default:     []string{},
				Description: "Path to the treesitter queries file",
				Required:    false,
				Type:        plugin.StringSlice,
			},
		},
		Initializer: Init,
	}
}

func Init(settings plugin.Settings) (core.Extension, error) {
	ignoreCommonImports, err := settings.GetBool(IgnoreCommonImports)
	if err != nil {
		return nil, err
	}
	classIndirect, err := settings.GetBool(ClassIndirect)
	if err != nil {
		return nil, err
	}
	treesitterQueriesFiles, err := settings.GetStringSlice(TreesitterQueriesFile)
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
		ClassIndirect:           classIndirect,
	}, nil
}
