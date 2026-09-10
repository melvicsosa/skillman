package fs

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeSkill(t *testing.T, dir, name, frontmatter string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.MkdirAll(p, 0o755); err != nil {
		t.Fatal(err)
	}
	content := "---\n" + frontmatter + "---\n\n# " + name + "\n"
	if frontmatter == "" {
		content = "# " + name + "\n"
	}
	if err := os.WriteFile(filepath.Join(p, SkillFile), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestParseSkillMD(t *testing.T) {
	tests := []struct {
		name     string
		in       string
		wantName string
		wantDesc string
		wantVer  string
		wantBody string
		wantErr  error
	}{
		{
			name:     "full",
			in:       "---\nname: go-testing\ndescription: \"Trigger: Go tests.\"\nlicense: Apache-2.0\nmetadata:\n  author: gp\n  version: \"1.0\"\n---\n\n## Body\n",
			wantName: "go-testing", wantDesc: "Trigger: Go tests.", wantVer: "1.0", wantBody: "\n## Body\n",
		},
		{
			name:     "non-string metadata is stringified",
			in:       "---\nname: x\ndescription: d\nversion: 2\nmetadata:\n  version: 3\n  tags: [a, b]\n---\nbody",
			wantName: "x", wantDesc: "d", wantVer: "3", wantBody: "body",
		},
		{
			name:     "top-level version fallback",
			in:       "---\nname: x\ndescription: d\nversion: 2.1\n---\n",
			wantName: "x", wantDesc: "d", wantVer: "2.1", wantBody: "",
		},
		{
			name: "missing frontmatter", in: "# Just markdown\n",
			wantBody: "# Just markdown\n", wantErr: ErrNoFrontmatter,
		},
		{
			name: "unterminated frontmatter", in: "---\nname: x\n",
			wantBody: "---\nname: x\n", wantErr: ErrNoFrontmatter,
		},
		{
			name: "invalid yaml", in: "---\nname: [unclosed\n---\nbody",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fm, body, err := ParseSkillMD([]byte(tt.in))
			if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if tt.name == "invalid yaml" {
				if err == nil || errors.Is(err, ErrNoFrontmatter) {
					t.Fatalf("expected yaml error, got %v", err)
				}
				return
			}
			if fm.Name != tt.wantName || fm.Description != tt.wantDesc || fm.Version() != tt.wantVer {
				t.Fatalf("got name=%q desc=%q ver=%q", fm.Name, fm.Description, fm.Version())
			}
			if body != tt.wantBody {
				t.Fatalf("body = %q, want %q", body, tt.wantBody)
			}
		})
	}
	fm, _, err := ParseSkillMD([]byte("---\nname: x\ndescription: d\nmetadata:\n  tags: [a, b]\n  nested:\n    k: v\n---\n"))
	if err != nil {
		t.Fatal(err)
	}
	if fm.Metadata["tags"] != "a,b" || fm.Metadata["nested"] != "k=v" {
		t.Fatalf("metadata = %v", fm.Metadata)
	}
}

func TestHashDirDeterministicAndFollowsRootSymlink(t *testing.T) {
	root := t.TempDir()
	a := writeSkill(t, root, "a", "name: a\ndescription: d\n")
	if err := os.MkdirAll(filepath.Join(a, "node_modules"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(a, "node_modules", "junk"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(a, ".DS_Store"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	h1, err := HashDir(a)
	if err != nil {
		t.Fatal(err)
	}
	h2, err := HashDir(a)
	if err != nil || h1 != h2 {
		t.Fatalf("hash not stable: %s vs %s (%v)", h1, h2, err)
	}
	link := filepath.Join(root, "a-link")
	if err := os.Symlink(a, link); err != nil {
		t.Fatal(err)
	}
	h3, err := HashDir(link)
	if err != nil || h3 != h1 {
		t.Fatalf("symlinked dir hash differs: %s vs %s (%v)", h3, h1, err)
	}
	if err := os.WriteFile(filepath.Join(a, "extra.md"), []byte("more"), 0o644); err != nil {
		t.Fatal(err)
	}
	h4, err := HashDir(a)
	if err != nil || h4 == h1 {
		t.Fatalf("hash did not change after adding a file (%v)", err)
	}
}

func TestDiscoverSkills(t *testing.T) {
	root := t.TempDir()
	writeSkill(t, root, "alpha", "name: alpha\ndescription: d\n")
	writeSkill(t, root, "_shared", "name: _shared\ndescription: d\n")
	writeSkill(t, root, ".hidden", "name: hidden\ndescription: d\n")
	if err := os.MkdirAll(filepath.Join(root, "no-skill-md"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "file.md"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	target := writeSkill(t, t.TempDir(), "beta", "name: beta\ndescription: d\n")
	if err := os.Symlink(target, filepath.Join(root, "beta")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "missing"), filepath.Join(root, "dangling")); err != nil {
		t.Fatal(err)
	}

	got, err := DiscoverSkills(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("got %d entries: %+v", len(got), got)
	}
	if got[0].Name != "alpha" || got[0].IsSymlink {
		t.Errorf("alpha = %+v", got[0])
	}
	resolvedTarget, _ := filepath.EvalSymlinks(target)
	if got[1].Name != "beta" || !got[1].IsSymlink || got[1].LinkTarget != resolvedTarget || got[1].Dangling {
		t.Errorf("beta = %+v (want target %s)", got[1], resolvedTarget)
	}
	if got[2].Name != "dangling" || !got[2].Dangling {
		t.Errorf("dangling = %+v", got[2])
	}

	if got, err := DiscoverSkills(filepath.Join(root, "does-not-exist")); err != nil || got != nil {
		t.Fatalf("missing dir: got %v, %v", got, err)
	}
}

func TestMoverRoundTripDirAndSymlink(t *testing.T) {
	root := t.TempDir()
	agentDir := filepath.Join(root, "agent")
	q := filepath.Join(root, "quarantine")
	real := writeSkill(t, agentDir, "real", "name: real\ndescription: d\n")
	target := writeSkill(t, filepath.Join(root, "vault"), "linked", "name: linked\ndescription: d\n")
	link := filepath.Join(agentDir, "linked")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}

	var m Mover
	dest, err := m.Disable(real, q)
	if err != nil {
		t.Fatal(err)
	}
	if dest != filepath.Join(q, "real") {
		t.Fatalf("dest = %s", dest)
	}
	if _, err := os.Lstat(real); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("source still present: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dest, SkillFile)); err != nil {
		t.Fatalf("moved content missing: %v", err)
	}

	ldest, err := m.Disable(link, q)
	if err != nil {
		t.Fatal(err)
	}
	if info, err := os.Lstat(ldest); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("quarantined entry is not a symlink: %v", err)
	}
	if _, err := os.Stat(filepath.Join(target, SkillFile)); err != nil {
		t.Fatalf("symlink target was touched: %v", err)
	}

	// Refuse to overwrite.
	writeSkill(t, agentDir, "real", "name: real\ndescription: d\n")
	if _, err := m.Disable(filepath.Join(agentDir, "real"), q); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("expected already-exists error, got %v", err)
	}
	if err := os.RemoveAll(filepath.Join(agentDir, "real")); err != nil {
		t.Fatal(err)
	}

	if err := m.Enable(dest, real); err != nil {
		t.Fatal(err)
	}
	if err := m.Enable(ldest, link); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(real, SkillFile)); err != nil {
		t.Fatalf("restored dir missing: %v", err)
	}
	if info, err := os.Lstat(link); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("restored link is not a symlink: %v", err)
	}
	if entries, _ := os.ReadDir(q); len(entries) != 0 {
		t.Fatalf("quarantine not empty: %v", entries)
	}
}

func TestCopyTreeFallback(t *testing.T) {
	root := t.TempDir()
	src := writeSkill(t, root, "src", "name: src\ndescription: d\n")
	if err := os.Symlink("SKILL.md", filepath.Join(src, "alias")); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(root, "dest")
	if err := copyTree(src, dest); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Lstat(filepath.Join(dest, "alias")); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("inner symlink not preserved: %v", err)
	}
	h1, _ := HashDir(src)
	h2, _ := HashDir(dest)
	if h1 != h2 {
		t.Fatalf("copied tree hash differs")
	}
}

