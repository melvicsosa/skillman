package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/melvicsosa/skillman/internal/app"
)

var (
	scanProject string
	scanJSON    bool
)

var scanCmd = &cobra.Command{
	Use:   "scan",
	Short: "Discover skills on disk and refresh the cache",
	RunE: func(cmd *cobra.Command, _ []string) error {
		rt, err := newRuntime(cmd.Context())
		if err != nil {
			return err
		}
		defer rt.Close()
		opts := app.ScanOptions{}
		if scanProject != "" {
			opts.ProjectRoot = &scanProject
		}
		sum, err := rt.svc.Scan(cmd.Context(), opts)
		if err != nil {
			return err
		}
		if scanJSON {
			return writeJSON(cmd.OutOrStdout(), sum)
		}
		printScanSummary(cmd, sum)
		return nil
	},
}

func init() {
	scanCmd.Flags().StringVar(&scanProject, "project", "", "scan only this project root (must be registered)")
	scanCmd.Flags().BoolVar(&scanJSON, "json", false, "print the summary as JSON")
}

func printScanSummary(cmd *cobra.Command, sum app.ScanSummary) {
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "Scanned: %d skills found, %d added, %d removed, %d disabled\n", sum.Found, sum.Added, sum.Removed, sum.Disabled)
	for _, e := range sum.Errors {
		fmt.Fprintf(cmd.ErrOrStderr(), "warning: %s\n", e)
	}
}
