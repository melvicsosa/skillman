package cli

import (
	"fmt"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

var (
	searchSource string
	searchJSON   bool
)

var searchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search skill registries (skills.sh, GitHub and Claude marketplaces)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		rt, err := newRuntime(cmd.Context())
		if err != nil {
			return err
		}
		defer rt.Close()
		res, err := rt.svc.Search(cmd.Context(), args[0], searchSource)
		if err != nil {
			return err
		}
		if searchJSON {
			return writeJSON(cmd.OutOrStdout(), res)
		}
		out := cmd.OutOrStdout()
		if len(res.Results) == 0 {
			fmt.Fprintln(out, "No results.")
		} else {
			tw := tabwriter.NewWriter(out, 2, 4, 2, ' ', 0)
			fmt.Fprintln(tw, "NAME\tSOURCE\tINSTALLS\tREGISTRY\tADD WITH")
			for _, r := range res.Results {
				fmt.Fprintf(tw, "%s\t%s\t%d\t%s\t%s\n", r.Name, r.Source, r.Installs, r.Registry, r.Ref)
				if r.Description != "" {
					fmt.Fprintf(tw, "  %s\t\t\t\t\n", truncate(r.Description, 100))
				}
			}
			if err := tw.Flush(); err != nil {
				return err
			}
		}
		for _, e := range res.Errors {
			fmt.Fprintln(cmd.ErrOrStderr(), "warning:", e)
		}
		return nil
	},
}

func init() {
	searchCmd.Flags().StringVar(&searchSource, "source", "", "only this registry: skillssh, github or marketplace (default all)")
	searchCmd.Flags().BoolVar(&searchJSON, "json", false, "print as JSON")
}
