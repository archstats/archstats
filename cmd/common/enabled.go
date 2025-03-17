package common

import (
	"github.com/archstats/archstats/cmd/config"
	"github.com/rs/zerolog/log"
	"github.com/samber/lo"
	"github.com/spf13/cobra"
)

func GetEnabledExtensions(cmd *cobra.Command) ([]*config.CLIConfiguredExtension, error) {
	enabledExtensions := AlwaysEnabled()
	wantToEnableExtensions, err := cmd.Flags().GetStringSlice(FlagExtension)
	if err != nil {
		log.Error().Msg("Error getting extensions")
		return nil, err
	}
	optionalExtensions := lo.SliceToMap(Optional(), func(extension *config.CLIConfiguredExtension) (string, *config.CLIConfiguredExtension) {
		return extension.Name, extension
	})
	for _, extension := range wantToEnableExtensions {
		if _, ok := optionalExtensions[extension]; !ok {
			log.Error().Msgf("Unknown extension %s", extension)
			return nil, err
		}
		enabledExtensions = append(enabledExtensions, optionalExtensions[extension])
	}
	return enabledExtensions, nil
}
