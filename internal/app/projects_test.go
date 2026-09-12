package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/melvicsosa/skillman/internal/domain"
)

func TestRegisterProjectRejectsPlainDir(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	plain := f.path("work/plain")
	if err := os.MkdirAll(plain, 0o755); err != nil {
		t.Fatal(err)
	}

	_, _, err := f.svc.RegisterProject(ctx, plain, RegisterProjectOptions{})
	if !errors.Is(err, domain.ErrNotAProject) {
		t.Fatalf("err = %v, want ErrNotAProject", err)
	}
	if _, statErr := os.Stat(filepath.Join(plain, ".agents", "skills")); !os.IsNotExist(statErr) {
		t.Fatalf("skills dir must not be created without the option: %v", statErr)
	}
	if list, _ := f.svc.ListProjects(ctx); len(list) != 0 {
		t.Fatalf("projects = %+v, want none", list)
	}
}

func TestRegisterProjectCreatesSkillsDir(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	plain := f.path("work/plain")
	if err := os.MkdirAll(plain, 0o755); err != nil {
		t.Fatal(err)
	}

	p, _, err := f.svc.RegisterProject(ctx, plain, RegisterProjectOptions{CreateSkillsDir: true})
	if err != nil {
		t.Fatal(err)
	}
	if p.Root != plain || p.Name != "plain" {
		t.Fatalf("project = %+v", p)
	}
	info, err := os.Stat(filepath.Join(plain, ".agents", "skills"))
	if err != nil || !info.IsDir() {
		t.Fatalf(".agents/skills not created: info=%v err=%v", info, err)
	}
}

func TestRegisterProjectAcceptsGitDir(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	root := f.path("work/repo")
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}

	p, _, err := f.svc.RegisterProject(ctx, root, RegisterProjectOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if p.Root != root {
		t.Fatalf("project = %+v", p)
	}
	if _, statErr := os.Stat(filepath.Join(root, ".agents", "skills")); !os.IsNotExist(statErr) {
		t.Fatalf("skills dir must not be created for a git repo: %v", statErr)
	}
}
