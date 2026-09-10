package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/melvicsosa/skillman/internal/domain"
)

// newSkillToggleCmd builds `enable` / `disable <skill-name> --agent <id> [--project <path>]`.
func newSkillToggleCmd(enable bool) *cobra.Command {
	verb := "disable"
	if enable {
		verb = "enable"
	}
	var (
		agent   string
		project string
	)
	cmd := &cobra.Command{
		Use:   verb + " <skill-name> --agent <id> [--project <path>]",
		Short: verb + " a skill for one agent (global by default)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if agent == "" {
				return fmt.Errorf("--agent is required")
			}
			rt, err := newRuntime(cmd.Context())
			if err != nil {
				return err
			}
			defer rt.Close()
			sk, err := rt.svc.ResolveSkill(cmd.Context(), args[0], domain.AgentID(agent), project)
			if err != nil {
				return err
			}
			before := sk.State
			if enable {
				sk, err = rt.svc.EnableSkill(cmd.Context(), sk.ID)
			} else {
				sk, err = rt.svc.DisableSkill(cmd.Context(), sk.ID)
			}
			if err != nil {
				return err
			}
			if before == sk.State {
				fmt.Fprintf(cmd.OutOrStdout(), "%s is already %s\n", sk.Name, sk.State)
				return nil
			}
			if enable {
				fmt.Fprintf(cmd.OutOrStdout(), "Enabled %s for %s: restored to %s\n", sk.Name, sk.AgentID, sk.Path)
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "Disabled %s for %s: moved to %s\n", sk.Name, sk.AgentID, sk.QuarantinePath)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&agent, "agent", "", "agent id (required)")
	cmd.Flags().StringVar(&project, "project", "", "project root for a project-scoped skill")
	return cmd
}
