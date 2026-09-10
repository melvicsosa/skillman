package cli

import (
	"fmt"
	"sort"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/melvicsosa/skillman/internal/app"
)

var doctorJSON bool

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Report data dir, database state, detected agents and skill issues",
	RunE:  runDoctor,
}

func init() {
	doctorCmd.Flags().BoolVar(&doctorJSON, "json", false, "print the issue report as JSON")
}

// runDoctor is read-only except for opening the database, which creates the
// data dir and applies migrations if they are missing.
func runDoctor(cmd *cobra.Command, _ []string) error {
	out := cmd.OutOrStdout()
	rt, err := newRuntime(cmd.Context())
	if err != nil {
		return err
	}
	defer rt.Close()

	report, err := rt.svc.Doctor(cmd.Context())
	if err != nil {
		return err
	}
	if doctorJSON {
		return writeJSON(out, report)
	}

	fmt.Fprintf(out, "Data dir:  %s\n", rt.dataDir)
	fmt.Fprintf(out, "Database:  %s\n", dbPath(rt.dataDir))
	v, err := rt.db.MigrationVersion(cmd.Context())
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "Migration: %d\n\n", v)

	agents, err := rt.svc.ListAgents(cmd.Context())
	if err != nil {
		return err
	}
	fmt.Fprintln(out, "Agents:")
	tw := tabwriter.NewWriter(out, 2, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "  ID\tNAME\tENABLED\tGLOBAL DIR\tSTATUS")
	for _, a := range agents {
		for i, d := range a.GlobalDirs {
			status := "missing"
			if dirExists(d) {
				status = "found"
			}
			id, name, enabled := string(a.ID), a.Name, fmt.Sprint(a.Enabled)
			if i > 0 {
				id, name, enabled = "", "", ""
			}
			fmt.Fprintf(tw, "  %s\t%s\t%s\t%s\t%s\n", id, name, enabled, d, status)
		}
	}
	if err := tw.Flush(); err != nil {
		return err
	}

	fmt.Fprintf(out, "\nIssues: %d\n", len(report.Issues))
	printIssues(cmd, report)
	return nil
}

func printIssues(cmd *cobra.Command, report app.DoctorReport) {
	out := cmd.OutOrStdout()
	kinds := make([]string, 0, len(report.Counts))
	for k := range report.Counts {
		kinds = append(kinds, k)
	}
	sort.Strings(kinds)
	for _, kind := range kinds {
		fmt.Fprintf(out, "\n%s (%d)\n", kind, report.Counts[kind])
		for _, i := range report.Issues {
			if i.Kind != kind {
				continue
			}
			where := i.Path
			if where == "" {
				where = i.Name + " [" + i.Scope + "]"
			}
			if i.Agent != "" && i.Kind != app.IssueDrift {
				where = i.Agent + " " + where
			}
			fmt.Fprintf(out, "  - %s\n      %s\n", where, i.Message)
			if i.Kind == app.IssueDrift {
				for _, d := range i.Details {
					fmt.Fprintf(out, "      %s\n", d)
				}
			}
		}
	}
}
