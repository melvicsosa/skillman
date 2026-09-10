//go:build darwin

package tray

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"sync"
	"time"

	"fyne.io/systray"
)

// PortChoices are the ports offered in the Port submenu.
var PortChoices = []int{3010, 3011, 3012, 4010, 8010}

const refreshEvery = 5 * time.Second

// Run shows the menu bar item and blocks until Quit is chosen.
func Run(cfg Config) error {
	a := &app{ctl: NewController(cfg), login: NewLoginItem(cfg)}
	systray.Run(a.onReady, a.onExit)
	return nil
}

type app struct {
	ctl   *Controller
	login LoginItem

	mu sync.Mutex
	st Status

	status, open, start, stop, restart, install, uninstall, custom, loginItem, quit *systray.MenuItem
	ports                                                                           map[int]*systray.MenuItem
	kick                                                                            chan struct{}
}

func (a *app) onReady() {
	systray.SetTemplateIcon(Icon44(), Icon44())
	systray.SetTooltip("skillman")

	a.status = systray.AddMenuItem("Checking…", "skillman server status")
	a.status.Disable()
	a.open = systray.AddMenuItem("Open skillman", "Open the web UI in the browser")
	systray.AddSeparator()
	a.start = systray.AddMenuItem("Start", "Start the server")
	a.stop = systray.AddMenuItem("Stop", "Stop the server")
	a.restart = systray.AddMenuItem("Restart", "Restart the server")
	a.install = systray.AddMenuItem("Install background service", "Run skillman at login with launchd")
	a.uninstall = systray.AddMenuItem("Uninstall background service", "Remove the launchd LaunchAgent")
	systray.AddSeparator()
	portMenu := systray.AddMenuItem("Port", "Port the UI listens on")
	a.ports = make(map[int]*systray.MenuItem, len(PortChoices))
	for _, p := range PortChoices {
		a.ports[p] = portMenu.AddSubMenuItem(strconv.Itoa(p), "Listen on port "+strconv.Itoa(p))
	}
	a.custom = portMenu.AddSubMenuItem("Custom…", "Set another port in Settings")
	systray.AddSeparator()
	a.loginItem = systray.AddMenuItemCheckbox("Launch at login", "Start skillman-tray when you log in", a.login.Installed())
	systray.AddSeparator()
	a.quit = systray.AddMenuItem("Quit skillman-tray", "Quit the menu bar app (the server keeps running)")

	a.kick = make(chan struct{}, 1)
	go a.loop()
}

func (a *app) onExit() {}

// loop serialises every action and refresh on one goroutine.
func (a *app) loop() {
	a.refresh()
	ticker := time.NewTicker(refreshEvery)
	defer ticker.Stop()
	portClicks := make(chan int)
	for p, item := range a.ports {
		go func(p int, ch <-chan struct{}) {
			for range ch {
				portClicks <- p
			}
		}(p, item.ClickedCh)
	}
	for {
		select {
		case <-ticker.C:
			a.refresh()
		case <-a.kick:
			a.refresh()
		case <-a.open.ClickedCh:
			a.openURL(a.current().URL)
		case <-a.start.ClickedCh:
			a.do("start", func(ctx context.Context, st Status) error { return a.ctl.Start(ctx, st) })
		case <-a.stop.ClickedCh:
			a.do("stop", func(ctx context.Context, st Status) error { return a.ctl.Stop(ctx, st) })
		case <-a.restart.ClickedCh:
			a.do("restart", func(ctx context.Context, st Status) error { return a.ctl.Restart(ctx, st) })
		case <-a.install.ClickedCh:
			a.do("install service", func(ctx context.Context, _ Status) error { return a.ctl.Install(ctx) })
		case <-a.uninstall.ClickedCh:
			a.do("uninstall service", func(ctx context.Context, _ Status) error { return a.ctl.Uninstall(ctx) })
		case p := <-portClicks:
			a.do("set port", func(ctx context.Context, st Status) error { return a.ctl.SetPort(ctx, st, p) })
		case <-a.custom.ClickedCh:
			a.openURL(a.current().URL + "/settings")
		case <-a.loginItem.ClickedCh:
			a.toggleLogin()
		case <-a.quit.ClickedCh:
			systray.Quit()
			return
		}
	}
}

func (a *app) current() Status {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.st
}

// refresh polls the CLI and the health endpoint and redraws the menu.
func (a *app) refresh() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	st, err := a.ctl.Status(ctx)
	if err != nil {
		a.status.SetTitle("Status unavailable")
		a.status.SetTooltip(err.Error())
		fmt.Fprintln(os.Stderr, "status:", err)
		return
	}
	if st.URL == "" {
		st.URL = "http://localhost:" + strconv.Itoa(st.Port)
	}
	a.mu.Lock()
	a.st = st
	a.mu.Unlock()
	a.draw(st)
}

func (a *app) draw(st Status) {
	a.status.SetTitle(st.Label())
	a.status.SetTooltip("data dir " + st.DataDir)
	setEnabled(a.open, st.Healthy)
	setEnabled(a.start, !st.Healthy)
	setEnabled(a.stop, st.Healthy)
	setEnabled(a.restart, st.Healthy)
	if st.Installed {
		a.install.Hide()
		a.uninstall.Show()
	} else {
		a.uninstall.Hide()
		a.install.Show()
	}
	setEnabled(a.install, st.Supported)
	for p, item := range a.ports {
		if p == st.Port {
			item.Check()
		} else {
			item.Uncheck()
		}
	}
	setEnabled(a.custom, st.Healthy)
	if a.login.Installed() {
		a.loginItem.Check()
	} else {
		a.loginItem.Uncheck()
	}
}

func setEnabled(item *systray.MenuItem, on bool) {
	if on {
		item.Enable()
	} else {
		item.Disable()
	}
}

// do runs one action against the last known status, then refreshes. A
// server that was just (re)started is given a moment to answer so the menu
// does not flash "Stopped".
func (a *app) do(name string, fn func(context.Context, Status) error) {
	st := a.current()
	a.status.SetTitle(capitalize(name) + "…")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := fn(ctx, st); err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", name, err)
		a.status.SetTitle("Could not " + name)
		a.status.SetTooltip(err.Error())
		time.Sleep(1500 * time.Millisecond)
		a.refresh()
		return
	}
	if name != "stop" && name != "uninstall service" {
		if next, err := a.ctl.Status(ctx); err == nil {
			a.ctl.WaitHealthy(ctx, next.Port, 8*time.Second)
		}
	} else {
		time.Sleep(500 * time.Millisecond)
	}
	a.refresh()
}

func (a *app) toggleLogin() {
	var err error
	if a.login.Installed() {
		err = a.login.Uninstall()
	} else {
		err = a.login.Install()
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "launch at login:", err)
	}
	if a.login.Installed() {
		a.loginItem.Check()
	} else {
		a.loginItem.Uncheck()
	}
}

func (a *app) openURL(url string) {
	if err := exec.Command("open", url).Start(); err != nil {
		fmt.Fprintln(os.Stderr, "open:", err)
	}
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return string(s[0]-'a'+'A') + s[1:]
}
