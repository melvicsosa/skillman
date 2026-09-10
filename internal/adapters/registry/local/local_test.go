package local

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/melvicsosa/skillman/internal/adapters/registry/archive/archivetest"
	"github.com/melvicsosa/skillman/internal/domain"
)

func TestLooks(t *testing.T) {
	dir := t.TempDir()
	for ref, want := range map[string]bool{"./x": true, "/abs": true, "~/x": true, dir: true, "owner/repo": false, "https://x/y.zip": false, "skillssh:a/b/c": false, "nope/none": false} {
		if got := Looks(ref); got != want {
			t.Errorf("Looks(%q) = %v", ref, got)
		}
	}
}

func TestFetchDirFileAndArchive(t *testing.T) {
	home := t.TempDir()
	skill := filepath.Join(home, "src", "alpha")
	os.MkdirAll(skill, 0o755)
	os.WriteFile(filepath.Join(skill, "SKILL.md"), []byte("---\nname: alpha\ndescription: d\n---\n"), 0o644)
	s := Source{Home: func() (string, error) { return home, nil }}
	ctx := context.Background()

	res, err := s.Fetch(ctx, "~/src/alpha")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Cleanup()
	if res.Name != "alpha" || res.Source.Type != domain.SourceLocal || res.Source.Ref != skill || filepath.Base(res.Dir) != "alpha" || res.Dir == skill {
		t.Fatalf("dir fetch = %+v", res)
	}

	res2, err := s.Fetch(ctx, filepath.Join(skill, "SKILL.md"))
	if err != nil || res2.Name != "alpha" {
		t.Fatalf("SKILL.md fetch = %+v err=%v", res2, err)
	}
	res2.Cleanup()

	tgz := filepath.Join(home, "pack.tar.gz")
	os.WriteFile(tgz, archivetest.TarGz(t, map[string]string{"beta/SKILL.md": "---\nname: beta\ndescription: d\n---\n"}), 0o644)
	res3, err := s.Fetch(ctx, tgz)
	if err != nil || res3.Name != "beta" || filepath.Base(res3.Dir) != "beta" {
		t.Fatalf("archive fetch = %+v err=%v", res3, err)
	}
	res3.Cleanup()

	flat := filepath.Join(home, "flat.tgz")
	os.WriteFile(flat, archivetest.TarGz(t, map[string]string{"SKILL.md": "x", "ref/a.md": "y"}), 0o644)
	res4, err := s.Fetch(ctx, flat)
	if err != nil || res4.Name != "flat" || filepath.Base(res4.Dir) != "flat" {
		t.Fatalf("flat archive fetch = %+v err=%v", res4, err)
	}
	if _, err := os.Stat(filepath.Join(res4.Dir, "ref", "a.md")); err != nil {
		t.Fatal(err)
	}
	res4.Cleanup()

	if _, err := s.Fetch(ctx, filepath.Join(home, "missing")); err == nil {
		t.Fatal("missing path accepted")
	}
}
