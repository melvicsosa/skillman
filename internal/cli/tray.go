package cli

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/spf13/cobra"
)

const trayBinary = "skillman-tray"

var (
	trayLogin   bool
	trayNoLogin bool
)

var trayCmd = &cobra.Command{
	Use:   "tray",
	Short: "Start the macOS menu bar app (skillman-tray)",
	Long: "Starts skillman-tray detached and returns. With --login or --no-login it only " +
		"enables or disables the tray's launch-at-login item (skillman-tray --install-login / --uninstall-login).",
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		if trayLogin && trayNoLogin {
			return errors.New("--login and --no-login are exclusive")
		}
		bin, err := findTrayBinary()
		if err != nil {
			return err
		}
		args := []string{}
		if dir, err := resolveDataDir(); err == nil && dataDir != "" {
			args = append(args, "--data-dir", dir)
		}
		switch {
		case trayLogin:
			return runTray(cmd, bin, append(args, "--install-login")...)
		case trayNoLogin:
			return runTray(cmd, bin, append(args, "--uninstall-login")...)
		}
		c := exec.Command(bin, args...)
		c.SysProcAttr = detachedAttr()
		if err := c.Start(); err != nil {
			return fmt.Errorf("start %s: %w", bin, err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "skillman-tray started (pid %d)\n", c.Process.Pid)
		return c.Process.Release()
	},
}

func runTray(cmd *cobra.Command, bin string, args ...string) error {
	c := exec.Command(bin, args...)
	c.Stdout = cmd.OutOrStdout()
	c.Stderr = cmd.ErrOrStderr()
	return c.Run()
}

// findTrayBinary looks next to the running executable first, then in PATH.
func findTrayBinary() (string, error) {
	if exe, err := os.Executable(); err == nil {
		if resolved, err := filepath.EvalSymlinks(exe); err == nil {
			exe = resolved
		}
		sibling := filepath.Join(filepath.Dir(exe), trayBinary)
		if info, err := os.Stat(sibling); err == nil && !info.IsDir() {
			return sibling, nil
		}
	}
	if p, err := exec.LookPath(trayBinary); err == nil {
		return p, nil
	}
	hint := "install it with `brew install melvicsosa/tap/skillman` or download the macOS release archive"
	if runtime.GOOS != "darwin" {
		hint = "the menu bar app is macOS only"
	}
	return "", fmt.Errorf("%s not found next to skillman or in PATH (%s)", trayBinary, hint)
}

func init() {
	trayCmd.Flags().BoolVar(&trayLogin, "login", false, "enable launch at login for the tray and exit")
	trayCmd.Flags().BoolVar(&trayNoLogin, "no-login", false, "disable launch at login for the tray and exit")
	rootCmd.AddCommand(trayCmd)
}
