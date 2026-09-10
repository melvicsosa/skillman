package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/melvicsosa/skillman/internal/app"
)

var (
	syncAll     bool
	syncProject string
	syncDryRun  bool
	syncJSON    bool
)

var syncCmd = &cobra.Command{
	Use:   "sync [<name>...] [--all] [--project <path>] [--dry-run]",
	Short: "Fan vault entries out to every enabled agent",
	Long: `Make sure the given vault entries (or every entry with --all) are installed
in every enabled, writable agent: globally, or in the project given with
--project. Entries already installed as copies are copied, the rest are
symlinked. A skill dir of the same name that is not the vault entry is
reported as a conflict and left alone.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 && !syncAll {
			return fmt.Errorf("pass entry names or --all")
		}
		rt, err := newRuntime(cmd.Context())
		if err != nil {
			return err
		}
		defer rt.Close()
		report, err := rt.svc.Sync(cmd.Context(), app.SyncOptions{Names: args, All: syncAll, Root: syncProject, DryRun: syncDryRun})
		if err != nil {
			return err
		}
		if syncJSON {
			return writeJSON(cmd.OutOrStdout(), report)
		}
		printSyncReport(cmd, report)
		if report.Conflicts > 0 {
			return fmt.Errorf("%d conflicts", report.Conflicts)
		}
		return nil
	},
}

func init() {
	syncCmd.Flags().BoolVar(&syncAll, "all", false, "sync every vault entry")
	syncCmd.Flags().StringVar(&syncProject, "project", "", "sync into this project root instead of globally")
	syncCmd.Flags().BoolVar(&syncDryRun, "dry-run", false, "report what would change without writing")
	syncCmd.Flags().BoolVar(&syncJSON, "json", false, "print the report as JSON")
}

func printSyncReport(cmd *cobra.Command, report app.SyncReport) {
	out := cmd.OutOrStdout()
	prefix := ""
	if report.DryRun {
		prefix = "[dry-run] "
	}
	for _, e := range report.Entries {
		fmt.Fprintf(out, "%s%s\n", prefix, e.Name)
		for _, l := range e.Links {
			line := fmt.Sprintf("  %-12s %-9s %s", l.Agent, l.Status, l.Path)
			if l.Message != "" {
				line += " (" + l.Message + ")"
			}
			fmt.Fprintln(out, line)
		}
	}
	fmt.Fprintf(out, "%s%d linked, %d already installed, %d conflicts\n", prefix, report.Linked, report.Already, report.Conflicts)
}
