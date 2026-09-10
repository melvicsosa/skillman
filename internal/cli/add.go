package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/melvicsosa/skillman/internal/app"
	"github.com/melvicsosa/skillman/internal/domain"
)

var (
	addTo      []string
	addAll     bool
	addCopy    bool
	addProject string
	addCommand bool
	addJSON    bool
)

var addCmd = &cobra.Command{
	Use:   "add <ref> [--to <agent>]... [--all] [--copy] [--project <path>] [--command]",
	Short: "Download a skill into the vault and install it into agents",
	Long: `Download a skill into the vault and install it into the given agents.

<ref> may be owner/repo, owner/repo/path[@ref], a github.com URL (including
/tree/<ref>/<path>), skillssh:<owner>/<repo>/<skill>, a local path (skill dir,
plugin dir, .mdc rule, .md command, .zip/.tar.gz) or a .zip/.tar.gz/SKILL.md URL.
Without --to or --all the skill is only stored in the vault.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		rt, err := newRuntime(cmd.Context())
		if err != nil {
			return err
		}
		defer rt.Close()
		opts := installOptions(addTo, addAll, addProject, addCopy, addCommand)
		res, err := rt.svc.Add(cmd.Context(), args[0], opts)
		if err != nil {
			return err
		}
		if addJSON {
			return writeJSON(cmd.OutOrStdout(), res)
		}
		out := cmd.OutOrStdout()
		for _, e := range res.Entries {
			from := ""
			if e.ConvertedFrom != "" {
				from = " (converted from " + e.ConvertedFrom + ")"
			}
			fmt.Fprintf(out, "Stored %s in vault: %s%s\n", e.Name, e.Path, from)
		}
		printLinks(cmd, res.Links)
		for _, w := range res.Warnings {
			fmt.Fprintln(cmd.ErrOrStderr(), "warning:", w)
		}
		return nil
	},
}

func init() {
	addCmd.Flags().StringArrayVar(&addTo, "to", nil, "agent id to install into (repeatable)")
	addCmd.Flags().BoolVar(&addAll, "all", false, "install into every enabled agent")
	addCmd.Flags().BoolVar(&addCopy, "copy", false, "copy instead of symlinking into the vault")
	addCmd.Flags().StringVar(&addProject, "project", "", "install into this project root instead of globally")
	addCmd.Flags().BoolVar(&addCommand, "command", false, "also write a .claude/commands/<name>.md stub for Claude Code")
	addCmd.Flags().BoolVar(&addJSON, "json", false, "print as JSON")
}

func installOptions(to []string, all bool, project string, copy, command bool) app.InstallOptions {
	opts := app.InstallOptions{All: all, Root: project, Copy: copy, Command: command}
	for _, id := range to {
		opts.Agents = append(opts.Agents, domain.AgentID(id))
	}
	return opts
}

func printLinks(cmd *cobra.Command, links []app.LinkResult) {
	for _, l := range links {
		how := "linked"
		if l.Copied {
			how = "copied"
		}
		if l.Existing {
			how = "already installed"
		}
		fmt.Fprintf(cmd.OutOrStdout(), "  %s %s: %s\n", l.Agent, how, l.Path)
	}
}
