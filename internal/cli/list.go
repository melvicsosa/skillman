package cli

import (
	"fmt"
	"path/filepath"
	"strconv"
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
		views := httpadapter.ToSkillViews(skills)
		if usage, err := rt.svc.Usage(cmd.Context()); err == nil {
			httpadapter.ApplyUsage(views, usage.Skills)
		}
		if listJSON {
			return writeJSON(cmd.OutOrStdout(), views)
		}
		if len(views) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "No skills cached. Run `skillman scan`.")
			return nil
		}
		withUsage := false
		for _, v := range views {
			if v.UsageCount > 0 {
				withUsage = true
				break
			}
		}
		tw := tabwriter.NewWriter(cmd.OutOrStdout(), 2, 4, 2, ' ', 0)
		used := func(v httpadapter.SkillView) string {
			if !withUsage {
				return ""
			}
			if v.UsageCount == 0 {
				return "\t-"
			}
			return "\t" + strconv.Itoa(v.UsageCount)
		}
		header := "NAME\tAGENT\tSCOPE\tSTATE\tVERSION"
		if withUsage {
			header += "\tUSED"
		}
		fmt.Fprintln(tw, header+"\tDESCRIPTION")
		for _, v := range views {
			scope := "global"
			if v.ProjectRoot != "" {
				scope = filepath.Base(v.ProjectRoot)
			}
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s%s\t%s\n", v.Name, v.Agent, scope, v.State, v.Version, used(v), truncate(v.Description, 60))
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
