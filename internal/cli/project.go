package cli

import (
	"errors"
	"fmt"
	"text/tabwriter"

	"github.com/spf13/cobra"

	httpadapter "github.com/melvicsosa/skillman/internal/adapters/http"
	"github.com/melvicsosa/skillman/internal/app"
	"github.com/melvicsosa/skillman/internal/domain"
)

var (
	projectListJSON       bool
	projectAddCreateSkill bool
)

var projectCmd = &cobra.Command{
	Use:   "project",
	Short: "Register, list or remove project roots whose skills are scanned",
}

var projectAddCmd = &cobra.Command{
	Use:   "add <path>",
	Short: "Register a project root and scan it",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		rt, err := newRuntime(cmd.Context())
		if err != nil {
			return err
		}
		defer rt.Close()
		p, sum, err := rt.svc.RegisterProject(cmd.Context(), args[0], app.RegisterProjectOptions{CreateSkillsDir: projectAddCreateSkill})
		if errors.Is(err, domain.ErrNotAProject) {
			return fmt.Errorf("%w\nre-run with --create-skills-dir to create %s", err, app.SharedProjectDir)
		}
		if err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Registered project %s (%s)\n", p.Name, p.Root)
		printScanSummary(cmd, sum)
		return nil
	},
}

var projectListCmd = &cobra.Command{
	Use:   "list",
	Short: "List registered projects",
	RunE: func(cmd *cobra.Command, _ []string) error {
		rt, err := newRuntime(cmd.Context())
		if err != nil {
			return err
		}
		defer rt.Close()
		list, err := rt.svc.ListProjects(cmd.Context())
		if err != nil {
			return err
		}
		if projectListJSON {
			return writeJSON(cmd.OutOrStdout(), httpadapter.ToProjectViews(list))
		}
		if len(list) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "No projects registered. Use `skillman project add <path>`.")
			return nil
		}
		tw := tabwriter.NewWriter(cmd.OutOrStdout(), 2, 4, 2, ' ', 0)
		fmt.Fprintln(tw, "ID\tNAME\tROOT\tREGISTERED")
		for _, p := range list {
			fmt.Fprintf(tw, "%d\t%s\t%s\t%s\n", p.ID, p.Name, p.Root, p.RegisteredAt.Local().Format("2006-01-02 15:04"))
		}
		return tw.Flush()
	},
}

var projectRemoveCmd = &cobra.Command{
	Use:   "remove <path|id>",
	Short: "Unregister a project (nothing on disk is touched)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		rt, err := newRuntime(cmd.Context())
		if err != nil {
			return err
		}
		defer rt.Close()
		p, err := rt.svc.RemoveProject(cmd.Context(), args[0])
		if err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Removed project %s (%s)\n", p.Name, p.Root)
		return nil
	},
}

func init() {
	projectAddCmd.Flags().BoolVar(&projectAddCreateSkill, "create-skills-dir", false, "create "+app.SharedProjectDir+" when the folder has no .git or skills dir")
	projectListCmd.Flags().BoolVar(&projectListJSON, "json", false, "print as JSON")
	projectCmd.AddCommand(projectAddCmd, projectListCmd, projectRemoveCmd)
}
