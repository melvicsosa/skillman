// Package cli wires the cobra command tree. Commands stay thin and delegate
// to adapters and use cases.
package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

const (
	defaultDataDirName = ".skillman"
	dbFileName         = "skillman.db"
)

var dataDir string

var rootCmd = &cobra.Command{
	Use:           "skillman",
	Short:         "Cross-agent AI skill manager with a local UI",
	Long:          "skillman discovers, enables, disables and installs AI agent skills across Claude Code, Codex, Cursor, Gemini CLI, Antigravity and OpenCode.",
	SilenceUsage:  true,
	SilenceErrors: true,
}

func init() {
	rootCmd.PersistentFlags().StringVar(&dataDir, "data-dir", "", "data directory (default ~/"+defaultDataDirName+")")
	rootCmd.AddCommand(versionCmd, serveCmd, doctorCmd)
}

// Execute runs the root command and exits non-zero on error.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

// resolveDataDir returns the effective data dir, expanding the default under
// the user's home when --data-dir was not given.
func resolveDataDir() (string, error) {
	if dataDir != "" {
		abs, err := filepath.Abs(dataDir)
		if err != nil {
			return "", err
		}
		return abs, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home dir: %w", err)
	}
	return filepath.Join(home, defaultDataDirName), nil
}

func dbPath(dir string) string { return filepath.Join(dir, dbFileName) }
