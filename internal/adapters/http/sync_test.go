package http

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/melvicsosa/skillman/internal/app"
)

func TestSyncRepairAdoptExportRoutes(t *testing.T) {
	srv, home, reg := newTestServerWithRegistry(t)
	reg.sources["fake:alpha"] = writeSkill(t, filepath.Join(home, "sources"), "alpha")
	if status, body := do(t, "POST", srv.URL+"/api/registry/add", map[string]any{"ref": "fake:alpha", "agents": []string{"codex"}, "copy": true}); status != 201 {
		t.Fatalf("add %d %s", status, body)
	}

	// Sync one entry (dry run), then all.
	status, body := do(t, "POST", srv.URL+"/api/vault/alpha/sync", map[string]any{"dryRun": true})
	var report app.SyncReport
	if status != 200 || json.Unmarshal(body, &report) != nil || !report.DryRun || report.Linked == 0 {
		t.Fatalf("sync dry %d %s", status, body)
	}
	if status, body = do(t, "POST", srv.URL+"/api/sync", nil); status != 200 || json.Unmarshal(body, &report) != nil || report.DryRun || report.Linked == 0 {
		t.Fatalf("sync all %d %s", status, body)
	}
	if _, err := os.Stat(filepath.Join(home, ".claude/skills/alpha/SKILL.md")); err != nil {
		t.Fatalf("sync did not copy into claude: %v", err)
	}

	// Auto-sync flag.
	if status, body = do(t, "PATCH", srv.URL+"/api/vault/alpha", map[string]any{"autoSync": true}); status != 200 || !strings.Contains(string(body), `"autoSync":true`) {
		t.Fatalf("patch %d %s", status, body)
	}
	if status, body = do(t, "PATCH", srv.URL+"/api/vault/alpha", map[string]any{}); status != 400 {
		t.Fatalf("patch without flag %d %s", status, body)
	}

	// Drift in the claude copy, reported with copies and repaired from the vault.
	edited := filepath.Join(home, ".claude/skills/alpha/SKILL.md")
	if err := os.WriteFile(edited, []byte("---\nname: alpha\ndescription: d\n---\nedited\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	status, body = do(t, "GET", srv.URL+"/api/doctor", nil)
	var doctor app.DoctorReport
	if status != 200 || json.Unmarshal(body, &doctor) != nil || doctor.Counts[app.IssueDrift] != 1 {
		t.Fatalf("doctor %d %s", status, body)
	}
	if issue := doctor.Issues[0]; issue.VaultRef != "alpha" || !issue.Repairable || len(issue.Copies) < 2 {
		t.Fatalf("drift issue = %+v", issue)
	}
	status, body = do(t, "POST", srv.URL+"/api/doctor/drift/alpha/repair", map[string]any{"from": "vault", "to": []string{"claude-code"}})
	var repair app.RepairResult
	if status != 200 || json.Unmarshal(body, &repair) != nil || len(repair.Replaced) != 1 {
		t.Fatalf("repair %d %s", status, body)
	}
	if status, body = do(t, "POST", srv.URL+"/api/doctor/drift/alpha/repair", map[string]any{}); status != 400 {
		t.Fatalf("repair without from %d %s", status, body)
	}
	if status, body = do(t, "POST", srv.URL+"/api/doctor/drift/nope/repair", map[string]any{"from": "vault"}); status != 404 {
		t.Fatalf("repair unknown %d %s", status, body)
	}

	// Adopt a hand-made skill.
	writeSkill(t, filepath.Join(home, ".config/opencode/skills"), "mine")
	if status, body = do(t, "POST", srv.URL+"/api/scan", nil); status != 200 {
		t.Fatalf("scan %d %s", status, body)
	}
	status, body = do(t, "POST", srv.URL+"/api/vault/adopt", map[string]any{"name": "mine", "from": "opencode"})
	if status != 201 || !strings.Contains(string(body), `"type":"local"`) {
		t.Fatalf("adopt %d %s", status, body)
	}
	if status, body = do(t, "POST", srv.URL+"/api/vault/adopt", map[string]any{"name": "mine", "from": "opencode"}); status != 409 {
		t.Fatalf("adopt twice %d %s", status, body)
	}

	// Export.
	out := filepath.Join(home, "out")
	status, body = do(t, "POST", srv.URL+"/api/vault/export", map[string]any{"outDir": out, "name": "demo", "skills": []string{"alpha", "mine"}})
	var export app.ExportResult
	if status != 201 || json.Unmarshal(body, &export) != nil || len(export.Skills) != 2 {
		t.Fatalf("export %d %s", status, body)
	}
	if _, err := os.Stat(filepath.Join(out, "demo", ".claude-plugin", "plugin.json")); err != nil {
		t.Fatal(err)
	}
	if status, body = do(t, "POST", srv.URL+"/api/vault/export", map[string]any{"outDir": out, "name": "demo", "skills": []string{}}); status != 400 {
		t.Fatalf("export without skills %d %s", status, body)
	}

	// Marketplaces setting round-trips through PATCH /api/settings.
	status, body = do(t, "PATCH", srv.URL+"/api/settings", map[string]any{"marketplaces": []string{"a/b", " c/d "}})
	if status != 200 || !strings.Contains(string(body), `"marketplaces":["a/b","c/d"]`) {
		t.Fatalf("settings %d %s", status, body)
	}
	if status, body = do(t, "PATCH", srv.URL+"/api/settings", map[string]any{"marketplaces": []string{"not a repo"}}); status != 422 {
		t.Fatalf("bad marketplace %d %s", status, body)
	}
	if status, body = do(t, "GET", srv.URL+"/api/settings", nil); status != 200 || !strings.Contains(string(body), `"marketplaces":["a/b","c/d"]`) {
		t.Fatalf("get settings %d %s", status, body)
	}

	// SPA fallback serves the app for client-side routes.
	for _, p := range []string{"/settings", "/vault", "/agent/codex"} {
		if status, body = do(t, "GET", srv.URL+p, nil); status != 200 || !strings.Contains(string(body), "<html") {
			t.Fatalf("spa %s %d %.80s", p, status, body)
		}
	}
}
