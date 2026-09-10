package tray

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// LoginLabel is the launchd label of the tray's own login item.
const LoginLabel = "com.melvicsosa.skillman-tray"

// LoginItem writes and removes the LaunchAgent that starts the tray at
// login. It only touches the plist: RunAtLoad makes launchd start the tray at
// the next login, and the running instance is left alone (bootstrapping it
// now would open a second menu bar item, booting it out would quit us).
type LoginItem struct {
	Dir        string
	Executable string
	DataDir    string
}

// NewLoginItem builds the login item for cfg.
func NewLoginItem(cfg Config) LoginItem {
	return LoginItem{Dir: cfg.LaunchAgentsDir, Executable: cfg.Executable, DataDir: cfg.DataDir}
}

// Path is <Dir>/<LoginLabel>.plist.
func (l LoginItem) Path() string { return filepath.Join(l.Dir, LoginLabel+".plist") }

// Installed reports whether the plist exists.
func (l LoginItem) Installed() bool {
	_, err := os.Stat(l.Path())
	return err == nil
}

// Install writes the plist.
func (l LoginItem) Install() error {
	if l.Dir == "" {
		return errors.New("login item: LaunchAgents directory is unknown")
	}
	if l.Executable == "" {
		return errors.New("login item: executable is required")
	}
	if err := os.MkdirAll(l.Dir, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(l.Path(), l.Plist(), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", l.Path(), err)
	}
	return nil
}

// Uninstall removes the plist; a missing file is not an error.
func (l LoginItem) Uninstall() error {
	if err := os.Remove(l.Path()); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("remove %s: %w", l.Path(), err)
	}
	return nil
}

// Plist renders the LaunchAgent: RunAtLoad without KeepAlive, so quitting
// the tray from its menu does not bring it back.
func (l LoginItem) Plist() []byte {
	var b bytes.Buffer
	b.WriteString(xml.Header)
	b.WriteString(`<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">` + "\n")
	b.WriteString("<plist version=\"1.0\">\n<dict>\n")
	fmt.Fprintf(&b, "\t<key>Label</key>\n\t<string>%s</string>\n", esc(LoginLabel))
	b.WriteString("\t<key>ProgramArguments</key>\n\t<array>\n")
	args := []string{l.Executable}
	if l.DataDir != "" {
		args = append(args, "--data-dir", l.DataDir)
	}
	for _, a := range args {
		fmt.Fprintf(&b, "\t\t<string>%s</string>\n", esc(a))
	}
	b.WriteString("\t</array>\n")
	b.WriteString("\t<key>RunAtLoad</key>\n\t<true/>\n")
	b.WriteString("\t<key>ProcessType</key>\n\t<string>Interactive</string>\n")
	b.WriteString("\t<key>LimitLoadToSessionType</key>\n\t<string>Aqua</string>\n")
	if home := os.Getenv(HomeEnv); home != "" {
		fmt.Fprintf(&b, "\t<key>EnvironmentVariables</key>\n\t<dict>\n\t\t<key>%s</key>\n\t\t<string>%s</string>\n\t</dict>\n", HomeEnv, esc(home))
	}
	b.WriteString("</dict>\n</plist>\n")
	return b.Bytes()
}

func esc(s string) string {
	var b bytes.Buffer
	_ = xml.EscapeText(&b, []byte(s))
	return b.String()
}
