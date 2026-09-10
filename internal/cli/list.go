package cli

import (
	"fmt"
	"path/filepath"
	"text/tabwriter"

	"github.com/spf13/cobra"

	httpadapter "github.com/melvicsosa/skillman/internal/adapters/http"
	"github.com/melvicsosa/skillman/internal/domain"
)

var (
	listAgent    string
	listProject  string
	listGlobal   bool
	listDisabled bool
	listJSON     bool
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List cached skills (run scan first)",
	RunE: func(cmd *cobra.Command, _ []string) error {
		if listGlobal && listProject != "" {
			return fmt.Errorf("--global and --project are mutually exclusive")
		}
		rt, err := newRuntime(cmd.Context())
		if err != nil {
			return err
		}
		defer rt.Close()

		f := domain.SkillFilter{Agent: domain.AgentID(listAgent)}
		if listGlobal {
			f.Scope = domain.GlobalScope
		}
		if listProject != "" {
			abs, err := filepath.Abs(listProject)
			if err != nil {
				return err
			}
			f.Scope = domain.ProjectScope(filepath.Clean(abs))
		}
		if listDisabled {
			f.State = domain.SkillDisabled
		}
		skills, err := rt.svc.ListSkills(cmd.Context(), f)
		if err != nil {
			return err
		}
		if listJSON {
			return writeJSON(cmd.OutOrStdout(), httpadapter.ToSkillViews(skills))
		}
		if len(skills) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "No skills cached. Run `skillman scan`.")
			return nil
		}
		tw := tabwriter.NewWriter(cmd.OutOrStdout(), 2, 4, 2, ' ', 0)
		fmt.Fprintln(tw, "NAME\tAGENT\tSCOPE\tSTATE\tVERSION\tDESCRIPTION")
		for _, s := range skills {
			scope := "global"
			if root := s.Scope.ProjectRoot(); root != "" {
				scope = filepath.Base(root)
			}
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n", s.Name, s.AgentID, scope, s.State, s.Version, truncate(s.Description, 60))
		}
		return tw.Flush()
	},
}

func init() {
	listCmd.Flags().StringVar(&listAgent, "agent", "", "only skills of this agent id")
	listCmd.Flags().StringVar(&listProject, "project", "", "only skills of this project root")
	listCmd.Flags().BoolVar(&listGlobal, "global", false, "only global skills")
	listCmd.Flags().BoolVar(&listDisabled, "disabled", false, "only disabled skills")
	listCmd.Flags().BoolVar(&listJSON, "json", false, "print as JSON")
}
