package cli

import (
	"fmt"
	"path/filepath"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/melvicsosa/skillman/internal/domain"
)

var (
	vaultListJSON       bool
	vaultRemoveForce    bool
	vaultInstallTo      []string
	vaultInstallAll     bool
	vaultInstallCopy    bool
	vaultInstallProj    string
	vaultInstallCommand bool
	vaultUninstAgent    string
	vaultUninstProj     string
)

var vaultCmd = &cobra.Command{
	Use:   "vault",
	Short: "Manage the vault of downloaded skills",
}

var vaultListCmd = &cobra.Command{
	Use:   "list",
	Short: "List vault entries and where they are installed",
	RunE: func(cmd *cobra.Command, _ []string) error {
		rt, err := newRuntime(cmd.Context())
		if err != nil {
			return err
		}
		defer rt.Close()
		list, err := rt.svc.VaultList(cmd.Context())
		if err != nil {
			return err
		}
		if vaultListJSON {
			return writeJSON(cmd.OutOrStdout(), list)
		}
		if len(list) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "Vault is empty. Use `skillman add <ref>` or `skillman import-lock`.")
			return nil
		}
		tw := tabwriter.NewWriter(cmd.OutOrStdout(), 2, 4, 2, ' ', 0)
		fmt.Fprintln(tw, "NAME\tSOURCE\tREF\tINSTALLED IN\tSPEC\tCONVERTED")
		for _, e := range list {
			spec := "ok"
			if !e.Spec.Valid {
				spec = fmt.Sprintf("%d issues", len(e.Spec.Issues))
			}
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n", e.Name, e.Source.Type, truncate(e.Source.Ref, 50), linkSummary(e.Links), spec, e.ConvertedFrom)
		}
		return tw.Flush()
	},
}

func linkSummary(links []domain.Skill) string {
	if len(links) == 0 {
		return "-"
	}
	seen := map[string]bool{}
	var parts []string
	for _, l := range links {
		label := string(l.AgentID)
		if root := l.Scope.ProjectRoot(); root != "" {
			label += "@" + filepath.Base(root)
		}
		if !seen[label] {
			seen[label] = true
			parts = append(parts, label)
		}
	}
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += ","
		}
		out += p
	}
	return out
}

var vaultRemoveCmd = &cobra.Command{
	Use:   "remove <name> [--force]",
	Short: "Delete a vault entry (refuses while installed unless --force)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		rt, err := newRuntime(cmd.Context())
		if err != nil {
			return err
		}
		defer rt.Close()
		res, err := rt.svc.VaultRemove(cmd.Context(), args[0], vaultRemoveForce)
		if err != nil {
			return err
		}
		for _, p := range res.Unlinked {
			fmt.Fprintf(cmd.OutOrStdout(), "  unlinked %s\n", p)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Removed %s from the vault\n", res.Name)
		return nil
	},
}

var vaultUpdateCmd = &cobra.Command{
	Use:   "update [<name>]",
	Short: "Re-fetch vault entries from their source (all when no name is given)",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		rt, err := newRuntime(cmd.Context())
		if err != nil {
			return err
		}
		defer rt.Close()
		names := args
		if len(names) == 0 {
			list, err := rt.svc.VaultList(cmd.Context())
			if err != nil {
				return err
			}
			for _, e := range list {
				names = append(names, e.Name)
			}
		}
		var failed int
		for _, name := range names {
			res, err := rt.svc.VaultUpdate(cmd.Context(), name)
			if err != nil {
				failed++
				fmt.Fprintf(cmd.ErrOrStderr(), "%s: %v\n", name, err)
				continue
			}
			if res.Updated {
				fmt.Fprintf(cmd.OutOrStdout(), "%s: updated (%d copies refreshed)\n", name, len(res.CopiesRefresh))
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "%s: up to date\n", name)
			}
			for _, w := range res.Warnings {
				fmt.Fprintln(cmd.ErrOrStderr(), "warning:", w)
			}
		}
		if failed > 0 {
			return fmt.Errorf("%d of %d entries failed to update", failed, len(names))
		}
		return nil
	},
}

var vaultInstallCmd = &cobra.Command{
	Use:   "install <name> --to <agent>... [--all] [--project <path>] [--copy] [--command]",
	Short: "Install a vault entry into agents",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		rt, err := newRuntime(cmd.Context())
		if err != nil {
			return err
		}
		defer rt.Close()
		opts := installOptions(vaultInstallTo, vaultInstallAll, vaultInstallProj, vaultInstallCopy, vaultInstallCommand)
		links, err := rt.svc.InstallFromVault(cmd.Context(), args[0], opts)
		if err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Installed %s\n", args[0])
		printLinks(cmd, links)
		return nil
	},
}

var vaultUninstallCmd = &cobra.Command{
	Use:   "uninstall <name> --agent <id> [--project <path>]",
	Short: "Remove a vault entry's link from one agent (the vault keeps it)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if vaultUninstAgent == "" {
			return fmt.Errorf("--agent is required")
		}
		rt, err := newRuntime(cmd.Context())
		if err != nil {
			return err
		}
		defer rt.Close()
		removed, err := rt.svc.UninstallFromVault(cmd.Context(), args[0], domain.AgentID(vaultUninstAgent), vaultUninstProj)
		if err != nil {
			return err
		}
		for _, p := range removed {
			fmt.Fprintf(cmd.OutOrStdout(), "  removed %s\n", p)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Uninstalled %s from %s\n", args[0], vaultUninstAgent)
		return nil
	},
}

func init() {
	vaultListCmd.Flags().BoolVar(&vaultListJSON, "json", false, "print as JSON")
	vaultRemoveCmd.Flags().BoolVar(&vaultRemoveForce, "force", false, "unlink from every agent first")
	vaultInstallCmd.Flags().StringArrayVar(&vaultInstallTo, "to", nil, "agent id to install into (repeatable)")
	vaultInstallCmd.Flags().BoolVar(&vaultInstallAll, "all", false, "install into every enabled agent")
	vaultInstallCmd.Flags().BoolVar(&vaultInstallCopy, "copy", false, "copy instead of symlinking")
	vaultInstallCmd.Flags().StringVar(&vaultInstallProj, "project", "", "install into this project root")
	vaultInstallCmd.Flags().BoolVar(&vaultInstallCommand, "command", false, "also write a .claude/commands/<name>.md stub")
	vaultUninstallCmd.Flags().StringVar(&vaultUninstAgent, "agent", "", "agent id (required)")
	vaultUninstallCmd.Flags().StringVar(&vaultUninstProj, "project", "", "project root for a project install")
	vaultCmd.AddCommand(vaultListCmd, vaultRemoveCmd, vaultUpdateCmd, vaultInstallCmd, vaultUninstallCmd)
}
