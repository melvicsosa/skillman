// Package tray implements the skillman-tray macOS menu bar app. It never
// opens the database itself: everything goes through the sibling skillman
// binary (`service status --json`, `config set port`, `serve`) and the HTTP
// API, so the core CLI stays CGO-free and the tray stays a thin remote.
package tray

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/melvicsosa/skillman/internal/brewpath"
)

// Environment variables the tray honours.
const (
	// DataDirEnv overrides the data directory when --data-dir is not given.
	DataDirEnv = "SKILLMAN_DATA_DIR"
	// HomeEnv mirrors the main binary: LaunchAgents live under this home.
	HomeEnv = "SKILLMAN_HOME"
)

const (
	defaultDataDirName = ".skillman"
	skillmanBinary     = "skillman"
)

// ErrSkillmanNotFound is returned when no skillman binary can be located.
var ErrSkillmanNotFound = errors.New("skillman binary not found next to skillman-tray or in PATH")

// Config is everything Run needs.
type Config struct {
	// DataDir is the skillman data directory (database, pid file, logs).
	DataDir string
	// Skillman is the path of the skillman CLI used for every action.
	Skillman string
	// LaunchAgentsDir is where the "Launch at login" plist is written.
	LaunchAgentsDir string
	// Executable is the tray binary itself, recorded in the login item.
	Executable string
}

// Load resolves the configuration from flag, environment and executable
// location. dataDirFlag wins over SKILLMAN_DATA_DIR, which wins over the
// default ~/.skillman.
func Load(dataDirFlag string) (Config, error) {
	dir, err := ResolveDataDir(dataDirFlag)
	if err != nil {
		return Config{}, err
	}
	exe, err := os.Executable()
	if err != nil {
		return Config{}, fmt.Errorf("resolve executable: %w", err)
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	// Keep paths stable across `brew upgrade` (the login item persists exe,
	// and the tray outlives the Cellar dir of the version it started from).
	exe = brewpath.Stable(exe)
	bin, err := FindSkillman(filepath.Dir(exe))
	if err != nil {
		return Config{}, err
	}
	bin = brewpath.Stable(bin)
	return Config{DataDir: dir, Skillman: bin, LaunchAgentsDir: LaunchAgentsDir(), Executable: exe}, nil
}

// ResolveDataDir picks the data dir from flag, environment or default.
func ResolveDataDir(flag string) (string, error) {
	dir := flag
	if dir == "" {
		dir = os.Getenv(DataDirEnv)
	}
	if dir != "" {
		return filepath.Abs(dir)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home dir: %w", err)
	}
	return filepath.Join(home, defaultDataDirName), nil
}

// FindSkillman looks for the skillman binary next to dir first, then in PATH.
func FindSkillman(dir string) (string, error) {
	if dir != "" {
		sibling := filepath.Join(dir, skillmanBinary)
		if info, err := os.Stat(sibling); err == nil && !info.IsDir() && info.Mode()&0o111 != 0 {
			return sibling, nil
		}
	}
	if p, err := exec.LookPath(skillmanBinary); err == nil {
		return p, nil
	}
	return "", ErrSkillmanNotFound
}

// LaunchAgentsDir is ~/Library/LaunchAgents under SKILLMAN_HOME when set,
// so isolated runs never touch the real user domain.
func LaunchAgentsDir() string {
	home := os.Getenv(HomeEnv)
	if home == "" {
		var err error
		if home, err = os.UserHomeDir(); err != nil {
			return ""
		}
	}
	return filepath.Join(home, "Library", "LaunchAgents")
}
