package service

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/melvicsosa/skillman/internal/domain"
)

// Runner executes launchctl. Tests substitute a fake.
type Runner func(ctx context.Context, args ...string) (string, error)

// Launchd manages a per-user LaunchAgent.
type Launchd struct {
	Label    string
	PlistDir string
	DataDir  string
	UID      int
	Run      Runner
}

var _ domain.ServiceManager = (*Launchd)(nil)

// NewLaunchd returns a manager writing <plistDir>/<label>.plist and logging
// under <dataDir>/logs.
func NewLaunchd(plistDir, dataDir string) *Launchd {
	return &Launchd{Label: domain.ServiceLabel, PlistDir: plistDir, DataDir: dataDir, UID: os.Getuid(), Run: runLaunchctl}
}

func runLaunchctl(ctx context.Context, args ...string) (string, error) {
	out, err := exec.CommandContext(ctx, "launchctl", args...).CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			msg = err.Error()
		}
		return string(out), fmt.Errorf("launchctl %s: %s", strings.Join(args, " "), msg)
	}
	return string(out), nil
}

// PlistPath is where the LaunchAgent definition lives.
func (l *Launchd) PlistPath() string { return filepath.Join(l.PlistDir, l.Label+".plist") }

// LogDir is where launchd redirects stdout and stderr.
func (l *Launchd) LogDir() string { return filepath.Join(l.DataDir, "logs") }

func (l *Launchd) target() string { return fmt.Sprintf("gui/%d/%s", l.UID, l.Label) }

func (l *Launchd) domainTarget() string { return fmt.Sprintf("gui/%d", l.UID) }

// Install writes the plist and loads it. An already loaded agent is
// booted out first so launchd picks up the new definition.
func (l *Launchd) Install(ctx context.Context, spec domain.ServiceSpec) error {
	if spec.Executable == "" {
		return errors.New("service install: executable is required")
	}
	if err := os.MkdirAll(l.LogDir(), 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(l.PlistDir, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(l.PlistPath(), l.Plist(spec), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", l.PlistPath(), err)
	}
	if l.loaded(ctx) {
		_, _ = l.Run(ctx, "bootout", l.target())
	}
	return l.load(ctx)
}

func (l *Launchd) load(ctx context.Context) error {
	if _, err := l.Run(ctx, "bootstrap", l.domainTarget(), l.PlistPath()); err == nil {
		return nil
	}
	// Older launchctl or a domain that rejects bootstrap: fall back to the legacy verb.
	if _, err := l.Run(ctx, "load", "-w", l.PlistPath()); err != nil {
		return err
	}
	return nil
}

// Uninstall stops the agent and removes the plist.
func (l *Launchd) Uninstall(ctx context.Context) error {
	if l.loaded(ctx) {
		if _, err := l.Run(ctx, "bootout", l.target()); err != nil {
			if _, err2 := l.Run(ctx, "unload", "-w", l.PlistPath()); err2 != nil {
				return err
			}
		}
	}
	if err := os.Remove(l.PlistPath()); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("remove %s: %w", l.PlistPath(), err)
	}
	return nil
}

// Start loads the agent when needed and kickstarts it.
func (l *Launchd) Start(ctx context.Context) error {
	if !l.installed() {
		return fmt.Errorf("service is not installed (run `skillman service install`): %w", domain.ErrNotFound)
	}
	if !l.loaded(ctx) {
		return l.load(ctx)
	}
	_, err := l.Run(ctx, "kickstart", l.target())
	return err
}

// Stop boots the agent out of the user domain; the plist stays so it runs
// again at next login or `service start`.
func (l *Launchd) Stop(ctx context.Context) error {
	if !l.loaded(ctx) {
		return nil
	}
	_, err := l.Run(ctx, "bootout", l.target())
	return err
}

// Restart kills the running instance and starts a fresh one. launchd keeps
// the job registered, so this also works when the caller is the instance
// being restarted.
func (l *Launchd) Restart(ctx context.Context) error {
	if !l.installed() {
		return fmt.Errorf("service is not installed (run `skillman service install`): %w", domain.ErrNotFound)
	}
	if !l.loaded(ctx) {
		return l.load(ctx)
	}
	_, err := l.Run(ctx, "kickstart", "-k", l.target())
	return err
}

// Status reports whether the plist exists and what launchd says about it.
func (l *Launchd) Status(ctx context.Context) (domain.ServiceState, error) {
	st := domain.ServiceState{Supported: true, Platform: "darwin", Label: l.Label, UnitPath: l.PlistPath(), LogDir: l.LogDir()}
	st.Installed = l.installed()
	out, err := l.Run(ctx, "print", l.target())
	if err != nil {
		return st, nil
	}
	st.Running, st.PID = parsePrint(out)
	return st, nil
}

func (l *Launchd) installed() bool {
	_, err := os.Stat(l.PlistPath())
	return err == nil
}

func (l *Launchd) loaded(ctx context.Context) bool {
	_, err := l.Run(ctx, "print", l.target())
	return err == nil
}

// parsePrint extracts "state = running" and "pid = N" from launchctl print.
func parsePrint(out string) (running bool, pid int) {
	for _, line := range strings.Split(out, "\n") {
		k, v, ok := strings.Cut(strings.TrimSpace(line), "=")
		if !ok {
			continue
		}
		switch strings.TrimSpace(k) {
		case "state":
			running = strings.TrimSpace(v) == "running"
		case "pid":
			pid, _ = strconv.Atoi(strings.TrimSpace(v))
		}
	}
	if pid > 0 {
		running = true
	}
	return running, pid
}

// Plist renders the LaunchAgent definition for spec.
func (l *Launchd) Plist(spec domain.ServiceSpec) []byte {
	var b bytes.Buffer
	b.WriteString(xml.Header)
	b.WriteString(`<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">` + "\n")
	b.WriteString("<plist version=\"1.0\">\n<dict>\n")
	key := func(k string) { fmt.Fprintf(&b, "\t<key>%s</key>\n", esc(k)) }
	str := func(v string) { fmt.Fprintf(&b, "\t<string>%s</string>\n", esc(v)) }
	key("Label")
	str(l.Label)
	key("ProgramArguments")
	b.WriteString("\t<array>\n")
	for _, a := range append([]string{spec.Executable}, spec.Args...) {
		fmt.Fprintf(&b, "\t\t<string>%s</string>\n", esc(a))
	}
	b.WriteString("\t</array>\n")
	key("RunAtLoad")
	b.WriteString("\t<true/>\n")
	key("KeepAlive")
	b.WriteString("\t<dict>\n\t\t<key>SuccessfulExit</key>\n\t\t<false/>\n\t</dict>\n")
	key("ProcessType")
	str("Background")
	key("StandardOutPath")
	str(filepath.Join(l.LogDir(), "skillman.log"))
	key("StandardErrorPath")
	str(filepath.Join(l.LogDir(), "skillman.err.log"))
	if len(spec.Env) > 0 {
		key("EnvironmentVariables")
		b.WriteString("\t<dict>\n")
		keys := make([]string, 0, len(spec.Env))
		for k := range spec.Env {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			fmt.Fprintf(&b, "\t\t<key>%s</key>\n\t\t<string>%s</string>\n", esc(k), esc(spec.Env[k]))
		}
		b.WriteString("\t</dict>\n")
	}
	b.WriteString("</dict>\n</plist>\n")
	return b.Bytes()
}

func esc(s string) string {
	var b bytes.Buffer
	_ = xml.EscapeText(&b, []byte(s))
	return b.String()
}
