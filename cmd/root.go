package cmd

import (
	"fmt"
	"github.com/RyanSusana/archstats/cmd/common"
	"github.com/spf13/cobra"
	"io"
)

var rootCmd = &cobra.Command{
	Use:   "archstats",
	Short: "archstats is a command line tool for generating software architectural insights",
	PreRun: func(cmd *cobra.Command, args []string) {
		fmt.Println("OK BRO!")
	},
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		//TODO

		//// Enable cpu profiling if requested.
		//if generalOptions.Profile.Cpu != "" {
		//	f, err := os.CreateViewFunc(generalOptions.Profile.Cpu)
		//	if err != nil {
		//		return "", err
		//	}
		//	defer f.Close() // TODO handle error
		//	if err := pprof.StartCPUProfile(f); err != nil {
		//		return "", err
		//	}
		//	defer pprof.StopCPUProfile()
		//}
		//
		//output, err := runArchStats(generalOptions)
		//
		//// Enable memory profiling if requested.
		//if generalOptions.Profile.Mem != "" {
		//	f, err := os.CreateViewFunc(generalOptions.Profile.Mem)
		//	if err != nil {
		//		return "", err
		//	}
		//	defer f.Close() // TODO handle error
		//	runtime.GC()
		//	if err := pprof.WriteHeapProfile(f); err != nil {
		//		return "", err
		//	}
		//}
		return nil
	},
}

func Execute(outStream, errorStream io.Writer, args []string) error {
	rootCmd.SetArgs(args)
	rootCmd.SetOut(outStream)
	rootCmd.SetErr(errorStream)
	return rootCmd.Execute()
}

func init() {
	rootCmd.PersistentFlags().StringSliceP(common.FlagExtension, "e", nil, "Archstat extension(s) to use")
	rootCmd.PersistentFlags().StringSliceP(common.FlagSnippet, "s", nil, "Regular Expression to match snippet types. FlagSnippet types are named by using regex named groups(?P<typeName>). For example, if you want to match a JavaScript function, you can use the regex 'function (?P<function>.*)'")
	rootCmd.PersistentFlags().StringP(common.FlagWorkingDirectory, "f", "", "Input directory")

	rootCmd.AddCommand(viewCmd)
	rootCmd.AddCommand(exportCmd)
}
