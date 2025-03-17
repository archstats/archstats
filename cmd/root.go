package cmd

import (
	"context"
	"github.com/archstats/archstats/cmd/common"
	"github.com/archstats/archstats/cmd/config"
	"github.com/archstats/archstats/cmd/export"
	"github.com/archstats/archstats/cmd/view"
	"github.com/archstats/archstats/core"
	"github.com/archstats/archstats/version"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"io"
)

func Cmd() (*cobra.Command, error) {
	cmd := &cobra.Command{
		Use:     "archstats",
		Short:   "archstats is a command line tool for generating software architectural insights",
		Version: version.Version(),

		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			log.Logger = log.Output(zerolog.ConsoleWriter{Out: cmd.OutOrStderr(), NoColor: true, TimeFormat: "2006-01-02 15:04:05.000"})
			verbose, err := cmd.Flags().GetBool(common.FlagVerbose)
			zerolog.SetGlobalLevel(zerolog.InfoLevel)

			if err != nil {
				log.Err(err).Msg("Error getting verbose flag")
			}
			if verbose {
				zerolog.SetGlobalLevel(zerolog.DebugLevel)
				log.Debug().Msg("Verbose output enabled")
			}
			return nil
		},
	}

	cmd.PersistentFlags().StringSliceP(common.FlagExtension, "e", nil, "Archstat extension(s) to use")
	cmd.PersistentFlags().StringSlice(common.FlagSnippet, nil, "Regular Expression to match snippet types. FlagSnippet types are named by using regex named groups(?P<typeName>). For example, if you want to match a JavaScript function, you can use the regex 'function (?P<function>.*)'")
	cmd.PersistentFlags().StringP(common.FlagWorkingDirectory, "f", "", "Input directory")

	cmd.PersistentFlags().BoolP(common.FlagVerbose, "v", false, "Verbose output")

	cmd.AddCommand(view.Cmd())
	cmd.AddCommand(export.Cmd())
	zerolog.SetGlobalLevel(zerolog.InfoLevel)

	err := setupAlwaysEnabledExtensions(cmd)
	if err != nil {
		return nil, err
	}
	err = setupOptionalExtensions(cmd)
	if err != nil {
		return nil, err
	}
	return cmd, nil
}

func setupAlwaysEnabledExtensions(cmd *cobra.Command) error {
	alwaysEnabledExtensions := common.AlwaysEnabled()
	for _, extension := range alwaysEnabledExtensions {
		err := setupExtension(cmd, extension, false)
		if err != nil {
			return err
		}
	}
	return nil
}

func setupOptionalExtensions(cmd *cobra.Command) error {
	optionalExtensions := common.Optional()
	for _, extension := range optionalExtensions {
		err := setupExtension(cmd, extension, true)
		if err != nil {
			return err
		}
	}
	return nil
}

func setupExtension(cmd *cobra.Command, cfg *config.CLIConfiguredExtension, enablePostfix bool) error {
	name := cfg.Name

	for param, argumentConfig := range cfg.Arguments {
		paramName := param
		description := argumentConfig.Description
		if enablePostfix {
			description += " (enabled by '" + name + "' extension with --extension/-e flag)"
		}
		switch argumentConfig.Type {
		case config.String:
			cmd.PersistentFlags().StringP(paramName, "", argumentConfig.Default.(string), description)
		case config.Int:
			cmd.PersistentFlags().IntP(paramName, "", argumentConfig.Default.(int), description)
		case config.Bool:
			cmd.PersistentFlags().BoolP(paramName, "", argumentConfig.Default.(bool), description)
		case config.StringSlice:
			cmd.PersistentFlags().StringSliceP(paramName, "", argumentConfig.Default.([]string), description)
		default:
			log.Error().Msgf("Unknown argument type %d", argumentConfig.Type)
		}
		if argumentConfig.Required {
			err := cmd.MarkPersistentFlagRequired(paramName)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func Execute(outStream, errorStream io.Writer, extensions []core.Extension, args []string) error {
	rootCmd, err := Cmd()
	if err != nil {
		return err
	}
	rootCmd.SetArgs(args)
	rootCmd.SetOut(outStream)
	rootCmd.SetErr(errorStream)

	return rootCmd.ExecuteContext(context.WithValue(context.Background(), "extraExtensions", extensions))
}
