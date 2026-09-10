package agents

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/melvicsosa/skillman/internal/domain"
)

func writeSkill(t *testing.T, dir, name string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.MkdirAll(p, 0o755); err != nil {
		t.Fatal(err)
	}
	content := "---\nname: " + name + "\ndescription: d\n---\n"
	if err := os.WriteFile(filepath.Join(p, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestHomeOverride(t *testing.T) {
	t.Setenv(HomeEnv, "/tmp/fakehome")
	if got := ExpandHome("~/.claude/skills"); got != "/tmp/fakehome/.claude/skills" {
		t.Fatalf("ExpandHome = %q", got)
	}
}

func TestDiscoverGlobalDedupesAliasDirs(t *testing.T) {
	home := t.TempDir()
	t.Setenv(HomeEnv, home)
	if err := mkdir(filepath.Join(home, ".gemini", "config", "skills")); err != nil {
		t.Fatal(err)
	}
	writeSkill(t, filepath.Join(home, ".gemini", "config", "skills"), "alpha")
	if err := os.Symlink(filepath.Join(home, ".gemini", "config", "skills"), filepath.Join(home, ".gemini", "antigravity", "skills")); err != nil {
		if err := mkdir(filepath.Join(home, ".gemini", "antigravity")); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(filepath.Join(home, ".gemini", "config", "skills"), filepath.Join(home, ".gemini", "antigravity", "skills")); err != nil {
			t.Fatal(err)
		}
	}
	spec, _ := Lookup("antigravity")
	got, err := spec.Discover(domain.GlobalScope, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Name != "alpha" || got[0].ReadOnly {
		t.Fatalf("got %+v", got)
	}
}

func TestDiscoverProjectAndPlugins(t *testing.T) {
	home := t.TempDir()
	t.Setenv(HomeEnv, home)
	root := t.TempDir()
	writeSkill(t, filepath.Join(root, ".agents", "skills"), "shared")
	writeSkill(t, filepath.Join(root, ".cursor", "skills"), "local")

	cursor, _ := Lookup("cursor")
	got, err := cursor.Discover(domain.ProjectScope(root), root)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("cursor project skills: %+v", got)
	}
	var src Source
	dirs := src.ProjectDirs(cursor.ToDomain(), root)
	if len(dirs) != 4 || dirs[0] != filepath.Join(root, ".cursor", "skills") {
		t.Fatalf("project dirs = %v", dirs)
	}

	pluginSkills := filepath.Join(home, ".claude", "plugins", "cache", "official", "expo", "1.0.0", "skills")
	writeSkill(t, pluginSkills, "eas")
	writeSkill(t, pluginSkills, "router")
	plugins, _ := Lookup(PluginsAgentID)
	got, err = plugins.Discover(domain.GlobalScope, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || !got[0].ReadOnly || got[0].Name != "eas" {
		t.Fatalf("plugin skills: %+v", got)
	}
	if got, _ := plugins.Discover(domain.ProjectScope(root), root); len(got) != 0 {
		t.Fatalf("plugins should have no project skills: %+v", got)
	}
}
