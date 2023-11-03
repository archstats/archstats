package cmd

import (
	"context"
	"github.com/archstats/archstats/cmd/common"
	"github.com/archstats/archstats/cmd/export"
	"github.com/archstats/archstats/cmd/view"
	"github.com/archstats/archstats/core"
	"github.com/archstats/archstats/version"
	"github.com/spf13/cobra"
	"io"
)

func Cmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "archstats",
		Short:   "archstats is a command line tool for generating software architectural insights",
		Version: version.Version(),

		PreRun: func(cmd *cobra.Command, args []string) {
		},
	}
	cmd.PersistentFlags().StringSliceP(common.FlagExtension, "e", nil, "Archstat extension(s) to use")
	cmd.PersistentFlags().StringSlice(common.FlagSnippet, nil, "Regular Expression to match snippet types. FlagSnippet types are named by using regex named groups(?P<typeName>). For example, if you want to match a JavaScript function, you can use the regex 'function (?P<function>.*)'")
	cmd.PersistentFlags().StringP(common.FlagWorkingDirectory, "f", "", "Input directory")

	cmd.PersistentFlags().StringToStringP(common.FlagSet, "s", nil, "Configuration for extensions")

	cmd.AddCommand(view.Cmd())
	cmd.AddCommand(export.Cmd())
	return cmd
}

func Execute(outStream, errorStream io.Writer, extensions []core.Extension, args []string) error {
	rootCmd := Cmd()
	rootCmd.SetArgs(args)
	rootCmd.SetOut(outStream)
	rootCmd.SetErr(errorStream)

	return rootCmd.ExecuteContext(context.WithValue(context.Background(), "extraExtensions", extensions))
}
