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

func driftIssue(t *testing.T, report DoctorReport, name string) Issue {
	t.Helper()
	for _, i := range report.Issues {
		if i.Kind == IssueDrift && i.Name == name {
			return i
		}
	}
	t.Fatalf("no drift issue for %s in %+v", name, report.Issues)
	return Issue{}
}

func TestDoctorDriftListsCopiesAndRepairFromVault(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	f.source("fake:alpha", "alpha", "body A\n")
	f.add("fake:alpha", InstallOptions{Agents: []domain.AgentID{"codex", "opencode"}, Copy: true})
	f.add("fake:alpha", InstallOptions{Agents: []domain.AgentID{"claude-code"}})
	// Drift: the opencode copy is edited.
	edited := f.path(".config/opencode/skills/alpha/SKILL.md")
	if err := os.WriteFile(edited, []byte("---\nname: alpha\ndescription: Description of alpha\n---\nedited\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	report, err := f.svc.Doctor(ctx)
	if err != nil {
		t.Fatal(err)
	}
	issue := driftIssue(t, report, "alpha")
	if issue.VaultRef != "alpha" || !issue.Repairable || len(issue.Copies) != len(issue.SkillIDs) || len(issue.Copies) < 3 {
		t.Fatalf("issue = %+v", issue)
	}
	byAgent := map[string]DriftCopy{}
	for _, c := range issue.Copies {
		byAgent[c.Agent] = c
	}
	if c := byAgent["claude-code"]; !c.IsSymlink || c.Repairable || c.VaultRef != "alpha" || !strings.Contains(c.Reason, "vault") {
		t.Fatalf("symlink copy = %+v", c)
	}
	if c := byAgent["opencode"]; c.IsSymlink || !c.Repairable || c.VaultRef != "" {
		t.Fatalf("edited copy = %+v", c)
	}
	if c := byAgent["codex"]; !c.Repairable || c.VaultRef != "alpha" {
		t.Fatalf("intact copy = %+v", c)
	}

	res, err := f.svc.RepairDrift(ctx, RepairOptions{Name: "alpha", From: RepairSourceVault})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Replaced) != 1 || res.Replaced[0] != f.path(".config/opencode/skills/alpha") {
		t.Fatalf("replaced = %+v", res)
	}
	skipped := map[string]string{}
	for _, s := range res.Skipped {
		skipped[s.Agent] = s.Reason
	}
	if skipped["codex"] != "already identical" || !strings.Contains(skipped["claude-code"], "vault") {
		t.Fatalf("skipped = %+v", res.Skipped)
	}
	b, _ := os.ReadFile(edited)
	if strings.Contains(string(b), "edited") {
		t.Fatal("opencode copy was not replaced")
	}
	report, _ = f.svc.Doctor(ctx)
	if report.Counts[IssueDrift] != 0 {
		t.Fatalf("drift remains: %+v", report.Issues)
	}
	if sk := f.resolve("alpha", "opencode"); sk.VaultRef != "alpha" {
		t.Fatalf("repaired copy not attributed to the vault: %+v", sk)
	}
}

func TestRepairFromAgentAndAdopt(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	// Two hand-made copies, no vault entry.
	f.writeSkill(".agents/skills", "tool", "version one\n")
	f.writeSkill(".config/opencode/skills", "tool", "version two\n")
	f.scan()
	report, err := f.svc.Doctor(ctx)
	if err != nil {
		t.Fatal(err)
	}
	issue := driftIssue(t, report, "tool")
	if issue.VaultRef != "" || !issue.Repairable {
		t.Fatalf("issue = %+v", issue)
	}
	if _, err := f.svc.RepairDrift(ctx, RepairOptions{Name: "tool", From: "claude-code"}); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("repair from agent without copy err = %v", err)
	}
	res, err := f.svc.RepairDrift(ctx, RepairOptions{Name: "tool", From: "opencode", To: []domain.AgentID{"codex"}})
	if err != nil || len(res.Replaced) != 1 || res.Replaced[0] != f.path(".agents/skills/tool") {
		t.Fatalf("repair = %+v err=%v", res, err)
	}
	b, _ := os.ReadFile(f.path(".agents/skills/tool/SKILL.md"))
	if !strings.Contains(string(b), "version two") {
		t.Fatalf("codex copy = %s", b)
	}

	entry, err := f.svc.VaultAdopt(ctx, "tool", "opencode", true)
	if err != nil {
		t.Fatal(err)
	}
	if entry.Source.Type != domain.SourceLocal || entry.Source.Ref != f.path(".config/opencode/skills/tool") || entry.Path != filepath.Join(f.dataDir, "vault", "tool") {
		t.Fatalf("adopted = %+v", entry)
	}
	if readLink(t, f.path(".config/opencode/skills/tool")) != entry.Path {
		t.Fatal("--link did not replace the agent dir with a symlink")
	}
	var sidecar domain.VaultEntry
	sb, _ := os.ReadFile(entry.Path + ".vault.json")
	if json.Unmarshal(sb, &sidecar) != nil || sidecar.Source.Type != domain.SourceLocal {
		t.Fatalf("sidecar = %s", sb)
	}
	if sk := f.resolve("tool", "opencode"); sk.VaultRef != "tool" || !sk.IsSymlink {
		t.Fatalf("opencode after adopt = %+v", sk)
	}
	if sk := f.resolve("tool", "codex"); sk.VaultRef != "tool" {
		t.Fatalf("codex copy should match the vault hash: %+v", sk)
	}
	if _, err := f.svc.VaultAdopt(ctx, "tool", "codex", false); !errors.Is(err, domain.ErrExists) {
		t.Fatalf("adopt twice err = %v", err)
	}
	if _, err := f.svc.VaultAdopt(ctx, "missing", "codex", false); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("adopt missing err = %v", err)
	}
	// Now the vault can be the sync source everywhere.
	sync, err := f.svc.Sync(ctx, SyncOptions{Names: []string{"tool"}})
	if err != nil || sync.Conflicts != 0 || sync.Linked == 0 {
		t.Fatalf("sync adopted = %+v err=%v", sync, err)
	}
}
