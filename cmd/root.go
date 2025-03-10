package cmd

import (
	"context"
	"github.com/archstats/archstats/cmd/common"
	"github.com/archstats/archstats/cmd/export"
	"github.com/archstats/archstats/cmd/view"
	"github.com/archstats/archstats/core"
	"github.com/archstats/archstats/version"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"io"
)

func Cmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "archstats",
		Short:   "archstats is a command line tool for generating software architectural insights",
		Version: version.Version(),

		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			verbose, err := cmd.Flags().GetBool(common.FlagVerbose)
			zerolog.SetGlobalLevel(zerolog.InfoLevel)

			log.Logger = log.Output(zerolog.ConsoleWriter{Out: cmd.OutOrStderr(), NoColor: true, TimeFormat: "2006-01-02 15:04:05.000"})
			if err != nil {
				log.Err(err).Msg("Error getting verbose flag")
			}
			if verbose {
				zerolog.SetGlobalLevel(zerolog.DebugLevel)
				log.Debug().Msg("Verbose output enabled")
			}
		},
	}
	cmd.PersistentFlags().StringSliceP(common.FlagExtension, "e", nil, "Archstat extension(s) to use")
	cmd.PersistentFlags().StringSlice(common.FlagSnippet, nil, "Regular Expression to match snippet types. FlagSnippet types are named by using regex named groups(?P<typeName>). For example, if you want to match a JavaScript function, you can use the regex 'function (?P<function>.*)'")
	cmd.PersistentFlags().StringP(common.FlagWorkingDirectory, "f", "", "Input directory")

	cmd.PersistentFlags().StringToStringP(common.FlagSet, "s", nil, "Configuration for extensions")
	cmd.PersistentFlags().BoolP(common.FlagVerbose, "v", false, "Verbose output")

	cmd.AddCommand(view.Cmd())
	cmd.AddCommand(export.Cmd())
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	return cmd
}

func Execute(outStream, errorStream io.Writer, extensions []core.Extension, args []string) error {
	rootCmd := Cmd()
	rootCmd.SetArgs(args)
	rootCmd.SetOut(outStream)
	rootCmd.SetErr(errorStream)

	return rootCmd.ExecuteContext(context.WithValue(context.Background(), "extraExtensions", extensions))
}
