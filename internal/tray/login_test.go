package tray

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoginItemInstallUninstall(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "LaunchAgents")
	item := LoginItem{Dir: dir, Executable: "/opt/skillman-tray", DataDir: "/data & dir"}
	if item.Installed() {
		t.Fatal("installed before Install")
	}
	if err := item.Install(); err != nil {
		t.Fatal(err)
	}
	if !item.Installed() {
		t.Fatal("not installed after Install")
	}
	raw, err := os.ReadFile(filepath.Join(dir, LoginLabel+".plist"))
	if err != nil {
		t.Fatal(err)
	}
	got := string(raw)
	for _, want := range []string{
		"<string>" + LoginLabel + "</string>",
		"<string>/opt/skillman-tray</string>",
		"<string>--data-dir</string>",
		"<string>/data &amp; dir</string>",
		"<key>RunAtLoad</key>\n\t<true/>",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("plist missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "KeepAlive") {
		t.Error("login item must not use KeepAlive")
	}
	if err := item.Uninstall(); err != nil {
		t.Fatal(err)
	}
	if item.Installed() {
		t.Fatal("still installed after Uninstall")
	}
	if err := item.Uninstall(); err != nil {
		t.Fatalf("second Uninstall: %v", err)
	}
}

func TestLoginItemRequiresExecutable(t *testing.T) {
	if err := (LoginItem{Dir: t.TempDir()}).Install(); err == nil {
		t.Fatal("expected error without executable")
	}
}

func TestResolveDataDir(t *testing.T) {
	t.Setenv(DataDirEnv, "/env/dir")
	if got, _ := ResolveDataDir("/flag/dir"); got != "/flag/dir" {
		t.Errorf("flag: got %q", got)
	}
	if got, _ := ResolveDataDir(""); got != "/env/dir" {
		t.Errorf("env: got %q", got)
	}
	t.Setenv(DataDirEnv, "")
	home, _ := os.UserHomeDir()
	if got, _ := ResolveDataDir(""); got != filepath.Join(home, defaultDataDirName) {
		t.Errorf("default: got %q", got)
	}
}

func TestFindSkillmanPrefersSibling(t *testing.T) {
	dir := t.TempDir()
	sibling := filepath.Join(dir, skillmanBinary)
	if err := os.WriteFile(sibling, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got, err := FindSkillman(dir); err != nil || got != sibling {
		t.Fatalf("got %q, %v", got, err)
	}
	t.Setenv("PATH", t.TempDir())
	if _, err := FindSkillman(t.TempDir()); err == nil {
		t.Fatal("expected ErrSkillmanNotFound")
	}
}
