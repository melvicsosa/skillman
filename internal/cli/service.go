package cli

import (
	"fmt"
	"strconv"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/melvicsosa/skillman/internal/app"
)

var serviceStatusJSON bool

var serviceCmd = &cobra.Command{
	Use:   "service",
	Short: "Run the server as a login service (macOS launchd LaunchAgent)",
}

func serviceAction(use, short string, run func(cmd *cobra.Command, rt *env) error) *cobra.Command {
	return &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			rt, err := newRuntime(cmd.Context())
			if err != nil {
				return err
			}
			defer rt.Close()
			return run(cmd, rt)
		},
	}
}

var serviceInstallCmd = serviceAction("install", "Write the LaunchAgent and start it", func(cmd *cobra.Command, rt *env) error {
	info, err := rt.svc.ServiceInstall(cmd.Context())
	if err != nil {
		return err
	}
	fmt.Fprintf(cmd.OutOrStdout(), "installed %s\n  plist: %s\n  logs:  %s\n  url:   %s\n", info.Label, info.UnitPath, info.LogDir, info.URL)
	return nil
})

var serviceUninstallCmd = serviceAction("uninstall", "Stop the service and remove the LaunchAgent", func(cmd *cobra.Command, rt *env) error {
	if _, err := rt.svc.ServiceUninstall(cmd.Context()); err != nil {
		return err
	}
	fmt.Fprintln(cmd.OutOrStdout(), "service uninstalled")
	return nil
})

var serviceStartCmd = serviceAction("start", "Start the installed service", func(cmd *cobra.Command, rt *env) error {
	if err := rt.svc.ServiceStart(cmd.Context()); err != nil {
		return err
	}
	fmt.Fprintln(cmd.OutOrStdout(), "service started")
	return nil
})

var serviceStopCmd = serviceAction("stop", "Stop the service until the next login or `service start`", func(cmd *cobra.Command, rt *env) error {
	if err := rt.svc.ServiceStop(cmd.Context()); err != nil {
		return err
	}
	fmt.Fprintln(cmd.OutOrStdout(), "service stopped")
	return nil
})

var serviceRestartCmd = serviceAction("restart", "Restart the service (picks up a changed port)", func(cmd *cobra.Command, rt *env) error {
	if err := rt.svc.ServiceRestart(cmd.Context()); err != nil {
		return err
	}
	fmt.Fprintln(cmd.OutOrStdout(), "service restarted")
	return nil
})

var serviceStatusCmd = serviceAction("status", "Show whether the service is installed, running and answering", func(cmd *cobra.Command, rt *env) error {
	info, err := rt.svc.ServiceStatus(cmd.Context())
	if err != nil {
		return err
	}
	if serviceStatusJSON {
		return writeJSON(cmd.OutOrStdout(), info)
	}
	printServiceStatus(cmd, info)
	return nil
})

func printServiceStatus(cmd *cobra.Command, info app.ServiceInfo) {
	tw := tabwriter.NewWriter(cmd.OutOrStdout(), 2, 4, 2, ' ', 0)
	yes := func(b bool) string {
		if b {
			return "yes"
		}
		return "no"
	}
	if !info.Supported {
		fmt.Fprintf(tw, "supported\tno (%s; only macOS launchd is implemented)\n", info.Platform)
	} else {
		fmt.Fprintf(tw, "label\t%s\n", info.Label)
		fmt.Fprintf(tw, "installed\t%s\n", yes(info.Installed))
		if info.Installed {
			fmt.Fprintf(tw, "plist\t%s\n", info.UnitPath)
			fmt.Fprintf(tw, "logs\t%s\n", info.LogDir)
		}
		fmt.Fprintf(tw, "running\t%s\n", yes(info.Running))
		if info.PID > 0 {
			fmt.Fprintf(tw, "pid\t%d\n", info.PID)
		}
	}
	fmt.Fprintf(tw, "port\t%d\n", info.Port)
	health := "no"
	if info.Healthy {
		health = "yes (" + info.URL + ")"
		if info.ServerPID > 0 {
			health += " pid " + strconv.Itoa(info.ServerPID)
		}
	}
	fmt.Fprintf(tw, "answering\t%s\n", health)
	fmt.Fprintf(tw, "data dir\t%s\n", info.DataDir)
	tw.Flush()
}

func init() {
	serviceStatusCmd.Flags().BoolVar(&serviceStatusJSON, "json", false, "print as JSON")
	serviceCmd.AddCommand(serviceInstallCmd, serviceUninstallCmd, serviceStartCmd, serviceStopCmd, serviceRestartCmd, serviceStatusCmd)
}
