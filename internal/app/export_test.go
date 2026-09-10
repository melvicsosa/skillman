package app

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	skillfs "github.com/melvicsosa/skillman/internal/adapters/fs"
	"github.com/melvicsosa/skillman/internal/domain"
)

func TestExportPlugin(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	f.source("fake:one", "one", "first\n")
	f.source("fake:two", "two", "second\n")
	f.add("fake:one", InstallOptions{})
	f.add("fake:two", InstallOptions{})
	out := filepath.Join(f.home, "out")

	res, err := f.svc.ExportPlugin(ctx, ExportOptions{OutDir: out, Name: "demo", Description: "Demo plugin", Skills: []string{"one", "two", "one"}})
	if err != nil {
		t.Fatal(err)
	}
	if res.Dir != filepath.Join(out, "demo") || len(res.Skills) != 2 || res.Manifest != filepath.Join(out, "demo", ".claude-plugin", "plugin.json") {
		t.Fatalf("result = %+v", res)
	}
	b, err := os.ReadFile(res.Manifest)
	if err != nil {
		t.Fatal(err)
	}
	var manifest map[string]any
	if json.Unmarshal(b, &manifest) != nil || manifest["name"] != "demo" || manifest["version"] != DefaultPluginVersion || manifest["description"] != "Demo plugin" {
		t.Fatalf("plugin.json = %s", b)
	}
	if _, ok := manifest["author"]; ok {
		t.Fatalf("author should be omitted: %s", b)
	}
	for _, name := range []string{"one", "two"} {
		want, _ := skillfs.HashDir(filepath.Join(f.dataDir, "vault", name))
		got, err := skillfs.HashDir(filepath.Join(res.Dir, "skills", name))
		if err != nil || got != want {
			t.Fatalf("%s copy hash %s want %s (%v)", name, got, want, err)
		}
	}
	readme, _ := os.ReadFile(filepath.Join(res.Dir, "README.md"))
	if !strings.Contains(string(readme), "`one`: Description of one") || !strings.Contains(string(readme), "`two`") {
		t.Fatalf("README = %s", readme)
	}
	// The exported directory is a valid plugin for the converter.
	if shape, err := f.svc.converter.Detect(res.Dir); err != nil || shape != domain.ShapeClaudePlugin {
		t.Fatalf("detect export = %v %v", shape, err)
	}
	if _, err := f.svc.ExportPlugin(ctx, ExportOptions{OutDir: out, Name: "demo", Skills: []string{"one"}}); !errors.Is(err, domain.ErrExists) {
		t.Fatalf("export over existing err = %v", err)
	}
	if _, err := f.svc.ExportPlugin(ctx, ExportOptions{OutDir: out, Name: "Bad Name", Skills: []string{"one"}}); !errors.Is(err, domain.ErrRejected) {
		t.Fatalf("bad name err = %v", err)
	}
	if _, err := f.svc.ExportPlugin(ctx, ExportOptions{OutDir: out, Name: "other", Skills: []string{"missing"}}); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("missing skill err = %v", err)
	}
	if _, err := os.Stat(filepath.Join(out, "other")); err == nil {
		t.Fatal("failed export left a directory behind")
	}
}
