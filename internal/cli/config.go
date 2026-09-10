package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/melvicsosa/skillman/internal/app"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Read and write settings (" + strings.Join(app.SettingKeys, ", ") + ")",
}

var configGetCmd = &cobra.Command{
	Use:   "get <key>",
	Short: "Print a setting",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		rt, err := newRuntime(cmd.Context())
		if err != nil {
			return err
		}
		defer rt.Close()
		v, err := rt.svc.ConfigGet(cmd.Context(), args[0])
		if err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), v)
		return nil
	},
}

var configSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Store a setting (an empty value clears it)",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		rt, err := newRuntime(cmd.Context())
		if err != nil {
			return err
		}
		defer rt.Close()
		if err := rt.svc.ConfigSet(cmd.Context(), args[0], args[1]); err != nil {
			return err
		}
		if args[0] == app.SettingPort {
			fmt.Fprintf(cmd.OutOrStdout(), "port set to %s. Restart the server (`skillman service restart`) to apply it.\n", strings.TrimSpace(args[1]))
			return nil
		}
		fmt.Fprintf(cmd.OutOrStdout(), "%s updated\n", args[0])
		return nil
	},
}

func init() {
	configCmd.AddCommand(configGetCmd, configSetCmd)
}
