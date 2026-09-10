package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/melvicsosa/skillman/internal/app/version"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version, commit and build date",
	Run: func(cmd *cobra.Command, _ []string) {
		fmt.Fprintf(cmd.OutOrStdout(), "skillman %s (commit %s, built %s)\n",
			version.Version, version.Commit, version.Date)
	},
}
