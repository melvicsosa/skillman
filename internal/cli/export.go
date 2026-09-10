package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/melvicsosa/skillman/internal/app"
)

var (
	exportName        string
	exportVersion     string
	exportDescription string
	exportJSON        bool
)

var exportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export vault skills in other formats",
}

var exportPluginCmd = &cobra.Command{
	Use:   "plugin <out-dir> --name <plugin-name> [--version 0.1.0] [--description ...] <skill>...",
	Short: "Write a Claude Code plugin (.claude-plugin/plugin.json + skills/) from vault skills",
	Args:  cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		if exportName == "" {
			return fmt.Errorf("--name is required")
		}
		rt, err := newRuntime(cmd.Context())
		if err != nil {
			return err
		}
		defer rt.Close()
		res, err := rt.svc.ExportPlugin(cmd.Context(), app.ExportOptions{
			OutDir: args[0], Name: exportName, Version: exportVersion, Description: exportDescription, Skills: args[1:],
		})
		if err != nil {
			return err
		}
		if exportJSON {
			return writeJSON(cmd.OutOrStdout(), res)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Exported %d skills to %s\n", len(res.Skills), res.Dir)
		for _, sk := range res.Skills {
			fmt.Fprintf(cmd.OutOrStdout(), "  skills/%s\n", sk)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Manifest: %s\n", res.Manifest)
		return nil
	},
}

func init() {
	exportPluginCmd.Flags().StringVar(&exportName, "name", "", "plugin name (required)")
	exportPluginCmd.Flags().StringVar(&exportVersion, "version", app.DefaultPluginVersion, "plugin version")
	exportPluginCmd.Flags().StringVar(&exportDescription, "description", "", "plugin description")
	exportPluginCmd.Flags().BoolVar(&exportJSON, "json", false, "print as JSON")
	exportCmd.AddCommand(exportPluginCmd)
}
