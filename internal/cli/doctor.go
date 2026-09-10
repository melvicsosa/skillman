package cli

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/melvicsosa/skillman/internal/adapters/agents"
	"github.com/melvicsosa/skillman/internal/adapters/storage/sqlite"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Report data dir, database state and detected agents",
	RunE:  runDoctor,
}

// runDoctor is read-only except for opening the database, which creates the
// data dir and applies migrations if they are missing.
func runDoctor(cmd *cobra.Command, _ []string) error {
	out := cmd.OutOrStdout()

	dir, err := resolveDataDir()
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "Data dir:  %s\n", dir)
	fmt.Fprintf(out, "Database:  %s\n", dbPath(dir))

	db, err := sqlite.Open(cmd.Context(), dbPath(dir))
	if err != nil {
		return err
	}
	defer db.Close()
	v, err := db.MigrationVersion(cmd.Context())
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "Migration: %d\n\n", v)

	fmt.Fprintln(out, "Agents:")
	tw := tabwriter.NewWriter(out, 2, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "  ID\tNAME\tGLOBAL DIR\tSTATUS")
	for _, s := range agents.Registry {
		for i, d := range s.ResolvedGlobalDirs() {
			status := "missing"
			if info, err := os.Stat(d); err == nil && info.IsDir() {
				status = "found"
			}
			id, name := string(s.ID), s.Name
			if i > 0 {
				id, name = "", ""
			}
			fmt.Fprintf(tw, "  %s\t%s\t%s\t%s\n", id, name, d, status)
		}
	}
	return tw.Flush()
}
