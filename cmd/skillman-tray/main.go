// Command skillman-tray is the macOS menu bar companion of skillman: it
// shows whether the server runs, opens the UI, starts, stops and restarts
// the server, switches the port and manages its own launch-at-login item.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/melvicsosa/skillman/internal/app/version"
	"github.com/melvicsosa/skillman/internal/tray"
)

func main() {
	fs := flag.NewFlagSet("skillman-tray", flag.ExitOnError)
	dataDir := fs.String("data-dir", "", "data directory (default $SKILLMAN_DATA_DIR or ~/.skillman)")
	installLogin := fs.Bool("install-login", false, "write the launch-at-login LaunchAgent and exit")
	uninstallLogin := fs.Bool("uninstall-login", false, "remove the launch-at-login LaunchAgent and exit")
	showVersion := fs.Bool("version", false, "print the version and exit")
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "usage: skillman-tray [--data-dir DIR] [--install-login | --uninstall-login]")
		fs.PrintDefaults()
	}
	_ = fs.Parse(os.Args[1:])

	if *showVersion {
		fmt.Println("skillman-tray", version.Version)
		return
	}
	cfg, err := tray.Load(*dataDir)
	if err != nil {
		fail(err)
	}
	login := tray.NewLoginItem(cfg)
	switch {
	case *installLogin && *uninstallLogin:
		fail(fmt.Errorf("--install-login and --uninstall-login are exclusive"))
	case *installLogin:
		if err := login.Install(); err != nil {
			fail(err)
		}
		fmt.Println("launch at login enabled:", login.Path())
		return
	case *uninstallLogin:
		if err := login.Uninstall(); err != nil {
			fail(err)
		}
		fmt.Println("launch at login disabled")
		return
	}
	if err := tray.Run(cfg); err != nil {
		fail(err)
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(1)
}
