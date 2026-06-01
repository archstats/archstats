package common

import (
	"fmt"
	"github.com/archstats/archstats/cmd/config"
	"github.com/archstats/archstats/core/walker"
	"github.com/rs/zerolog/log"
	"github.com/samber/lo"
	"github.com/spf13/cobra"
	"path/filepath"
)

func GetEnabledExtensions(cmd *cobra.Command) ([]*config.CLIConfiguredExtension, error) {
	enabledExtensions := AlwaysEnabled()
	wantToEnableExtensions, err := cmd.Flags().GetStringSlice(FlagExtension)
	if err != nil {
		log.Error().Msg("Error getting extensions")
		return nil, err
	}
	optionalExtensions := Optional()
	optionalMap := lo.SliceToMap(optionalExtensions, func(extension *config.CLIConfiguredExtension) (string, *config.CLIConfiguredExtension) {
		return extension.Name, extension
	})

	if len(wantToEnableExtensions) > 0 {
		// Explicit extensions specified: disable auto-discovery
		for _, extensionName := range wantToEnableExtensions {
			if _, ok := optionalMap[extensionName]; !ok {
				return nil, fmt.Errorf("unknown extension: %s", extensionName)
			}
			enabledExtensions = append(enabledExtensions, optionalMap[extensionName])
		}
		return enabledExtensions, nil
	}

	// No explicit extensions: execute auto-discovery
	rootDir, _ := cmd.Flags().GetString(FlagWorkingDirectory)
	rootDir, _ = filepath.Abs(rootDir)

	// Recursively get all unignored files using the existing walker
	unignoredFiles := walker.GetAllFiles(rootDir)
	var filePaths []string
	for _, f := range unignoredFiles {
		filePaths = append(filePaths, f.Path())
	}

	discoveryCtx := &config.DiscoveryContext{
		RootDir: rootDir,
		Files:   filePaths,
	}

	var discoveredNames []string
	for _, optExt := range optionalExtensions {
		if optExt.DiscoveryTrigger != nil && optExt.DiscoveryTrigger(discoveryCtx) {
			enabledExtensions = append(enabledExtensions, optExt)
			discoveredNames = append(discoveredNames, optExt.Name)
		}
	}

	if len(discoveredNames) > 0 {
		log.Debug().Msgf("Auto-discovered extensions: %v", discoveredNames)
	}

	return enabledExtensions, nil
}
