package cmd

import (
	"github.com/RyanSusana/archstats/cmd/export/sqlite"
	"github.com/spf13/cobra"
)

var exportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export data to a destination",
	Long:  `Export data to a destination`,
}

func init() {
	exportCmd.AddCommand(sqlite.Command)
}
