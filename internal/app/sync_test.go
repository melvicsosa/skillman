package app

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/melvicsosa/skillman/internal/domain"
)

func linkStatus(report SyncReport, name, agent string) string {
	for _, e := range report.Entries {
		if e.Name != name {
			continue
		}
		for _, l := range e.Links {
			if l.Agent == agent {
				return l.Status
			}
		}
	}
	return ""
}

func TestSyncFansOutAndReportsConflicts(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	f.source("fake:alpha", "alpha", "body A\n")
	f.add("fake:alpha", InstallOptions{})
	// A foreign skill with the same name in opencode must not be touched.
	foreign := f.writeSkill(".config/opencode/skills", "alpha", "someone else\n")

	dry, err := f.svc.Sync(ctx, SyncOptions{All: true, DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if !dry.DryRun || dry.Linked == 0 || dry.Conflicts != 1 || exists(f.path(".agents/skills/alpha")) {
		t.Fatalf("dry run = %+v", dry)
	}

	report, err := f.svc.Sync(ctx, SyncOptions{All: true})
	if err != nil {
		t.Fatal(err)
	}
	if report.Conflicts != 1 || report.Linked == 0 || linkStatus(report, "alpha", "opencode") != SyncConflict {
		t.Fatalf("sync = %+v", report)
	}
	vaultDir := filepath.Join(f.dataDir, "vault", "alpha")
	for _, rel := range []string{".agents/skills/alpha", ".claude/skills/alpha", ".cursor/skills/alpha"} {
		if readLink(t, f.path(rel)) != vaultDir {
			t.Fatalf("%s does not link into the vault", rel)
		}
	}
	if b, _ := os.ReadFile(filepath.Join(foreign, "SKILL.md")); !strings.Contains(string(b), "someone else") {
		t.Fatal("foreign skill was overwritten")
	}
	if sk := f.resolve("alpha", "codex"); sk.VaultRef != "alpha" {
		t.Fatalf("rescan after sync: %+v", sk)
	}

	// Second run: everything already there, the conflict persists.
	again, err := f.svc.Sync(ctx, SyncOptions{Names: []string{"alpha"}})
	if err != nil || again.Linked != 0 || again.Already == 0 || again.Conflicts != 1 {
		t.Fatalf("second sync = %+v err=%v", again, err)
	}
	if _, err := f.svc.Sync(ctx, SyncOptions{}); err == nil {
		t.Fatal("sync without names or --all should fail")
	}
	if _, err := f.svc.Sync(ctx, SyncOptions{Names: []string{"nope"}}); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("unknown entry err = %v", err)
	}
}

func TestSyncCopiesWhenEntryWasCopied(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	f.source("fake:beta", "beta", "body B\n")
	f.add("fake:beta", InstallOptions{Agents: []domain.AgentID{"codex"}, Copy: true})

	report, err := f.svc.Sync(ctx, SyncOptions{Names: []string{"beta"}})
	if err != nil {
		t.Fatal(err)
	}
	if linkStatus(report, "beta", "codex") != SyncAlready || linkStatus(report, "beta", "claude-code") != SyncCopied {
		t.Fatalf("copy sync = %+v", report)
	}
	info, err := os.Lstat(f.path(".claude/skills/beta"))
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		t.Fatalf("claude copy = %v %v", info, err)
	}
}

func TestSyncIntoProject(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	f.source("fake:gamma", "gamma", "body G\n")
	f.add("fake:gamma", InstallOptions{})
	root := filepath.Join(f.home, "proj")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	report, err := f.svc.Sync(ctx, SyncOptions{All: true, Root: root})
	if err != nil {
		t.Fatal(err)
	}
	if report.Conflicts != 0 || !exists(filepath.Join(root, ".agents/skills/gamma")) || !exists(filepath.Join(root, ".claude/skills/gamma")) {
		t.Fatalf("project sync = %+v", report)
	}
	for _, e := range report.Entries {
		for _, l := range e.Links {
			if l.Scope != string(domain.ProjectScope(root)) {
				t.Fatalf("link scope = %q", l.Scope)
			}
		}
	}
	if exists(f.path(".agents/skills/gamma")) {
		t.Fatal("project sync touched the global dir")
	}
}

func TestAutoSyncRunsOnScan(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	f.source("fake:delta", "delta", "body D\n")
	f.add("fake:delta", InstallOptions{})
	sum := f.scan()
	if sum.Sync != nil || exists(f.path(".agents/skills/delta")) {
		t.Fatalf("auto-sync ran without the flag: %+v", sum)
	}
	entry, err := f.svc.VaultSetAutoSync(ctx, "delta", true)
	if err != nil || !entry.AutoSync {
		t.Fatalf("set auto-sync: %+v %v", entry, err)
	}
	var sidecar domain.VaultEntry
	b, _ := os.ReadFile(filepath.Join(f.dataDir, "vault", "delta.vault.json"))
	if json.Unmarshal(b, &sidecar) != nil || !sidecar.AutoSync {
		t.Fatalf("sidecar = %s", b)
	}
	sum = f.scan()
	if sum.Sync == nil || sum.Sync.Linked == 0 || readLink(t, f.path(".agents/skills/delta")) != filepath.Join(f.dataDir, "vault", "delta") {
		t.Fatalf("scan summary = %+v", sum)
	}
	if sk := f.resolve("delta", "codex"); sk.VaultRef != "delta" {
		t.Fatalf("same scan did not pick up the link: %+v", sk)
	}
	// The flag survives a database wipe (sidecar is the truth).
	f.wipeDB()
	f.open()
	f.scan()
	got, err := f.svc.VaultGet(ctx, "delta")
	if err != nil || !got.AutoSync {
		t.Fatalf("after wipe = %+v %v", got, err)
	}
	if _, err := f.svc.VaultSetAutoSync(ctx, "delta", false); err != nil {
		t.Fatal(err)
	}
	if sum := f.scan(); sum.Sync != nil {
		t.Fatalf("auto-sync still runs after off: %+v", sum)
	}
}