func TestInspectorSpecIssues(t *testing.T) {
	root := t.TempDir()
	var insp Inspector

	ok := writeSkill(t, root, "good-one", "name: good-one\ndescription: fine\nmetadata:\n  version: \"2\"\n")
	info, err := insp.Inspect(ok)
	if err != nil || len(info.SpecIssues) != 0 || info.Version != "2" || info.ContentHash == "" {
		t.Fatalf("good skill: %+v err=%v", info, err)
	}

	bad := writeSkill(t, root, "bad", "name: Bad_Name\n")
	info, err = insp.Inspect(bad)
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(info.SpecIssues, "; ")
	if !strings.Contains(joined, "must match") || !strings.Contains(joined, "missing 'description'") {
		t.Fatalf("issues = %v", info.SpecIssues)
	}

	mismatch := writeSkill(t, root, "dir-name", "name: other-name\ndescription: d\n")
	info, _ = insp.Inspect(mismatch)
	if len(info.SpecIssues) != 1 || !strings.Contains(info.SpecIssues[0], "does not match directory") {
		t.Fatalf("issues = %v", info.SpecIssues)
	}

	none := writeSkill(t, root, "none", "")
	info, err = insp.Inspect(none)
	if err != nil || len(info.SpecIssues) != 1 || !strings.Contains(info.SpecIssues[0], "no YAML frontmatter") {
		t.Fatalf("no frontmatter: %+v err=%v", info, err)
	}
}
