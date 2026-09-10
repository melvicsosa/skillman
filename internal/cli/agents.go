package cli

import (
	"fmt"
	"text/tabwriter"

	"github.com/spf13/cobra"

	httpadapter "github.com/melvicsosa/skillman/internal/adapters/http"
	"github.com/melvicsosa/skillman/internal/domain"
)

var agentsJSON bool

var agentsCmd = &cobra.Command{
	Use:   "agents",
	Short: "List supported agents with their enabled flag and skill count",
	RunE: func(cmd *cobra.Command, _ []string) error {
		rt, err := newRuntime(cmd.Context())
		if err != nil {
			return err
		}
		defer rt.Close()
		list, err := rt.svc.ListAgents(cmd.Context())
		if err != nil {
			return err
		}
		if agentsJSON {
			return writeJSON(cmd.OutOrStdout(), httpadapter.ToAgentViews(list))
		}
		tw := tabwriter.NewWriter(cmd.OutOrStdout(), 2, 4, 2, ' ', 0)
		fmt.Fprintln(tw, "ID\tNAME\tENABLED\tSKILLS\tGLOBAL DIR\tSTATUS")
		for _, a := range list {
			status := "missing"
			if a.Exists {
				status = "found"
			}
			fmt.Fprintf(tw, "%s\t%s\t%v\t%d\t%s\t%s\n", a.ID, a.Name, a.Enabled, a.SkillCount, a.GlobalDirs[0], status)
		}
		return tw.Flush()
	},
}

var agentCmd = &cobra.Command{
	Use:   "agent",
	Short: "Enable or disable an agent (hides it from scans; no files are touched)",
}

func newAgentToggleCmd(enabled bool) *cobra.Command {
	verb := "disable"
	if enabled {
		verb = "enable"
	}
	return &cobra.Command{
		Use:   verb + " <agent-id>",
		Short: verb + " an agent",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			rt, err := newRuntime(cmd.Context())
			if err != nil {
				return err
			}
			defer rt.Close()
			a, err := rt.svc.ToggleAgent(cmd.Context(), domain.AgentID(args[0]), enabled)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Agent %s (%s) %sd\n", a.ID, a.Name, verb)
			return nil
		},
	}
}

func init() {
	agentsCmd.Flags().BoolVar(&agentsJSON, "json", false, "print as JSON")
	agentCmd.AddCommand(newAgentToggleCmd(true), newAgentToggleCmd(false))
}
