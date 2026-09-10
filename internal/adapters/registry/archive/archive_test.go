package archive

import (
	"archive/zip"
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/melvicsosa/skillman/internal/adapters/registry/archive/archivetest"
)

func TestExtractTarGzSubpathAndStrip(t *testing.T) {
	data := archivetest.TarGz(t, map[string]string{
		"owner-repo-abc123/README.md":                   "r",
		"owner-repo-abc123/skills/alpha/SKILL.md":       "a",
		"owner-repo-abc123/skills/alpha/scripts/run.sh": "s",
		"owner-repo-abc123/skills/beta/SKILL.md":        "b",
		"../evil":                                       "x",
	})
	dst := t.TempDir()
	res, err := ExtractTarGz(bytes.NewReader(data), dst, Options{StripTop: true, Subpath: "skills/alpha"})
	if err != nil {
		t.Fatal(err)
	}
	if res.TopDir != "owner-repo-abc123" || res.Files != 2 {
		t.Fatalf("result = %+v", res)
	}
	if b, err := os.ReadFile(filepath.Join(dst, "scripts", "run.sh")); err != nil || string(b) != "s" {
		t.Fatalf("nested file: %v %q", err, b)
	}
	if _, err := os.Stat(filepath.Join(dst, "README.md")); err == nil {
		t.Fatal("README outside subpath was extracted")
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(dst), "evil")); err == nil {
		t.Fatal("path traversal entry was written")
	}
	_, err = ExtractTarGz(bytes.NewReader(data), t.TempDir(), Options{StripTop: true, Subpath: "skills/nope"})
	if !errors.Is(err, ErrSubpathMissing) {
		t.Fatalf("missing subpath err = %v", err)
	}
}

func TestExtractZip(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, content := range map[string]string{"pkg/SKILL.md": "---\nname: pkg\n---\n", "pkg/ref/a.md": "a"} {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		w.Write([]byte(content))
	}
	zw.Close()
	dst := t.TempDir()
	res, err := ExtractZip(&buf, dst, Options{})
	if err != nil || res.TopDir != "pkg" || res.Files != 2 {
		t.Fatalf("zip = %+v err=%v", res, err)
	}
	if _, err := os.Stat(filepath.Join(dst, "pkg", "ref", "a.md")); err != nil {
		t.Fatal(err)
	}
}
