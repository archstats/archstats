package export

import (
	"github.com/archstats/archstats/cmd/export/sqlite"
	"github.com/spf13/cobra"
)

func Cmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export data to a destination",
		Long:  `Export data to a destination`,
	}

	cmd.AddCommand(sqlite.Cmd())
	return cmd
}
