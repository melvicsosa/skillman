package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	importLockPath string
	importLockJSON bool
)

var importLockCmd = &cobra.Command{
	Use:   "import-lock [--file <path>]",
	Short: "Register skills from ~/.agents/.skill-lock.json into the vault",
	RunE: func(cmd *cobra.Command, _ []string) error {
		rt, err := newRuntime(cmd.Context())
		if err != nil {
			return err
		}
		defer rt.Close()
		res, err := rt.svc.ImportLock(cmd.Context(), importLockPath)
		if err != nil {
			return err
		}
		if importLockJSON {
			return writeJSON(cmd.OutOrStdout(), res)
		}
		out := cmd.OutOrStdout()
		fmt.Fprintf(out, "Lockfile: %s\n", res.LockFile)
		for _, n := range res.Imported {
			fmt.Fprintf(out, "  imported %s\n", n)
		}
		for _, s := range res.Skipped {
			fmt.Fprintf(out, "  skipped %s: %s\n", s.Name, s.Reason)
		}
		fmt.Fprintf(out, "%d imported, %d skipped\n", len(res.Imported), len(res.Skipped))
		return nil
	},
}

func init() {
	importLockCmd.Flags().StringVar(&importLockPath, "file", "", "lockfile path (default ~/.agents/.skill-lock.json)")
	importLockCmd.Flags().BoolVar(&importLockJSON, "json", false, "print as JSON")
}
