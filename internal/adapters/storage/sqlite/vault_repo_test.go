package sqlite

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/melvicsosa/skillman/internal/domain"
)

func TestVaultRepoRoundTrip(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	repo := NewVaultRepo(db)

	if v, err := db.MigrationVersion(ctx); err != nil || v < 3 {
		t.Fatalf("migration version = %d err=%v", v, err)
	}
	e := domain.VaultEntry{
		Name: "alpha", Path: "/data/vault/alpha", ContentHash: "h1",
		Source:     domain.VaultSource{Type: domain.SourceGitHub, Ref: "o/r/skills/alpha", URL: "https://github.com/o/r", Subpath: "skills/alpha", Commit: "abc123"},
		SourceHash: "s1", ConvertedFrom: "claude-command",
		Spec:        domain.SpecReport{Valid: false, Issues: []string{"missing description"}},
		InstalledAt: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
	}
	if err := repo.UpsertVault(ctx, e); err != nil {
		t.Fatal(err)
	}
	got, err := repo.GetVault(ctx, "alpha")
	if err != nil {
		t.Fatal(err)
	}
	if got.Source != e.Source || got.ContentHash != "h1" || got.SourceHash != "s1" || got.ConvertedFrom != "claude-command" {
		t.Fatalf("GetVault = %+v", got)
	}
	if got.Spec.Valid || len(got.Spec.Issues) != 1 || !got.InstalledAt.Equal(e.InstalledAt) || !got.UpdatedAt.Equal(e.InstalledAt) {
		t.Fatalf("GetVault spec/times = %+v", got)
	}

	e.ContentHash = "h2"
	e.Spec = domain.SpecReport{Valid: true}
	if err := repo.UpsertVault(ctx, e); err != nil {
		t.Fatal(err)
	}
	list, err := repo.ListVault(ctx)
	if err != nil || len(list) != 1 || list[0].ContentHash != "h2" || !list[0].Spec.Valid || list[0].Spec.Issues == nil {
		t.Fatalf("ListVault = %+v err=%v", list, err)
	}
	if err := repo.DeleteVault(ctx, "alpha"); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.GetVault(ctx, "alpha"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("after delete err = %v", err)
	}
	if err := repo.DeleteVault(ctx, "alpha"); err != nil {
		t.Fatalf("second delete err = %v", err)
	}
}
