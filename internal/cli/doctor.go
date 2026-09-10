package cli

import (
	"fmt"
	"sort"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/melvicsosa/skillman/internal/app"
	"github.com/melvicsosa/skillman/internal/domain"
)

func shortHash(h string) string {
	if len(h) > 12 {
		return h[:12]
	}
	return h
}

var (
	doctorJSON     bool
	doctorFixDrift string
	doctorFrom     string
	doctorTo       []string
	doctorProject  string
)

var doctorCmd = &cobra.Command{
	Use:   "doctor [--fix-drift <name> --from <agent|vault> [--to <agent>...] [--project <path>]]",
	Short: "Report data dir, database state, detected agents and skill issues",
	RunE:  runDoctor,
}

func init() {
	doctorCmd.Flags().BoolVar(&doctorJSON, "json", false, "print the issue report as JSON")
	doctorCmd.Flags().StringVar(&doctorFixDrift, "fix-drift", "", "repair the drifting skill with this name")
	doctorCmd.Flags().StringVar(&doctorFrom, "from", "", "repair source: an agent id or \"vault\"")
	doctorCmd.Flags().StringArrayVar(&doctorTo, "to", nil, "only repair these agents' copies (repeatable)")
	doctorCmd.Flags().StringVar(&doctorProject, "project", "", "project root of the drifting copies (default global)")
}

// runFixDrift handles --fix-drift.
func runFixDrift(cmd *cobra.Command, rt *env) error {
	if doctorFrom == "" {
		return fmt.Errorf("--from <agent|vault> is required with --fix-drift")
	}
	opts := app.RepairOptions{Name: doctorFixDrift, From: doctorFrom, Root: doctorProject}
	for _, a := range doctorTo {
		opts.To = append(opts.To, domain.AgentID(a))
	}
	res, err := rt.svc.RepairDrift(cmd.Context(), opts)
	if err != nil {
		return err
	}
	if doctorJSON {
		return writeJSON(cmd.OutOrStdout(), res)
	}
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "Repaired %s from %s (%s)\n", res.Name, res.From, res.SourcePath)
	for _, p := range res.Replaced {
		fmt.Fprintf(out, "  replaced %s\n", p)
	}
	for _, sk := range res.Skipped {
		fmt.Fprintf(out, "  skipped  %s (%s: %s)\n", sk.Path, sk.Agent, sk.Reason)
	}
	if len(res.Replaced) == 0 {
		fmt.Fprintln(out, "Nothing to replace.")
	}
	return nil
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
	if doctorFixDrift != "" {
		return runFixDrift(cmd, rt)
	}

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
				for _, c := range i.Copies {
					state := "repairable"
					if !c.Repairable {
						state = c.Reason
					}
					fmt.Fprintf(out, "      %-12s %s %s (%s)\n", c.Agent, shortHash(c.Hash), c.Path, state)
				}
				sources := "an agent"
				if i.VaultRef != "" {
					sources = "vault or an agent"
				}
				if i.Repairable {
					fmt.Fprintf(out, "      fix: skillman doctor --fix-drift %s --from <%s>\n", i.Name, sources)
				}
				if i.VaultRef == "" {
					fmt.Fprintf(out, "      adopt: skillman vault adopt %s --from <agent>\n", i.Name)
				}
			}
		}
	}
}
