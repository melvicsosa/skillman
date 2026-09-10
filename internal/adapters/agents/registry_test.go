package agents

import (
	"regexp"
	"strings"
	"testing"
)

var slugRE = regexp.MustCompile(`^[a-z0-9-]+$`)

func TestRegistryInvariants(t *testing.T) {
	if len(Registry) != 7 {
		t.Fatalf("expected 7 agents, got %d", len(Registry))
	}
	seenID := map[string]bool{}
	seenName := map[string]bool{}
	for _, s := range Registry {
		id := string(s.ID)
		if !slugRE.MatchString(id) {
			t.Errorf("agent id %q is not a slug", id)
		}
		if seenID[id] {
			t.Errorf("duplicate agent id %q", id)
		}
		seenID[id] = true
		if seenName[s.Name] {
			t.Errorf("duplicate agent name %q", s.Name)
		}
		seenName[s.Name] = true
		if len(s.GlobalDirs) == 0 {
			t.Errorf("agent %q has no global dirs", id)
		}
		for _, d := range s.GlobalDirs {
			if !strings.HasPrefix(d, "~/") {
				t.Errorf("agent %q global dir %q must start with ~/", id, d)
			}
		}
		for _, d := range s.ProjectDirs {
			if strings.HasPrefix(d, "/") || strings.HasPrefix(d, "~") {
				t.Errorf("agent %q project dir %q must be relative", id, d)
			}
		}
	}
}

func TestExpandHome(t *testing.T) {
	t.Setenv("HOME", "/tmp/fakehome")
	if got := ExpandHome("~/.claude/skills"); got != "/tmp/fakehome/.claude/skills" {
		t.Fatalf("ExpandHome = %q", got)
	}
	if got := ExpandHome("/abs/path"); got != "/abs/path" {
		t.Fatalf("ExpandHome abs = %q", got)
	}
}

func TestExistsUsesExpandedDirs(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	s := Spec{ID: "x", Name: "X", GlobalDirs: []string{"~/skills-a", "~/skills-b"}}
	if s.Exists() {
		t.Fatal("Exists should be false before dirs are created")
	}
	if err := mkdir(home + "/skills-b"); err != nil {
		t.Fatal(err)
	}
	if !s.Exists() {
		t.Fatal("Exists should be true when any global dir exists")
	}
}
