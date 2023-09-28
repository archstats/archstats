package cmd

import (
	"context"
	"github.com/RyanSusana/archstats/analysis"
	"github.com/RyanSusana/archstats/cmd/common"
	"github.com/RyanSusana/archstats/cmd/export"
	"github.com/RyanSusana/archstats/cmd/view"
	"github.com/spf13/cobra"
	"io"
)

func Cmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "archstats",
		Short: "archstats is a command line tool for generating software architectural insights",

		PreRun: func(cmd *cobra.Command, args []string) {
		},
	}
	cmd.PersistentFlags().StringSliceP(common.FlagExtension, "e", nil, "Archstat extension(s) to use")
	cmd.PersistentFlags().StringSliceP(common.FlagSnippet, "s", nil, "Regular Expression to match snippet types. FlagSnippet types are named by using regex named groups(?P<typeName>). For example, if you want to match a JavaScript function, you can use the regex 'function (?P<function>.*)'")
	cmd.PersistentFlags().StringP(common.FlagWorkingDirectory, "f", "", "Input directory")

	cmd.AddCommand(view.Cmd())
	cmd.AddCommand(export.Cmd())
	return cmd
}

func Execute(outStream, errorStream io.Writer, extensions []analysis.Extension, args []string) error {
	rootCmd := Cmd()
	rootCmd.SetArgs(args)
	rootCmd.SetOut(outStream)
	rootCmd.SetErr(errorStream)

	return rootCmd.ExecuteContext(context.WithValue(context.Background(), "extraExtensions", extensions))
}
