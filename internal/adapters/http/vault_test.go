package http

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	skillfs "github.com/melvicsosa/skillman/internal/adapters/fs"
	"github.com/melvicsosa/skillman/internal/app"
	"github.com/melvicsosa/skillman/internal/domain"
)

// fakeRegistry serves "fake:<name>" refs from directories in sources.
type fakeRegistry struct {
	sources map[string]string
	hits    []domain.RegistryResult
}

func (f *fakeRegistry) ID() string              { return "fake" }
func (f *fakeRegistry) Matches(ref string) bool { return strings.HasPrefix(ref, "fake:") }
func (f *fakeRegistry) Search(context.Context, string) ([]domain.RegistryResult, error) {
	return f.hits, nil
}
func (f *fakeRegistry) Trending(context.Context) ([]domain.RegistryResult, error) { return f.hits, nil }
func (f *fakeRegistry) Fetch(_ context.Context, ref string) (domain.FetchResult, error) {
	src, ok := f.sources[ref]
	if !ok {
		return domain.FetchResult{}, fmt.Errorf("%s: %w", ref, domain.ErrNotFound)
	}
	tmp, err := os.MkdirTemp("", "fake-*")
	if err != nil {
		return domain.FetchResult{}, err
	}
	dir := filepath.Join(tmp, filepath.Base(src))
	if err := skillfs.CopyTree(src, dir); err != nil {
		return domain.FetchResult{}, err
	}
	return domain.FetchResult{Dir: dir, Source: domain.VaultSource{Type: "fake", Ref: ref}, Cleanup: func() { os.RemoveAll(tmp) }}, nil
}

func TestVaultAndRegistryRoutes(t *testing.T) {
	srv, home, reg := newTestServerWithRegistry(t)
	reg.sources["fake:alpha"] = writeSkill(t, filepath.Join(home, "sources"), "alpha")
	reg.hits = []domain.RegistryResult{{Registry: "fake", ID: "alpha", Name: "alpha", Ref: "fake:alpha"}}

	status, body := do(t, "GET", srv.URL+"/api/registry/search?q=alpha", nil)
	var search app.SearchResult
	if status != 200 || json.Unmarshal(body, &search) != nil || len(search.Results) != 1 || search.Results[0].Ref != "fake:alpha" {
		t.Fatalf("search %d %s", status, body)
	}
	if status, body = do(t, "GET", srv.URL+"/api/registry/search", nil); status != 400 {
		t.Fatalf("search without q %d %s", status, body)
	}
	if status, body = do(t, "GET", srv.URL+"/api/registry/trending", nil); status != 200 || !strings.Contains(string(body), `"fake:alpha"`) {
		t.Fatalf("trending %d %s", status, body)
	}
	if status, body = do(t, "GET", srv.URL+"/api/registry/curated", nil); status != 501 {
		t.Fatalf("curated %d %s", status, body)
	}

	status, body = do(t, "POST", srv.URL+"/api/registry/add", map[string]any{"ref": "fake:alpha", "agents": []string{"codex"}})
	var added app.AddResult
	if status != 201 || json.Unmarshal(body, &added) != nil || len(added.Entries) != 1 || len(added.Links) != 1 {
		t.Fatalf("add %d %s", status, body)
	}
	if status, body = do(t, "POST", srv.URL+"/api/registry/add", map[string]any{"ref": "fake:missing"}); status != 404 {
		t.Fatalf("add missing %d %s", status, body)
	}
	if status, body = do(t, "POST", srv.URL+"/api/registry/add", map[string]any{}); status != 400 {
		t.Fatalf("add without ref %d %s", status, body)
	}

	status, body = do(t, "GET", srv.URL+"/api/vault", nil)
	var vault []VaultView
	if status != 200 || json.Unmarshal(body, &vault) != nil || len(vault) != 1 || vault[0].Name != "alpha" || len(vault[0].Links) != 3 {
		t.Fatalf("vault %d %s", status, body)
	}
	status, body = do(t, "POST", srv.URL+"/api/vault/alpha/install", map[string]any{"agents": []string{"opencode"}})
	if status != 200 || !strings.Contains(string(body), ".config/opencode/skills/alpha") {
		t.Fatalf("install %d %s", status, body)
	}
	if status, body = do(t, "POST", srv.URL+"/api/vault/alpha/install", map[string]any{}); status != 400 {
		t.Fatalf("install without agents %d %s", status, body)
	}
	if status, body = do(t, "POST", srv.URL+"/api/vault/alpha/update", nil); status != 200 || !strings.Contains(string(body), `"updated":false`) {
		t.Fatalf("update %d %s", status, body)
	}
	status, body = do(t, "POST", srv.URL+"/api/vault/alpha/uninstall", map[string]any{"agent": "opencode"})
	if status != 200 || !strings.Contains(string(body), ".config/opencode/skills/alpha") {
		t.Fatalf("uninstall %d %s", status, body)
	}
	if status, body = do(t, "DELETE", srv.URL+"/api/vault/alpha", nil); status != 409 {
		t.Fatalf("delete linked %d %s", status, body)
	}
	if status, body = do(t, "DELETE", srv.URL+"/api/vault/alpha?force=true", nil); status != 200 {
		t.Fatalf("delete forced %d %s", status, body)
	}
	if status, body = do(t, "DELETE", srv.URL+"/api/vault/alpha", nil); status != 404 {
		t.Fatalf("delete missing %d %s", status, body)
	}
	if status, body = do(t, "GET", srv.URL+"/api/vault", nil); status != 200 || string(body) != "[]\n" {
		t.Fatalf("empty vault %d %s", status, body)
	}
}

func TestImportLockRoute(t *testing.T) {
	srv, home, _ := newTestServerWithRegistry(t)
	writeSkill(t, filepath.Join(home, ".agents", "skills"), "locked")
	lock := `{"version":3,"skills":{"locked":{"source":"o/r","sourceType":"github","sourceUrl":"https://github.com/o/r.git","skillPath":"skills/locked/SKILL.md","skillFolderHash":"h"}}}`
	os.MkdirAll(filepath.Join(home, ".agents"), 0o755)
	if err := os.WriteFile(filepath.Join(home, ".agents", ".skill-lock.json"), []byte(lock), 0o644); err != nil {
		t.Fatal(err)
	}
	status, body := do(t, "POST", srv.URL+"/api/import-lock", nil)
	var res app.LockImportResult
	if status != 200 || json.Unmarshal(body, &res) != nil || len(res.Imported) != 1 {
		t.Fatalf("import-lock %d %s", status, body)
	}
	status, body = do(t, "GET", srv.URL+"/api/vault", nil)
	if status != 200 || !strings.Contains(string(body), `"type":"lock"`) {
		t.Fatalf("vault after import %d %s", status, body)
	}
}
