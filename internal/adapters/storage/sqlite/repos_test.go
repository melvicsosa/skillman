package sqlite

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/melvicsosa/skillman/internal/domain"
)

func openTestDB(t *testing.T) *DB {
	t.Helper()
	db, err := Open(context.Background(), filepath.Join(t.TempDir(), "skillman.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := NewAgentRepo(db).UpsertAgent(context.Background(), domain.Agent{ID: "claude-code", Name: "Claude Code", Enabled: true}); err != nil {
		t.Fatal(err)
	}
	if err := NewAgentRepo(db).UpsertAgent(context.Background(), domain.Agent{ID: "codex", Name: "Codex", Enabled: true}); err != nil {
		t.Fatal(err)
	}
	return db
}

func skill(agent domain.AgentID, scope domain.Scope, path, name string) domain.Skill {
	return domain.Skill{
		ID: domain.SkillID(agent, scope, path), AgentID: agent, Scope: scope, Path: path, Name: name,
		Description: "Description of " + name, State: domain.SkillEnabled, ContentHash: "h-" + name,
	}
}

func TestSkillRepoUpsertListGetUpdate(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	repo := NewSkillRepo(db)

	a := skill("claude-code", domain.GlobalScope, "/h/.claude/skills/alpha", "alpha")
	b := skill("codex", domain.GlobalScope, "/h/.agents/skills/beta", "beta")
	if err := repo.UpsertMany(ctx, []domain.Skill{a, b}); err != nil {
		t.Fatal(err)
	}
	a.Description = "changed"
	if err := repo.UpsertMany(ctx, []domain.Skill{a}); err != nil {
		t.Fatal(err)
	}

	all, err := repo.List(ctx, domain.SkillFilter{})
	if err != nil || len(all) != 2 {
		t.Fatalf("list all = %d err=%v", len(all), err)
	}
	got, err := repo.GetByID(ctx, a.ID)
	if err != nil || got.Description != "changed" || got.State != domain.SkillEnabled {
		t.Fatalf("GetByID = %+v err=%v", got, err)
	}
	if _, err := repo.GetByID(ctx, "nope"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("missing id err = %v", err)
	}

	tests := []struct {
		name string
		f    domain.SkillFilter
		want int
	}{
		{"by agent", domain.SkillFilter{Agent: "codex"}, 1},
		{"by scope", domain.SkillFilter{Scope: domain.GlobalScope}, 2},
		{"by project scope", domain.SkillFilter{Scope: domain.ProjectScope("/x")}, 0},
		{"by query name", domain.SkillFilter{Query: "BET"}, 1},
		{"by query description", domain.SkillFilter{Query: "changed"}, 1},
		{"by query like escape", domain.SkillFilter{Query: "%"}, 0},
		{"by state", domain.SkillFilter{State: domain.SkillDisabled}, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := repo.List(ctx, tt.f)
			if err != nil || len(got) != tt.want {
				t.Fatalf("got %d err=%v", len(got), err)
			}
		})
	}

	if err := repo.UpdateState(ctx, b.ID, domain.SkillDisabled, "/q/codex/beta"); err != nil {
		t.Fatal(err)
	}
	got, _ = repo.GetByID(ctx, b.ID)
	if got.State != domain.SkillDisabled || got.QuarantinePath != "/q/codex/beta" {
		t.Fatalf("after UpdateState = %+v", got)
	}
	if err := repo.UpdateState(ctx, "nope", domain.SkillEnabled, ""); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("UpdateState missing = %v", err)
	}
}

func TestSkillRepoMarkMissingKeepsDisabled(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	repo := NewSkillRepo(db)

	keep := skill("codex", domain.GlobalScope, "/h/.agents/skills/keep", "keep")
	gone := skill("codex", domain.GlobalScope, "/h/.agents/skills/gone", "gone")
	disabled := skill("codex", domain.GlobalScope, "/h/.agents/skills/off", "off")
	disabled.State = domain.SkillDisabled
	other := skill("claude-code", domain.GlobalScope, "/h/.claude/skills/other", "other")
	proj := skill("codex", domain.ProjectScope("/p"), "/p/.agents/skills/proj", "proj")
	if err := repo.UpsertMany(ctx, []domain.Skill{keep, gone, disabled, other, proj}); err != nil {
		t.Fatal(err)
	}
	n, err := repo.MarkMissing(ctx, "codex", domain.GlobalScope, []string{keep.ID})
	if err != nil || n != 1 {
		t.Fatalf("removed %d err=%v", n, err)
	}
	all, _ := repo.List(ctx, domain.SkillFilter{})
	names := map[string]bool{}
	for _, s := range all {
		names[s.Name] = true
	}
	if names["gone"] || !names["keep"] || !names["off"] || !names["other"] || !names["proj"] {
		t.Fatalf("remaining = %v", names)
	}
	// Running with nothing seen removes everything enabled in that scope.
	n, err = repo.MarkMissing(ctx, "codex", domain.GlobalScope, nil)
	if err != nil || n != 1 {
		t.Fatalf("second pass removed %d err=%v", n, err)
	}
}

func TestProjectRepoAndCascade(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	projects := NewProjectRepo(db)
	skills := NewSkillRepo(db)

	p, err := projects.Add(ctx, domain.Project{Root: "/p/one", Name: "one"})
	if err != nil || p.ID == 0 {
		t.Fatalf("Add = %+v err=%v", p, err)
	}
	if _, err := projects.Add(ctx, domain.Project{Root: "/p/one", Name: "dup"}); !errors.Is(err, domain.ErrExists) {
		t.Fatalf("duplicate err = %v", err)
	}
	got, err := projects.GetByRoot(ctx, "/p/one")
	if err != nil || got.ID != p.ID || got.Name != "one" || got.RegisteredAt.IsZero() {
		t.Fatalf("GetByRoot = %+v err=%v", got, err)
	}
	if _, err := projects.GetByRoot(ctx, "/nope"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("GetByRoot missing = %v", err)
	}

	s := skill("codex", domain.ProjectScope("/p/one"), "/p/one/.agents/skills/x", "x")
	s.ProjectID = &p.ID
	if err := skills.UpsertMany(ctx, []domain.Skill{s}); err != nil {
		t.Fatal(err)
	}
	list, _ := projects.List(ctx)
	if len(list) != 1 {
		t.Fatalf("List = %+v", list)
	}
	if err := projects.Remove(ctx, p.ID); err != nil {
		t.Fatal(err)
	}
	if err := projects.Remove(ctx, p.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("Remove twice = %v", err)
	}
	if left, _ := skills.List(ctx, domain.SkillFilter{}); len(left) != 0 {
		t.Fatalf("project skills not cascaded: %+v", left)
	}
}

func TestScanRepo(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	repo := NewScanRepo(db)
	id, err := repo.Start(ctx)
	if err != nil || id == 0 {
		t.Fatalf("Start = %d err=%v", id, err)
	}
	if err := repo.Finish(ctx, id, 3, []string{"boom"}); err != nil {
		t.Fatal(err)
	}
	var found int
	var errs, finished string
	if err := db.QueryRowContext(ctx, `SELECT skills_found, errors, finished_at FROM scans WHERE id = ?`, id).Scan(&found, &errs, &finished); err != nil {
		t.Fatal(err)
	}
	if found != 3 || errs != `["boom"]` || finished == "" {
		t.Fatalf("scan row = %d %s %s", found, errs, finished)
	}
}
