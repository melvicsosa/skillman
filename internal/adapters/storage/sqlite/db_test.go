package sqlite

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/melvicsosa/skillman/internal/domain"
)

func TestOpenMigrateAndSettings(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "nested", "skillman.db")

	db, err := Open(ctx, path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	v1, err := db.MigrationVersion(ctx)
	if err != nil {
		t.Fatalf("MigrationVersion: %v", err)
	}
	if v1 < 1 {
		t.Fatalf("expected migration version >= 1, got %d", v1)
	}

	// Migrating again must be a no-op.
	if err := db.Migrate(ctx); err != nil {
		t.Fatalf("second Migrate: %v", err)
	}
	v2, err := db.MigrationVersion(ctx)
	if err != nil {
		t.Fatalf("MigrationVersion after re-run: %v", err)
	}
	if v1 != v2 {
		t.Fatalf("migration version changed on re-run: %d -> %d", v1, v2)
	}

	settings := NewSettingsRepo(db)
	if _, ok, err := settings.GetSetting(ctx, "port"); err != nil || ok {
		t.Fatalf("expected missing setting, got ok=%v err=%v", ok, err)
	}
	if err := settings.SetSetting(ctx, "port", "3010"); err != nil {
		t.Fatalf("SetSetting: %v", err)
	}
	if err := settings.SetSetting(ctx, "port", "4000"); err != nil {
		t.Fatalf("SetSetting overwrite: %v", err)
	}
	got, ok, err := settings.GetSetting(ctx, "port")
	if err != nil || !ok || got != "4000" {
		t.Fatalf("GetSetting = %q ok=%v err=%v, want 4000", got, ok, err)
	}
}

func TestAgentRepoRoundTrip(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, filepath.Join(t.TempDir(), "skillman.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	repo := NewAgentRepo(db)
	a := domain.Agent{
		ID: "claude-code", Name: "Claude Code", Enabled: true,
		GlobalDirs: []string{"/home/x/.claude/skills"}, ProjectDirs: []string{".claude/skills"},
		SupportsSymlink: true,
	}
	if err := repo.UpsertAgent(ctx, a); err != nil {
		t.Fatalf("UpsertAgent: %v", err)
	}
	a.Enabled = false
	if err := repo.UpsertAgent(ctx, a); err != nil {
		t.Fatalf("UpsertAgent update: %v", err)
	}
	list, err := repo.ListAgents(ctx)
	if err != nil {
		t.Fatalf("ListAgents: %v", err)
	}
	if len(list) != 1 || list[0].ID != "claude-code" || list[0].Enabled || len(list[0].GlobalDirs) != 1 {
		t.Fatalf("unexpected agents: %+v", list)
	}
}
