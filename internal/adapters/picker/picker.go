// Package picker opens native OS dialogs on behalf of the local server,
// which can return absolute paths that a browser never exposes.
package picker

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/melvicsosa/skillman/internal/domain"
)

// Runner executes a command and returns its standard output. Errors should
// carry the command's standard error text so cancellation can be detected.
type Runner func(ctx context.Context, name string, args ...string) ([]byte, error)

// Picker implements domain.FolderPicker.
type Picker struct {
	run Runner
}

var _ domain.FolderPicker = Picker{}

// New returns a Picker backed by real processes.
func New() Picker { return Picker{run: execRunner} }

// NewWithRunner returns a Picker using run instead of spawning processes.
func NewWithRunner(run Runner) Picker { return Picker{run: run} }

// execRunner runs the command and folds stderr into the returned error.
func execRunner(ctx context.Context, name string, args ...string) ([]byte, error) {
	out, err := exec.CommandContext(ctx, name, args...).Output()
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) && len(ee.Stderr) > 0 {
			return out, fmt.Errorf("%s: %s: %w", name, strings.TrimSpace(string(ee.Stderr)), err)
		}
		return out, fmt.Errorf("%s: %w", name, err)
	}
	return out, nil
}

// pickWithOsascript shows the AppleScript "choose folder" dialog. It is
// platform-neutral so it can be tested everywhere with a fake Runner.
func pickWithOsascript(ctx context.Context, run Runner, prompt string) (string, error) {
	out, err := run(ctx, "osascript",
		"-e", "activate",
		"-e", `set f to choose folder with prompt "`+escapeAppleScript(prompt)+`"`,
		"-e", "POSIX path of f",
	)
	if err != nil {
		if isCanceled(err) {
			return "", domain.ErrCanceled
		}
		return "", fmt.Errorf("choose folder: %w", err)
	}
	path := cleanPath(string(out))
	if path == "" {
		return "", errors.New("choose folder: empty result")
	}
	return path, nil
}

// isCanceled reports whether osascript failed because the user pressed
// Cancel (AppleScript error -128, "User canceled.").
func isCanceled(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "(-128)") || strings.Contains(msg, "User canceled")
}

// cleanPath drops the trailing newline and slash, keeping "/" for root.
func cleanPath(s string) string {
	s = strings.TrimRight(s, "\r\n")
	if s == "" {
		return ""
	}
	if t := strings.TrimRight(s, "/"); t != "" {
		return t
	}
	return "/"
}

// escapeAppleScript escapes s for use inside an AppleScript string literal.
func escapeAppleScript(s string) string {
	return strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(s)
}
